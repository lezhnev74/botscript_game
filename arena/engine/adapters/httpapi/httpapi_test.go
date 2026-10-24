package httpapi_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/lezhnev/arena/engine/adapters/httpapi"
	"github.com/lezhnev/arena/engine/adapters/protocol"
	"github.com/lezhnev/arena/engine/app"
	"github.com/lezhnev/arena/engine/app/apptest"
	"github.com/lezhnev/arena/engine/domain/sim"
)

const room = "#########\n#.......#\n#.......#\n#.......#\n#.......#\n#.......#\n#########\n"

const validBot = "function loop() { num[0] = 9; pin[1] = at(2, 3); wait(); }"

type rig struct {
	a   *app.Arena
	srv *httptest.Server
}

func newRig(t *testing.T, opts httpapi.Options) *rig {
	t.Helper()
	return newRigWith(t, opts, app.Config{})
}

// newRigWith is newRig with the Config fields set in extra (rules, map and seed are fixed).
func newRigWith(t *testing.T, opts httpapi.Options, extra app.Config) *rig {
	t.Helper()
	extra.Rules, extra.Map, extra.Seed = sim.DefaultRules(), room, 3
	a, err := app.NewArena(
		extra,
		app.Deps{Store: &apptest.MemStore{}, Broadcaster: &apptest.Feed{}, IDs: &apptest.Seq{}},
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(a.Close)
	srv := httptest.NewServer(httpapi.New(a, opts))
	t.Cleanup(srv.Close)
	return &rig{a: a, srv: srv}
}

// call sends a request and returns the status and body.
func (r *rig) call(t *testing.T, method, path, token, body string) (int, string, http.Header) {
	t.Helper()
	req, err := http.NewRequest(method, r.srv.URL+path, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b), resp.Header
}

func (r *rig) join(t *testing.T, name string) protocol.JoinResponse {
	t.Helper()
	code, body, _ := r.call(t, "POST", "/api/join", "", `{"name":"`+name+`"}`)
	var j protocol.JoinResponse
	if code != 200 || json.Unmarshal([]byte(body), &j) != nil {
		t.Fatalf("join: %d %s", code, body)
	}
	return j
}

func (r *rig) state(t *testing.T, token string) protocol.BotState {
	t.Helper()
	_, body, _ := r.call(t, "GET", "/api/state", token, "")
	return decode[protocol.BotState](t, body)
}

func (r *rig) tick(t *testing.T, n int) {
	t.Helper()
	for range n {
		if err := r.a.Tick(); err != nil {
			t.Fatal(err)
		}
	}
}

func decode[T any](t *testing.T, body string) T {
	t.Helper()
	var v T
	if err := json.Unmarshal([]byte(body), &v); err != nil {
		t.Fatalf("%s: %v", body, err)
	}
	return v
}

func TestJoinGivesTokenAndBotID(t *testing.T) {
	r := newRig(t, httpapi.Options{})
	j := r.join(t, "Ann")
	if j.Token != "tok-1" || j.BotID != "p1" {
		t.Errorf("%+v", j)
	}
}

func TestJoinRejectsBadInput(t *testing.T) {
	r := newRig(t, httpapi.Options{})
	for _, body := range []string{`{"name":""}`, `{"name":"` + strings.Repeat("x", 25) + `"}`, `not json`, ``, `{"name":"a","extra":1}`} {
		code, resp, _ := r.call(t, "POST", "/api/join", "", body)
		if code != 400 || decode[protocol.ErrorBody](t, resp).Error == "" {
			t.Errorf("%q: %d %s", body, code, resp)
		}
	}
}

func TestJoinOnAFullArenaIs409(t *testing.T) {
	a, err := app.NewArena(
		app.Config{Rules: sim.DefaultRules(), Map: "###\n#.#\n###\n", Seed: 1},
		app.Deps{Store: &apptest.MemStore{}, Broadcaster: &apptest.Feed{}, IDs: &apptest.Seq{}},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	srv := httptest.NewServer(httpapi.New(a, httpapi.Options{}))
	defer srv.Close()
	for want := range 2 {
		resp, err := http.Post(srv.URL+"/api/join", "application/json", strings.NewReader(`{"name":"x"}`))
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if code := []int{200, 409}[want]; resp.StatusCode != code {
			t.Errorf("join %d: %d, want %d", want+1, resp.StatusCode, code)
		}
	}
}

func TestGetOwnBot(t *testing.T) {
	r := newRig(t, httpapi.Options{})
	j := r.join(t, "Ann")
	for _, path := range []string{"/api/bot", "/api/bot/" + j.BotID} {
		code, body, _ := r.call(t, "GET", path, j.Token, "")
		b := decode[protocol.BotBody](t, body)
		if code != 200 || b.ID != j.BotID || b.Name != "Ann" || !strings.Contains(b.Source, "loop") {
			t.Errorf("%s: %d %s", path, code, body)
		}
	}
}

func TestPutOwnBotThenGetShowsIt(t *testing.T) {
	r := newRig(t, httpapi.Options{})
	j := r.join(t, "Ann")
	if code, body, _ := r.call(t, "PUT", "/api/bot/"+j.BotID, j.Token, validBot); code != 204 {
		t.Fatalf("put: %d %s", code, body)
	}
	_, body, _ := r.call(t, "GET", "/api/bot", j.Token, "")
	if got := decode[protocol.BotBody](t, body).Source; got != validBot {
		t.Errorf("source %q", got)
	}
}

func TestMissingOrUnknownTokenIs401(t *testing.T) {
	r := newRig(t, httpapi.Options{})
	r.join(t, "Ann")
	for _, c := range []struct{ method, path, token string }{
		{"GET", "/api/bot", ""}, {"GET", "/api/bot", "wrong"}, {"PUT", "/api/bot", "wrong"},
		{"GET", "/api/bot/p1", "wrong"}, {"GET", "/api/state", ""},
	} {
		code, body, hdr := r.call(t, c.method, c.path, c.token, validBot)
		if code != 401 || decode[protocol.ErrorBody](t, body).Error == "" || hdr.Get("WWW-Authenticate") != "Bearer" {
			t.Errorf("%+v: %d %s", c, code, body)
		}
	}
}

func TestOtherAuthSchemeIs401(t *testing.T) {
	r := newRig(t, httpapi.Options{})
	j := r.join(t, "Ann")
	req, _ := http.NewRequest("GET", r.srv.URL+"/api/bot", nil)
	req.Header.Set("Authorization", "Basic "+j.Token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 401 {
		t.Errorf("status %d", resp.StatusCode)
	}
}

func TestSomeoneElsesBotIs403(t *testing.T) {
	r := newRig(t, httpapi.Options{})
	ann, bob := r.join(t, "Ann"), r.join(t, "Bob")
	for _, method := range []string{"GET", "PUT"} {
		code, body, _ := r.call(t, method, "/api/bot/"+bob.BotID, ann.Token, validBot)
		if code != 403 || decode[protocol.ErrorBody](t, body).Error == "" {
			t.Errorf("%s: %d %s", method, code, body)
		}
	}
	_, body, _ := r.call(t, "GET", "/api/bot", bob.Token, "")
	if strings.Contains(decode[protocol.BotBody](t, body).Source, "a = 9") {
		t.Error("Bob's bot was changed by Ann")
	}
}

func TestInvalidProgramIs422WithLineColMsgHint(t *testing.T) {
	r := newRig(t, httpapi.Options{})
	j := r.join(t, "Ann")
	code, body, _ := r.call(t, "PUT", "/api/bot", j.Token, "function loop( {\n  wait();\n}")
	v := decode[protocol.ValidationBody](t, body)
	if code != 422 || len(v.Errors) == 0 {
		t.Fatalf("%d %s", code, body)
	}
	e := v.Errors[0]
	if got := []bool{e.Line > 0, e.Col > 0, e.Msg != "", e.Hint != ""}; slices.Contains(got, false) {
		t.Errorf("error %+v", e)
	}
	for _, key := range []string{`"line"`, `"col"`, `"msg"`, `"hint"`, `"errors"`} {
		if !strings.Contains(body, key) {
			t.Errorf("body lacks %s: %s", key, body)
		}
	}
}

func TestSecondSwapTooSoonIs429WithRetryBeats(t *testing.T) {
	r := newRig(t, httpapi.Options{})
	j := r.join(t, "Ann")
	if code, _, _ := r.call(t, "PUT", "/api/bot", j.Token, validBot); code != 204 {
		t.Fatalf("first put: %d", code)
	}
	code, body, _ := r.call(t, "PUT", "/api/bot", j.Token, validBot)
	e := decode[protocol.ErrorBody](t, body)
	if code != 429 || e.RetryBeats <= 0 || e.Error == "" {
		t.Errorf("%d %s", code, body)
	}
}

func TestOversizedSourceIs413(t *testing.T) {
	r := newRig(t, httpapi.Options{})
	j := r.join(t, "Ann")
	code, _, _ := r.call(t, "PUT", "/api/bot", j.Token, strings.Repeat("x", 70_000))
	if code != 413 {
		t.Errorf("status %d", code)
	}
}

func TestStateShowsMemoryAfterTheBotEnters(t *testing.T) {
	r := newRig(t, httpapi.Options{})
	j := r.join(t, "Ann")
	before := r.state(t, j.Token)
	if before.InArena || before.ID != j.BotID {
		t.Errorf("before: %+v", before)
	}
	r.call(t, "PUT", "/api/bot", j.Token, validBot)
	r.tick(t, 2)
	after := r.state(t, j.Token)
	if !after.InArena || after.Num[0] != 9 || len(after.Trail) == 0 {
		t.Errorf("after: %+v", after)
	}
}

func TestStateShowsPins(t *testing.T) {
	r := newRig(t, httpapi.Options{})
	j := r.join(t, "Ann")
	r.call(t, "PUT", "/api/bot", j.Token, validBot)
	r.tick(t, 2)
	pin := r.state(t, j.Token).Pin
	if p := pin[1]; p == nil || p.X != 2 || p.Y != 3 || pin[0] != nil {
		t.Errorf("pins: %+v", pin)
	}
}

func TestLeaderboardIsPublic(t *testing.T) {
	r := newRig(t, httpapi.Options{})
	code, body, _ := r.call(t, "GET", "/api/leaderboard", "", "")
	if code != 200 || strings.TrimSpace(body) != "[]" {
		t.Errorf("empty: %d %s", code, body)
	}
	r.join(t, "Ann")
	if err := r.a.Tick(); err != nil {
		t.Fatal(err)
	}
	_, body, _ = r.call(t, "GET", "/api/leaderboard", "", "")
	rows := decode[[]protocol.LeaderboardEntry](t, body)
	if len(rows) != 1 || rows[0].Name != "Ann" || rows[0].Bot != "p1" {
		t.Errorf("%s", body)
	}
}

func TestLastGameBoardIsPublicAndEmptyUntilAGameEnds(t *testing.T) {
	r := newRig(t, httpapi.Options{})
	lastRows := func() []string {
		code, body, _ := r.call(t, "GET", "/api/leaderboard/last", "", "")
		out := []string{fmt.Sprint(code)}
		for _, e := range decode[[]protocol.LeaderboardEntry](t, body) {
			out = append(out, e.Name)
		}
		return out
	}
	before := lastRows()
	r.join(t, "Ann")
	if err := r.a.SetTimer(1); err != nil {
		t.Fatal(err)
	}
	if err := r.a.Tick(); err != nil {
		t.Fatal(err)
	}
	post, _, _ := r.call(t, "POST", "/api/leaderboard/last", "", "")
	got := fmt.Sprint(before, lastRows(), post)
	if want := "[200] [200 Ann] 405"; got != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

func TestWrongMethodIs405(t *testing.T) {
	r := newRig(t, httpapi.Options{})
	if code, _, _ := r.call(t, "GET", "/api/join", "", ""); code != 405 {
		t.Errorf("status %d", code)
	}
}

func TestGetProfileIs405NotABotLookup(t *testing.T) {
	r := newRig(t, httpapi.Options{})
	j := r.join(t, "Ann")
	code, _, h := r.call(t, "GET", "/api/bot/profile", j.Token, "")
	if code != 405 || h.Get("Allow") != "PUT" {
		t.Errorf("%d Allow=%q", code, h.Get("Allow"))
	}
}

func TestUnknownAPIPathIs404JSON(t *testing.T) {
	r := newRig(t, httpapi.Options{})
	code, body, _ := r.call(t, "GET", "/api/nope", "", "")
	if code != 404 || decode[protocol.ErrorBody](t, body).Error == "" {
		t.Errorf("%d %s", code, body)
	}
}

func TestWSRouteIsDelegated(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ws here")) })
	r := newRig(t, httpapi.Options{WS: h})
	if _, body, _ := r.call(t, "GET", "/ws", "", ""); body != "ws here" {
		t.Errorf("body %q", body)
	}
}

// failing is a Service whose every call fails with an unexpected error.
type failing struct{ httpapi.Service }

func (failing) Join(string) (string, string, error) { return "", "", io.ErrUnexpectedEOF }

func TestUnexpectedErrorsAre500WithoutDetails(t *testing.T) {
	srv := httptest.NewServer(httpapi.New(failing{}, httpapi.Options{}))
	defer srv.Close()
	resp, err := http.Post(srv.URL+"/api/join", "application/json", strings.NewReader(`{"name":"x"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 500 || strings.Contains(string(b), "EOF") {
		t.Errorf("%d %s", resp.StatusCode, b)
	}
}

func TestClosedArenaIs503(t *testing.T) {
	r := newRig(t, httpapi.Options{})
	r.a.Close()
	if code, _, _ := r.call(t, "POST", "/api/join", "", `{"name":"x"}`); code != 503 {
		t.Errorf("status %d", code)
	}
}

func writeDist(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string]string{"index.html": "<html>SPA</html>", "assets/app.js": "console.log(1)"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestSPARoutesServeIndexAndAssetsServeThemselves(t *testing.T) {
	r := newRig(t, httpapi.Options{WebDir: writeDist(t)})
	for _, path := range []string{"/", "/bot", "/leaderboard", "/index.html"} {
		code, body, hdr := r.call(t, "GET", path, "", "")
		if code != 200 || body != "<html>SPA</html>" || !strings.HasPrefix(hdr.Get("Content-Type"), "text/html") {
			t.Errorf("%s: %d %q %v", path, code, body, hdr.Get("Content-Type"))
		}
	}
	code, body, _ := r.call(t, "GET", "/assets/app.js", "", "")
	if code != 200 || body != "console.log(1)" {
		t.Errorf("asset: %d %q", code, body)
	}
}

func TestStaticMisses(t *testing.T) {
	r := newRig(t, httpapi.Options{WebDir: writeDist(t)})
	for _, path := range []string{"/nope", "/assets", "/assets/", "/assets/missing.js", "/%2e%2e/secret"} {
		if code, _, _ := r.call(t, "GET", path, "", ""); code != 404 {
			t.Errorf("%s: %d", path, code)
		}
	}
}

func TestMissingDistShowsAPlaceholder(t *testing.T) {
	for _, dir := range []string{"", filepath.Join(t.TempDir(), "nodist")} {
		r := newRig(t, httpapi.Options{WebDir: dir})
		code, body, hdr := r.call(t, "GET", "/", "", "")
		if code != 200 || !strings.Contains(body, "web/dist") || !strings.HasPrefix(hdr.Get("Content-Type"), "text/html") {
			t.Errorf("%q: %d %s", dir, code, body)
		}
	}
}

const goodProfile = `{"name":"Zed","look":{"color":"#00ff7f","body":"ghost","eyes":"x","mouth":"zigzag","hat":"horns"}}`

func TestGetBotReturnsItsLook(t *testing.T) {
	r := newRig(t, httpapi.Options{})
	j := r.join(t, "Ann")
	_, body, _ := r.call(t, "GET", "/api/bot", j.Token, "")
	want := protocol.FromLook(app.DefaultLook(j.BotID))
	if got := decode[protocol.BotBody](t, body).Look; got != want {
		t.Errorf("look %+v, want %+v", got, want)
	}
}

func TestPutProfileRenamesAndRestyles(t *testing.T) {
	r := newRig(t, httpapi.Options{})
	j := r.join(t, "Ann")
	if code, body, _ := r.call(t, "PUT", "/api/bot/profile", j.Token, goodProfile); code != 204 {
		t.Fatalf("put: %d %s", code, body)
	}
	_, body, _ := r.call(t, "GET", "/api/bot", j.Token, "")
	b := decode[protocol.BotBody](t, body)
	if b.Name != "Zed" || b.Look.Color != "#00ff7f" || b.Look.Hat != "horns" {
		t.Errorf("%+v", b)
	}
}

func TestPutProfileRejectsBadInputWith400(t *testing.T) {
	r := newRig(t, httpapi.Options{})
	j := r.join(t, "Ann")
	for name, body := range map[string]string{
		"bad part":     strings.Replace(goodProfile, "ghost", "blob", 1),
		"bad color":    strings.Replace(goodProfile, "#00ff7f", "green", 1),
		"empty name":   strings.Replace(goodProfile, "Zed", " ", 1),
		"missing look": `{"name":"Zed"}`,
		"not json":     `nope`,
		"extra field":  `{"name":"Zed","x":1}`,
	} {
		code, resp, _ := r.call(t, "PUT", "/api/bot/profile", j.Token, body)
		if code != 400 || decode[protocol.ErrorBody](t, resp).Error == "" {
			t.Errorf("%s: %d %s", name, code, resp)
		}
	}
	_, resp, _ := r.call(t, "PUT", "/api/bot/profile", j.Token, strings.Replace(goodProfile, "ghost", "blob", 1))
	if !strings.Contains(resp, "bad look: body") {
		t.Errorf("the error must name the field: %s", resp)
	}
}

// The web editor routes errors to fields by these exact texts (look-controller.ts fieldOf).
func TestPutProfileErrorTextsArePinnedForTheWeb(t *testing.T) {
	r := newRig(t, httpapi.Options{})
	j := r.join(t, "Ann")
	for field, old := range map[string][2]string{"color": {"#00ff7f", "green"}, "body": {"ghost", "blob"}} {
		_, resp, _ := r.call(t, "PUT", "/api/bot/profile", j.Token, strings.Replace(goodProfile, old[0], old[1], 1))
		if got := decode[protocol.ErrorBody](t, resp).Error; got != "bad look: "+field {
			t.Errorf("%s: %q", field, got)
		}
	}
	_, resp, _ := r.call(t, "PUT", "/api/bot/profile", j.Token, strings.Replace(goodProfile, "Zed", " ", 1))
	if got := decode[protocol.ErrorBody](t, resp).Error; got != "name must be 1 to 24 characters" {
		t.Errorf("name: %q", got)
	}
}

func TestPutProfileNeedsAKnownToken(t *testing.T) {
	r := newRig(t, httpapi.Options{})
	for _, token := range []string{"", "nope"} {
		if code, _, _ := r.call(t, "PUT", "/api/bot/profile", token, goodProfile); code != 401 {
			t.Errorf("token %q: %d", token, code)
		}
	}
}

func checkOpts() httpapi.Options {
	return httpapi.Options{Check: app.Check, BaseURL: "http://example.test"}
}

func TestConfigReportsTheBaseURL(t *testing.T) {
	r := newRig(t, checkOpts())
	code, body, _ := r.call(t, "GET", "/api/config", "", "")
	if got := decode[protocol.ConfigBody](t, body); code != 200 || got.BaseURL != "http://example.test" {
		t.Errorf("%d %s", code, body)
	}
	empty := newRig(t, httpapi.Options{})
	if _, body, _ := empty.call(t, "GET", "/api/config", "", ""); !strings.Contains(body, `"baseUrl":""`) {
		t.Errorf("unset base url: %s", body)
	}
}

func TestCheckAnswersABrokenProgramWithErrors(t *testing.T) {
	r := newRig(t, checkOpts())
	code, body, h := r.call(t, "POST", "/api/check", "", "function loop() { jump(N); }")
	got := decode[protocol.CheckResponse](t, body)
	if code != 200 || got.Ok || got.Trial != nil || len(got.Errors) == 0 {
		t.Fatalf("%d %s", code, body)
	}
	if h.Get("Content-Type") != "application/json" {
		t.Errorf("content type %q", h.Get("Content-Type"))
	}
	checkFields(t, got.Errors[0])
}

func checkFields(t *testing.T, e protocol.CheckError) {
	t.Helper()
	for name, ok := range map[string]bool{"line": e.Line == 1, "col": e.Col > 0, "message": e.Message != "", "hint": e.Hint != ""} {
		if !ok {
			t.Errorf("error %+v: bad %s", e, name)
		}
	}
}

func TestCheckPlaysATrialOfAGoodProgramByPostAndGet(t *testing.T) {
	r := newRig(t, checkOpts())
	src := "function loop() { scan(E); move(N); fire(E); }"
	_, post, _ := r.call(t, "POST", "/api/check", "", src)
	_, get, _ := r.call(t, "GET", "/api/check?src="+url.QueryEscape(src), "", "")
	got := decode[protocol.CheckResponse](t, post)
	if !got.Ok || got.Trial == nil || got.Trial.Beats != 100 || len(got.Errors) != 0 || got.Trial.Warnings == nil {
		t.Fatalf("%s", post)
	}
	if post != get {
		t.Errorf("POST and GET differ:\n%s\n%s", post, get)
	}
}

func TestCheckRejectsMissingAndOversizedPrograms(t *testing.T) {
	r := newRig(t, checkOpts())
	big := strings.Repeat("x", 16<<10+1)
	for name, c := range map[string]struct {
		method, path, body string
		want               int
	}{
		"get without src":   {"GET", "/api/check", "", 400},
		"get too large":     {"GET", "/api/check?src=" + big, "", 400},
		"post too large":    {"POST", "/api/check", big, 413},
		"post at the limit": {"POST", "/api/check", strings.Repeat("x", 16<<10), 200},
		"other method":      {"PUT", "/api/check", "x", 405},
	} {
		if code, body, _ := r.call(t, c.method, c.path, "", c.body); code != c.want {
			t.Errorf("%s: %d %s, want %d", name, code, body, c.want)
		}
	}
}

func TestCheckIsAbsentWithoutAChecker(t *testing.T) {
	r := newRig(t, httpapi.Options{})
	if code, _, _ := r.call(t, "POST", "/api/check", "", "x"); code != 404 {
		t.Errorf("got %d, want 404", code)
	}
}

func TestCheckFailureIsAnInternalError(t *testing.T) {
	r := newRig(t, httpapi.Options{Check: func(string) (app.CheckResult, error) { return app.CheckResult{}, io.ErrUnexpectedEOF }})
	code, body, _ := r.call(t, "POST", "/api/check", "", "x")
	if code != 500 || strings.Contains(body, "EOF") {
		t.Errorf("%d %s", code, body)
	}
}
