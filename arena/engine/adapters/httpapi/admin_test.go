package httpapi_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lezhnev/arena/engine/adapters/httpapi"
	"github.com/lezhnev/arena/engine/adapters/protocol"
	"github.com/lezhnev/arena/engine/app"
	"github.com/lezhnev/arena/engine/app/apptest"
	"github.com/lezhnev/arena/engine/domain/geo"
	"github.com/lezhnev/arena/engine/domain/sim"
)

const adminPass = "s3cret"

type fakeClock struct {
	mu     sync.Mutex
	t      time.Time
	slept  time.Duration
	sleeps int
}

func (c *fakeClock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *fakeClock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

func (c *fakeClock) sleep(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.slept += d
	c.sleeps++
}

type adminRig struct {
	*rig
	clock *fakeClock
}

func newAdminRig(t *testing.T, password string, trusted ...netip.Prefix) *adminRig {
	t.Helper()
	clk := &fakeClock{t: time.Unix(1000, 0)}
	a, err := app.NewArena(
		app.Config{Rules: sim.DefaultRules(), Map: room, Seed: 3, MinBots: 2},
		app.Deps{Store: &apptest.MemStore{}, Broadcaster: &apptest.Feed{}, IDs: &apptest.Seq{}},
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(a.Close)
	n := 0
	srv := httptest.NewServer(httpapi.New(a, httpapi.Options{Admin: httpapi.AdminOptions{
		Service: a, Password: password, Now: clk.now, Sleep: clk.sleep,
		NewSessionID: func() string { n++; return "sess-" + string(rune('0'+n)) },
		NewSeed:      func() uint64 { return 4242 },
		Beat:         time.Second,
	}, TrustedProxies: trusted}))
	t.Cleanup(srv.Close)
	return &adminRig{rig: &rig{a: a, srv: srv}, clock: clk}
}

// do sends a request with an optional admin cookie value.
func (r *adminRig) do(t *testing.T, method, path, cookie, body string) (int, string, *http.Response) {
	t.Helper()
	req, err := http.NewRequest(method, r.srv.URL+path, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if cookie != "" {
		req.AddCookie(&http.Cookie{Name: "arena_admin", Value: cookie})
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b), resp
}

func (r *adminRig) login(t *testing.T) string {
	t.Helper()
	code, body, resp := r.do(t, "POST", "/api/admin/login", "", `{"password":"`+adminPass+`"}`)
	if code != 204 {
		t.Fatalf("login: %d %s", code, body)
	}
	for _, c := range resp.Cookies() {
		if c.Name == "arena_admin" {
			return c.Value
		}
	}
	t.Fatal("no session cookie")
	return ""
}

func TestAdminIsDisabledWithoutAPassword(t *testing.T) {
	r := newAdminRig(t, "")
	for _, p := range []string{"/api/admin/status", "/api/admin/login", "/api/admin/kick"} {
		for _, m := range []string{"GET", "POST"} {
			if code, _, _ := r.do(t, m, p, "x", `{}`); code != 404 {
				t.Errorf("%s %s: %d, want 404", m, p, code)
			}
		}
	}
}

func hardened(c *http.Cookie) bool {
	return c.HttpOnly && c.SameSite == http.SameSiteStrictMode && c.Path == "/api/admin" && c.Value != ""
}

func TestLoginSetsAHardenedCookie(t *testing.T) {
	r := newAdminRig(t, adminPass)
	_, _, resp := r.do(t, "POST", "/api/admin/login", "", `{"password":"`+adminPass+`"}`)
	if resp.StatusCode != 204 || len(resp.Cookies()) != 1 {
		t.Fatalf("%d %v", resp.StatusCode, resp.Cookies())
	}
	c := resp.Cookies()[0]
	if !hardened(c) || c.Secure || c.MaxAge != 12*3600 {
		t.Errorf("cookie %+v", c)
	}
}

func TestLoginBehindATrustedHTTPSProxySetsASecureCookie(t *testing.T) {
	r := newAdminRig(t, adminPass, netip.MustParsePrefix("127.0.0.0/8"), netip.MustParsePrefix("::1/128"))
	req, _ := http.NewRequest("POST", r.srv.URL+"/api/admin/login", strings.NewReader(`{"password":"`+adminPass+`"}`))
	req.Header.Set("X-Forwarded-Proto", "https")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 204 || len(resp.Cookies()) != 1 || !resp.Cookies()[0].Secure {
		t.Errorf("%d %+v", resp.StatusCode, resp.Cookies())
	}
}

func TestWrongPasswordIs401AfterADelayAndSetsNoCookie(t *testing.T) {
	r := newAdminRig(t, adminPass)
	for _, body := range []string{`{"password":"nope"}`, `{"password":""}`} {
		code, _, resp := r.do(t, "POST", "/api/admin/login", "", body)
		if code != 401 || len(resp.Cookies()) != 0 {
			t.Errorf("%s: %d %v", body, code, resp.Cookies())
		}
	}
	if r.clock.sleeps != 2 || r.clock.slept <= 0 {
		t.Errorf("delay: %d sleeps, %v", r.clock.sleeps, r.clock.slept)
	}
}

func TestBadLoginBodyIs400(t *testing.T) {
	r := newAdminRig(t, adminPass)
	if code, _, _ := r.do(t, "POST", "/api/admin/login", "", `nope`); code != 400 {
		t.Errorf("%d", code)
	}
}

func TestSixthFailureInAMinuteIs429EvenWithTheRightPasswordThenItClears(t *testing.T) {
	r := newAdminRig(t, adminPass)
	for i := range 5 {
		if code, _, _ := r.do(t, "POST", "/api/admin/login", "", `{"password":"x"}`); code != 401 {
			t.Fatalf("try %d: %d", i, code)
		}
	}
	slept := r.clock.sleeps
	code, _, resp := r.do(t, "POST", "/api/admin/login", "", `{"password":"`+adminPass+`"}`)
	if code != 429 || resp.Header.Get("Retry-After") == "" || r.clock.sleeps != slept {
		t.Errorf("blocked: %d, sleeps %d->%d", code, slept, r.clock.sleeps)
	}
	r.clock.advance(61 * time.Second)
	r.login(t)
}

func TestFailuresOfOneMinuteAgoDoNotAccumulate(t *testing.T) {
	r := newAdminRig(t, adminPass)
	for range 4 {
		r.do(t, "POST", "/api/admin/login", "", `{"password":"x"}`)
	}
	r.clock.advance(61 * time.Second)
	for range 4 {
		r.do(t, "POST", "/api/admin/login", "", `{"password":"x"}`)
	}
	r.login(t)
}

func TestAdminRoutesNeedASession(t *testing.T) {
	r := newAdminRig(t, adminPass)
	for _, c := range []struct{ m, p, b string }{
		{"GET", "/api/admin/status", ""}, {"POST", "/api/admin/kick", `{"id":"x"}`},
		{"POST", "/api/admin/reset-board", ""}, {"POST", "/api/admin/min-bots", `{"n":1}`},
		{"POST", "/api/admin/map/preview", `{"width":19,"height":13}`}, {"POST", "/api/admin/map/apply", `{"map":"x"}`},
		{"POST", "/api/admin/timer", `{"minutes":1}`}, {"POST", "/api/admin/start", ""},
	} {
		if code, _, _ := r.do(t, c.m, c.p, "", c.b); code != 401 {
			t.Errorf("%s %s without cookie: %d", c.m, c.p, code)
		}
		if code, _, _ := r.do(t, c.m, c.p, "forged", c.b); code != 401 {
			t.Errorf("%s %s forged cookie: %d", c.m, c.p, code)
		}
	}
}

func TestSessionExpiresAfterTwelveHours(t *testing.T) {
	r := newAdminRig(t, adminPass)
	s := r.login(t)
	r.clock.advance(11*time.Hour + 59*time.Minute)
	if code, _, _ := r.do(t, "GET", "/api/admin/status", s, ""); code != 200 {
		t.Errorf("before expiry: %d", code)
	}
	r.clock.advance(2 * time.Minute)
	if code, _, _ := r.do(t, "GET", "/api/admin/status", s, ""); code != 401 {
		t.Errorf("after expiry: %d", code)
	}
}

func TestLogoutEndsTheSession(t *testing.T) {
	r := newAdminRig(t, adminPass)
	s := r.login(t)
	code, _, resp := r.do(t, "POST", "/api/admin/logout", s, "")
	if code != 204 || len(resp.Cookies()) != 1 || resp.Cookies()[0].MaxAge >= 0 {
		t.Errorf("logout: %d %v", code, resp.Cookies())
	}
	if code, _, _ := r.do(t, "GET", "/api/admin/status", s, ""); code != 401 {
		t.Errorf("after logout: %d", code)
	}
	if code, _, _ := r.do(t, "POST", "/api/admin/logout", "", ""); code != 204 {
		t.Errorf("logout without session: %d", code)
	}
}

func TestMutatingRoutesRefuseGET(t *testing.T) {
	r := newAdminRig(t, adminPass)
	s := r.login(t)
	for _, p := range []string{"/api/admin/kick", "/api/admin/reset-board", "/api/admin/min-bots", "/api/admin/login", "/api/admin/logout", "/api/admin/map/preview", "/api/admin/map/apply", "/api/admin/timer", "/api/admin/start"} {
		code, _, resp := r.do(t, "GET", p, s, "")
		if code != 405 || resp.Header.Get("Allow") != "POST" {
			t.Errorf("GET %s: %d", p, code)
		}
	}
	if code, _, _ := r.do(t, "POST", "/api/admin/status", s, ""); code != 405 {
		t.Errorf("POST status: %d", code)
	}
}

func TestStatusListsBotsMinBotsAndBeat(t *testing.T) {
	r := newAdminRig(t, adminPass)
	s := r.login(t)
	r.join(t, "Ann")
	r.tick(t, 2)
	code, body, _ := r.do(t, "GET", "/api/admin/status", s, "")
	st := decode[protocol.AdminStatus](t, body)
	if code != 200 || st.MinBots != 2 || st.Beat != 2 || len(st.Bots) != 2 {
		t.Fatalf("%d %+v", code, st)
	}
	if got := [2]bool{st.Bots[0].House, st.Bots[1].House}; got != [2]bool{false, true} || st.Bots[0].Name != "Ann" {
		t.Errorf("bots %+v", st.Bots)
	}
}

func TestStatusReportsTheMapSize(t *testing.T) {
	r := newAdminRig(t, adminPass)
	s := r.login(t)
	_, body, _ := r.do(t, "GET", "/api/admin/status", s, "")
	if st := decode[protocol.AdminStatus](t, body); st.MapWidth != 9 || st.MapHeight != 7 {
		t.Errorf("map size %dx%d", st.MapWidth, st.MapHeight)
	}
}

func TestKickOverHTTP(t *testing.T) {
	r := newAdminRig(t, adminPass)
	s := r.login(t)
	j := r.join(t, "Ann")
	r.tick(t, 1)
	cases := []struct {
		body string
		want int
	}{
		{`{"id":"nobody"}`, 404}, {`{"id":"house-1"}`, 409}, {`nope`, 400}, {`{"id":"` + j.BotID + `"}`, 204},
	}
	for _, c := range cases {
		if code, _, _ := r.do(t, "POST", "/api/admin/kick", s, c.body); code != c.want {
			t.Errorf("%s: %d, want %d", c.body, code, c.want)
		}
	}
	if code, _, _ := r.call(t, "GET", "/api/bot", j.Token, ""); code != 401 {
		t.Errorf("kicked token: %d", code)
	}
}

func TestResetBoardOverHTTP(t *testing.T) {
	r := newAdminRig(t, adminPass)
	s := r.login(t)
	j := r.join(t, "Ann")
	r.tick(t, 1)
	if code, _, _ := r.do(t, "POST", "/api/admin/reset-board", s, ""); code != 204 {
		t.Fatalf("%d", code)
	}
	for _, e := range r.a.Leaderboard() {
		if e.Score != 0 || e.Bot == j.BotID && e.Name != "Ann" {
			t.Errorf("%+v", e)
		}
	}
}

func TestMinBotsOverHTTP(t *testing.T) {
	r := newAdminRig(t, adminPass)
	s := r.login(t)
	for _, c := range []struct {
		body string
		want int
	}{{`{"n":4}`, 204}, {`{"n":-1}`, 400}, {`{"n":999}`, 400}, {`{"n":"x"}`, 400}} {
		if code, _, _ := r.do(t, "POST", "/api/admin/min-bots", s, c.body); code != c.want {
			t.Errorf("%s: %d, want %d", c.body, code, c.want)
		}
	}
	if r.a.MinBots() != 4 {
		t.Errorf("MinBots %d", r.a.MinBots())
	}
}

func TestAdminPageIsAnSPARoute(t *testing.T) {
	r := newRig(t, httpapi.Options{})
	code, body, _ := r.call(t, "GET", "/admin", "", "")
	if code != 200 || !strings.Contains(body, "Bot Arena") {
		t.Errorf("%d %s", code, body)
	}
}

func TestMapPreviewGeneratesFromTheGivenOrADrawnSeed(t *testing.T) {
	r := newAdminRig(t, adminPass)
	s := r.login(t)
	code, body, _ := r.do(t, "POST", "/api/admin/map/preview", s, `{"width":19,"height":13,"seed":7}`)
	given := decode[protocol.AdminMapPreview](t, body)
	want, _ := geo.Generate(19, 13, geo.DefaultWallPercent, 7)
	if code != 200 || given.Seed != 7 || given.Map != want {
		t.Fatalf("given seed: %d %+v", code, given)
	}
	code, body, _ = r.do(t, "POST", "/api/admin/map/preview", s, `{"width":11,"height":9}`)
	drawn := decode[protocol.AdminMapPreview](t, body)
	want, _ = geo.Generate(11, 9, geo.DefaultWallPercent, 4242)
	if code != 200 || drawn.Seed != 4242 || drawn.Map != want {
		t.Errorf("drawn seed: %d %+v", code, drawn)
	}
}

func TestMapPreviewUsesTheGivenWallPercent(t *testing.T) {
	r := newAdminRig(t, adminPass)
	s := r.login(t)
	code, body, _ := r.do(t, "POST", "/api/admin/map/preview", s, `{"width":19,"height":13,"walls":12,"seed":7}`)
	dense := decode[protocol.AdminMapPreview](t, body)
	want, _ := geo.Generate(19, 13, 12, 7)
	if code != 200 || dense.Map != want {
		t.Errorf("walls 12: %d %+v", code, dense)
	}
}

func TestMapPreviewRejectsBadSizesAndBodies(t *testing.T) {
	r := newAdminRig(t, adminPass)
	s := r.login(t)
	for _, body := range []string{`{"width":5,"height":13}`, `{"width":19,"height":99}`, `{}`, `nope`, `{"width":19,"height":13,"seed":-1}`, `{"width":19,"height":13,"walls":16}`, `{"width":19,"height":13,"walls":-1}`} {
		if code, _, _ := r.do(t, "POST", "/api/admin/map/preview", s, body); code != 400 {
			t.Errorf("%s: %d", body, code)
		}
	}
}

func TestMapApplyOverHTTP(t *testing.T) {
	r := newAdminRig(t, adminPass)
	s := r.login(t)
	text, _ := geo.Generate(15, 11, geo.DefaultWallPercent, 1)
	body, _ := json.Marshal(protocol.AdminMapApplyRequest{Map: text})
	if code, msg, _ := r.do(t, "POST", "/api/admin/map/apply", s, string(body)); code != 204 {
		t.Fatalf("apply: %d %s", code, msg)
	}
	r.tick(t, 1)
	if w, h := r.a.MapSize(); w != 15 || h != 11 {
		t.Errorf("size %dx%d", w, h)
	}
	cases := []struct {
		body string
		want int
	}{
		{`{"map":"#.#\n#?#"}`, 400}, {`{"map":""}`, 400}, {`{"map":"###\n#A#\n###"}`, 400},
		{`{"map":"###\n#.#\n###"}`, 409}, {`nope`, 400},
	}
	for _, c := range cases {
		if code, _, _ := r.do(t, "POST", "/api/admin/map/apply", s, c.body); code != c.want {
			t.Errorf("%s: %d, want %d", c.body, code, c.want)
		}
	}
}

func (r *adminRig) status(t *testing.T, cookie string) protocol.AdminStatus {
	t.Helper()
	_, body, _ := r.do(t, "GET", "/api/admin/status", cookie, "")
	return decode[protocol.AdminStatus](t, body)
}

func TestStatusReportsTheGameState(t *testing.T) {
	r := newAdminRig(t, adminPass)
	s := r.login(t)
	game := func() [3]int {
		st := r.status(t, s)
		running := 0
		if st.Running {
			running = 1
		}
		return [3]int{running, st.Remaining, st.BeatMs}
	}
	got := [][3]int{game()}
	r.do(t, "POST", "/api/admin/timer", s, `{"minutes":0.05}`) // 3 s = 3 beats
	r.tick(t, 1)
	got = append(got, game())
	r.tick(t, 2)
	got = append(got, game())
	want := [][3]int{{1, -1, 1000}, {1, 2, 1000}, {0, 0, 1000}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("running/remaining/beatMs over time: %v, want %v", got, want)
	}
}

func TestTimerRouteConvertsMinutesToBeats(t *testing.T) {
	r := newAdminRig(t, adminPass)
	s := r.login(t)
	cases := []struct {
		body string
		want int
	}{
		{`{"minutes":2}`, 204}, {`{"minutes":-1}`, 400}, {`{"minutes":100000}`, 400}, {`nope`, 400}, {`{"minutes":0}`, 204},
	}
	for _, c := range cases {
		if code, _, _ := r.do(t, "POST", "/api/admin/timer", s, c.body); code != c.want {
			t.Errorf("%s: %d, want %d", c.body, code, c.want)
		}
	}
	r.do(t, "POST", "/api/admin/timer", s, `{"minutes":2}`)
	if _, rem := r.a.GameState(); rem != 120 {
		t.Errorf("2 minutes at 1 s per beat = 120 beats, got %d", rem)
	}
	r.do(t, "POST", "/api/admin/timer", s, `{"minutes":0}`)
	if _, rem := r.a.GameState(); rem != -1 {
		t.Errorf("cancel left %d", rem)
	}
}

func TestStartRouteIs409WhileRunningAndStartsAfterGameOver(t *testing.T) {
	r := newAdminRig(t, adminPass)
	s := r.login(t)
	if code, _, _ := r.do(t, "POST", "/api/admin/start", s, ""); code != 409 {
		t.Errorf("while running: %d", code)
	}
	if err := r.a.SetTimer(1); err != nil {
		t.Fatal(err)
	}
	r.tick(t, 1)
	if code, _, _ := r.do(t, "POST", "/api/admin/timer", s, `{"minutes":1}`); code != 409 {
		t.Errorf("timer while over: %d", code)
	}
	if code, _, _ := r.do(t, "POST", "/api/admin/reset-board", s, ""); code != 409 {
		t.Errorf("reset while over: %d", code)
	}
	if code, _, _ := r.do(t, "POST", "/api/admin/start", s, ""); code != 204 {
		t.Errorf("start: %d", code)
	}
	if running, _ := r.a.GameState(); !running {
		t.Error("not running after start")
	}
}
