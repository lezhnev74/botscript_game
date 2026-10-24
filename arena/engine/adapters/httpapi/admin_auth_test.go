package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

type testClock struct{ t time.Time }

func (c *testClock) now() time.Time { return c.t }

func newTestHandlers(clk *testClock) *adminHandlers {
	return &adminHandlers{auth: newAdminAuth(AdminOptions{
		Password: "pw", Now: clk.now, Sleep: func(time.Duration) {},
		NewSessionID: func() string { return "s" },
	})}
}

func tryLogin(h *adminHandlers, host, pass string) *httptest.ResponseRecorder {
	req := httptest.NewRequest("POST", "/api/admin/login", strings.NewReader(`{"password":"`+pass+`"}`))
	req.RemoteAddr = host + ":1234"
	w := httptest.NewRecorder()
	h.login(w, req)
	return w
}

func fillHosts(h *adminHandlers) {
	for i := range maxFailureHosts {
		h.auth.attempt("h"+strconv.Itoa(i), "bad")
	}
}

func TestStaleHostsAreSweptSoANewHostIsStillRateLimited(t *testing.T) {
	clk := &testClock{t: time.Unix(1000, 0)}
	h := newTestHandlers(clk)
	fillHosts(h)
	clk.t = clk.t.Add(61 * time.Second)
	for i := range maxFailures {
		if w := tryLogin(h, "new", "bad"); w.Code != http.StatusUnauthorized {
			t.Fatalf("try %d: %d", i, w.Code)
		}
	}
	if w := tryLogin(h, "new", "pw"); w.Code != http.StatusTooManyRequests {
		t.Errorf("sixth: %d, want 429", w.Code)
	}
	if n := len(h.auth.fails); n > 1 {
		t.Errorf("stale hosts kept: %d", n)
	}
}

func TestAFullTableOfFreshHostsRefusesANewHostWithRetryAfter(t *testing.T) {
	clk := &testClock{t: time.Unix(1000, 0)}
	h := newTestHandlers(clk)
	fillHosts(h)
	clk.t = clk.t.Add(20 * time.Second)
	w := tryLogin(h, "new", "pw")
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("got %d, want 429", w.Code)
	}
	if got := w.Header().Get("Retry-After"); got != "40" {
		t.Errorf("Retry-After %q, want 40", got)
	}
	if w := tryLogin(h, "h0", "pw"); w.Code != http.StatusNoContent {
		t.Errorf("known host: %d", w.Code)
	}
}

func TestConcurrentWrongLoginsFromOneHostGetAtMostFiveGuesses(t *testing.T) {
	h := newTestHandlers(&testClock{t: time.Unix(1000, 0)})
	var mu sync.Mutex
	codes := map[int]int{}
	var wg sync.WaitGroup
	for range 50 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c := tryLogin(h, "1.2.3.4", "bad").Code
			mu.Lock()
			codes[c]++
			mu.Unlock()
		}()
	}
	wg.Wait()
	if codes[http.StatusUnauthorized] != maxFailures || codes[http.StatusTooManyRequests] != 50-maxFailures {
		t.Errorf("codes %v, want %d x 401 and %d x 429", codes, maxFailures, 50-maxFailures)
	}
}

func TestConcurrentNewHostsNeverGrowTheTablePastTheCap(t *testing.T) {
	h := newTestHandlers(&testClock{t: time.Unix(1000, 0)})
	var wg sync.WaitGroup
	for i := range 3 * maxFailureHosts {
		wg.Add(1)
		go func() {
			defer wg.Done()
			tryLogin(h, "h"+strconv.Itoa(i), "bad")
		}()
	}
	wg.Wait()
	if n := len(h.auth.fails); n != maxFailureHosts {
		t.Errorf("table has %d hosts, want %d", n, maxFailureHosts)
	}
}

func TestAFailureExactlyOneWindowOldIsForgotten(t *testing.T) {
	clk := &testClock{t: time.Unix(1000, 0)}
	h := newTestHandlers(clk)
	for range maxFailures {
		tryLogin(h, "x", "bad")
	}
	clk.t = clk.t.Add(failureWindow)
	if w := tryLogin(h, "x", "pw"); w.Code != http.StatusNoContent {
		t.Errorf("got %d, want 204", w.Code)
	}
}
