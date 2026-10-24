package httpapi

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/lezhnev/arena/engine/app"
)

func TestRateLimiterAllowsLimitPerWindowPerHost(t *testing.T) {
	clk := &testClock{t: time.Unix(1000, 0)}
	l := newRateLimiter(3, time.Minute, 10, clk.now)
	for i := range 3 {
		if ok, _ := l.allow("a"); !ok {
			t.Fatalf("request %d refused", i)
		}
	}
	ok, wait := l.allow("a")
	if ok || wait != time.Minute {
		t.Errorf("fourth: ok=%v wait=%v, want refused for a minute", ok, wait)
	}
	if ok, _ := l.allow("b"); !ok {
		t.Error("another host must not be limited")
	}
	clk.t = clk.t.Add(61 * time.Second)
	if ok, _ := l.allow("a"); !ok {
		t.Error("the window should have passed")
	}
}

func TestRateLimiterTableIsBounded(t *testing.T) {
	clk := &testClock{t: time.Unix(1000, 0)}
	l := newRateLimiter(3, time.Minute, 5, clk.now)
	for i := range 5 {
		l.allow("h" + strconv.Itoa(i))
		clk.t = clk.t.Add(time.Second)
	}
	ok, wait := l.allow("new")
	if ok || wait <= 0 {
		t.Errorf("full table of fresh hosts: ok=%v wait=%v, want refused", ok, wait)
	}
	clk.t = clk.t.Add(2 * time.Minute)
	if ok, _ := l.allow("new"); !ok {
		t.Error("stale hosts should be swept to make room")
	}
	if n := len(l.hits); n != 1 {
		t.Errorf("table holds %d hosts, want 1", n)
	}
}

func checkHandler(clk *testClock, seen *string) http.Handler {
	return New(nil, Options{
		Check: func(src string) (app.CheckResult, error) {
			*seen = src
			return app.CheckResult{Trial: &app.Trial{}}, nil
		},
		Now: clk.now,
	})
}

func TestCheckRateLimitIsPerRemoteHostAndIgnoresForwardedFor(t *testing.T) {
	var seen string
	h := checkHandler(&testClock{t: time.Unix(1000, 0)}, &seen)
	post := func(host string) int {
		req := httptest.NewRequest("POST", "/api/check", strings.NewReader("x"))
		req.RemoteAddr = host + ":9"
		req.Header.Set("X-Forwarded-For", strconv.Itoa(len(seen))+".9.9.9")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		return w.Code
	}
	for range checkLimit {
		if c := post("1.1.1.1"); c != 200 {
			t.Fatalf("got %d", c)
		}
	}
	if c := post("1.1.1.1"); c != 429 {
		t.Errorf("over the limit: %d, want 429", c)
	}
	if c := post("2.2.2.2"); c != 200 {
		t.Errorf("other host: %d", c)
	}
}

func TestCheckGetDecodesTheProgram(t *testing.T) {
	var seen string
	h := checkHandler(&testClock{t: time.Unix(1000, 0)}, &seen)
	req := httptest.NewRequest("GET", "/api/check?src="+url.QueryEscape("a+b c\n{}"), nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != 200 || seen != "a+b c\n{}" {
		t.Errorf("code %d, src %q", w.Code, seen)
	}
}

func TestCheckAnswers503WhenAllTrialSlotsAreBusy(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	h := New(nil, Options{Check: func(string) (app.CheckResult, error) {
		started <- struct{}{}
		<-release
		return app.CheckResult{Trial: &app.Trial{}}, nil
	}})
	post := func(host string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("POST", "/api/check", strings.NewReader("x"))
		req.RemoteAddr = host + ":9"
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		return w
	}
	done := make(chan int, maxTrials)
	for i := range maxTrials {
		go func() { done <- post("10.0.0." + strconv.Itoa(i)).Code }()
		<-started
	}
	w := post("10.0.1.1")
	if w.Code != 503 || w.Header().Get("Retry-After") != "1" {
		t.Errorf("busy: %d Retry-After=%q, want 503 and 1", w.Code, w.Header().Get("Retry-After"))
	}
	close(release)
	for range maxTrials {
		if c := <-done; c != 200 {
			t.Errorf("running check answered %d", c)
		}
	}
	go func() { <-started }()
	if c := post("10.0.1.2").Code; c != 200 {
		t.Errorf("after the slots freed: %d, want 200", c)
	}
}

// postCheck posts a program and returns the status, or 0 when net/http dropped
// the connection after recovering a panic.
func postCheck(url string) int {
	req, _ := http.NewRequest("POST", url+"/api/check", strings.NewReader("x"))
	req.Header.Set("Connection", "close")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0
	}
	defer resp.Body.Close()
	return resp.StatusCode
}

// A checker that errors or panics must still free its trial slot.
func TestCheckFreesTheTrialSlotAfterAnErrorOrPanic(t *testing.T) {
	bads := map[string]CheckFunc{
		"error": func(string) (app.CheckResult, error) { return app.CheckResult{}, io.ErrUnexpectedEOF },
		"panic": func(string) (app.CheckResult, error) { panic("boom") },
	}
	for name, bad := range bads {
		t.Run(name, func(t *testing.T) { checkSlotSurvives(t, bad) })
	}
}

func checkSlotSurvives(t *testing.T, bad CheckFunc) {
	failing := true
	srv := httptest.NewServer(New(nil, Options{Check: func(src string) (app.CheckResult, error) {
		if failing {
			return bad(src)
		}
		return app.CheckResult{Trial: &app.Trial{}}, nil
	}}))
	defer srv.Close()
	for i := range 2 * maxTrials {
		if postCheck(srv.URL) == http.StatusServiceUnavailable {
			t.Fatalf("failing request %d answered 503: slot leaked", i)
		}
	}
	failing = false
	if c := postCheck(srv.URL); c != 200 {
		t.Errorf("working checker after failures: %d, want 200", c)
	}
}

type failingBody struct{}

func (failingBody) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }

func TestCheckBodyReadFailureIsBadRequestNotTooLarge(t *testing.T) {
	h := checkHandler(&testClock{t: time.Unix(1000, 0)}, new(string))
	req := httptest.NewRequest("POST", "/api/check", failingBody{})
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != 400 {
		t.Errorf("got %d, want 400", w.Code)
	}
}

func TestCheckRateLimitCountsRejectedRequestsToo(t *testing.T) {
	h := checkHandler(&testClock{t: time.Unix(1000, 0)}, new(string))
	get := func() int {
		req := httptest.NewRequest("GET", "/api/check", nil) // no src: 400
		req.RemoteAddr = "1.1.1.1:9"
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		return w.Code
	}
	for range checkLimit {
		if c := get(); c != 400 {
			t.Fatalf("got %d, want 400", c)
		}
	}
	if c := get(); c != 429 {
		t.Errorf("got %d, want 429", c)
	}
}
