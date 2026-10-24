package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/netip"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/lezhnev/arena/engine/adapters/protocol"
	"github.com/lezhnev/arena/engine/adapters/store"
	"github.com/lezhnev/arena/engine/adapters/ws"
	"github.com/lezhnev/arena/engine/app"
	"github.com/lezhnev/arena/engine/app/apptest"
	"github.com/lezhnev/arena/engine/domain/geo"
	"github.com/lezhnev/arena/engine/domain/sim"
)

func TestParseConfigDefaults(t *testing.T) {
	c, err := parseConfig(nil, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	want := config{addr: ":8080", beat: 1000 * time.Millisecond, minBots: 5, maxPlay: 50, maxStay: 24 * time.Hour, reboot: 10 * time.Second, coinPts: 1, seed: 1, dataDir: "data", webDir: filepath.Join("..", "web", "dist"), envFile: ".env"}
	if !reflect.DeepEqual(c, want) {
		t.Errorf("%+v", c)
	}
}

func TestParseConfigFlags(t *testing.T) {
	c, err := parseConfig([]string{"-addr", "127.0.0.1:9", "-beat", "50", "-seed", "7", "-min-bots", "0", "-max-players", "9", "-max-stay", "90m", "-reboot", "2500ms", "-coin-points", "3", "-data", "d", "-web", "w", "-map", "m.txt", "-origin", "localhost:5173", "-origin", "x.test", "-env", "e", "-trust-proxy", "172.16.0.0/12", "-trust-proxy", "10.0.0.7"}, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	want := config{addr: "127.0.0.1:9", beat: 50 * time.Millisecond, maxPlay: 9, maxStay: 90 * time.Minute, reboot: 2500 * time.Millisecond, coinPts: 3, seed: 7, dataDir: "d", webDir: "w", mapFile: "m.txt", origins: []string{"localhost:5173", "x.test"}, envFile: "e", proxies: []netip.Prefix{netip.MustParsePrefix("172.16.0.0/12"), netip.MustParsePrefix("10.0.0.7/32")}}
	if !reflect.DeepEqual(c, want) {
		t.Errorf("%+v", c)
	}
}

func TestParseConfigRejects(t *testing.T) {
	for _, args := range [][]string{{"-beat", "0"}, {"-min-bots", "-1"}, {"-max-players", "-1"}, {"-max-stay", "-1h"}, {"-max-stay", "soon"}, {"-reboot", "-1s"}, {"-reboot", "soon"}, {"-trust-proxy", "nope"}, {"-coin-points", "-1"}, {"-nope"}, {"extra"}} {
		if _, err := parseConfig(args, io.Discard); err == nil {
			t.Errorf("%v accepted", args)
		}
	}
}

func TestMaxStayIsConvertedToBeatsRoundedUpAndZeroMeansNoLimit(t *testing.T) {
	for _, tc := range []struct {
		stay, beat time.Duration
		want       int
	}{
		{24 * time.Hour, time.Second, 86400},
		{time.Hour, 1500 * time.Millisecond, 2400},
		{time.Hour + time.Millisecond, time.Second, 3601},
		{0, time.Second, 0},
	} {
		c := config{maxStay: tc.stay, beat: tc.beat}
		if got := c.stayBeats(); got != tc.want {
			t.Errorf("%v at %v: %d, want %d", tc.stay, tc.beat, got, tc.want)
		}
	}
	if c, err := parseConfig([]string{"-max-stay", "0"}, io.Discard); err != nil || c.maxStay != 0 {
		t.Errorf("-max-stay 0: %+v %v", c, err)
	}
}

func TestRebootIsConvertedToBeatsRoundedUpAndZeroMeansOff(t *testing.T) {
	for _, tc := range []struct {
		reboot, beat time.Duration
		want         int
	}{
		{10 * time.Second, time.Second, 10},
		{10 * time.Second, 300 * time.Millisecond, 34},
		{0, time.Second, 0},
	} {
		c := config{reboot: tc.reboot, beat: tc.beat}
		if got := c.rebootBeats(); got != tc.want {
			t.Errorf("%v at %v: %d, want %d", tc.reboot, tc.beat, got, tc.want)
		}
	}
	if c, err := parseConfig([]string{"-reboot", "0"}, io.Discard); err != nil || c.reboot != 0 {
		t.Errorf("-reboot 0: %+v %v", c, err)
	}
}

func TestConfigPointsOverrideTheCoinOnly(t *testing.T) {
	c, err := parseConfig([]string{"-coin-points", "5"}, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	want := app.DefaultPoints()
	want.Coin = 5
	if got := c.points(); *got != want {
		t.Errorf("%+v, want %+v", *got, want)
	}
}

func TestDefaultMapIsAValidRoom(t *testing.T) {
	dir := t.TempDir()
	if err := run(cancelled(), []string{"-addr", "127.0.0.1:0", "-data", dir}, strings.NewReader(""), io.Discard, io.Discard); err != nil {
		t.Fatal(err)
	}
}

func TestBadMapFileFails(t *testing.T) {
	err := run(cancelled(), []string{"-addr", "127.0.0.1:0", "-data", t.TempDir(), "-map", "/nonexistent"}, strings.NewReader(""), io.Discard, io.Discard)
	if err == nil {
		t.Error("want an error")
	}
	bad := filepath.Join(t.TempDir(), "bad.txt")
	_ = os.WriteFile(bad, []byte("not a map\n##\n#"), 0o644)
	if err := run(cancelled(), []string{"-addr", "127.0.0.1:0", "-data", t.TempDir(), "-map", bad}, strings.NewReader(""), io.Discard, io.Discard); err == nil {
		t.Error("a broken map must fail")
	}
}

func TestBusyAddressFails(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	addr, done := start(t, ctx, t.TempDir())
	err := run(cancelled(), []string{"-addr", addr, "-data", t.TempDir()}, strings.NewReader(""), io.Discard, io.Discard)
	if err == nil {
		t.Error("second listener on the same address should fail")
	}
	cancel()
	<-done
}

func cancelled() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	return ctx
}

// start runs the server on a free port; it returns the address and a channel
// that yields run's error when it stops.
func start(t *testing.T, ctx context.Context, dir string) (string, <-chan error) {
	t.Helper()
	return startWith(t, ctx, dir, strings.NewReader(""))
}

// startWith is start with a given stdin (the admin console).
func startWith(t *testing.T, ctx context.Context, dir string, stdin io.Reader) (string, <-chan error) {
	t.Helper()
	pr, pw := io.Pipe()
	done := make(chan error, 1)
	go func() {
		done <- run(ctx, []string{"-addr", "127.0.0.1:0", "-beat", "10", "-data", dir, "-min-bots", "2", "-env", filepath.Join(dir, ".env")}, stdin, pw, io.Discard)
		pw.Close()
	}()
	line, err := bufio.NewReader(pr).ReadString('\n')
	if err != nil {
		t.Fatalf("no listen line: %v", err)
	}
	go io.Copy(io.Discard, pr) //nolint:errcheck
	_, addr, _ := strings.Cut(strings.TrimSpace(line), "http://")
	return addr, done
}

func api(t *testing.T, method, url, token, body string) (int, []byte) {
	t.Helper()
	req, _ := http.NewRequest(method, url, strings.NewReader(body))
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, b
}

func TestServerJoinsAndKeepsTokensAcrossRestart(t *testing.T) {
	dir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	addr, done := start(t, ctx, dir)
	j := joinAs(t, addr, "Ann")
	waitFile(t, filepath.Join(dir, "bots.json"))
	if code, body := api(t, "GET", "http://"+addr+"/api/leaderboard", "", ""); code != 200 || !strings.Contains(string(body), "Ann") {
		t.Errorf("leaderboard %d %s", code, body)
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}

	ctx2, cancel2 := context.WithCancel(context.Background())
	addr2, done2 := start(t, ctx2, dir)
	code, body := api(t, "GET", "http://"+addr2+"/api/bot", j.Token, "")
	if code != 200 || !strings.Contains(string(body), j.BotID) {
		t.Errorf("after restart %d %s", code, body)
	}
	cancel2()
	<-done2
}

func TestServerServesThePlaceholderAndWS(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	addr, done := start(t, ctx, t.TempDir())
	if code, body := api(t, "GET", "http://"+addr+"/", "", ""); code != 200 || !strings.Contains(string(body), "Bot Arena") {
		t.Errorf("index %d %s", code, body)
	}
	if code, _ := api(t, "GET", "http://"+addr+"/ws", "", ""); code != http.StatusUpgradeRequired && code != http.StatusBadRequest {
		t.Errorf("/ws without upgrade: %d", code)
	}
	cancel()
	<-done
}

func joinAs(t *testing.T, addr, name string) protocol.JoinResponse {
	t.Helper()
	code, body := api(t, "POST", "http://"+addr+"/api/join", "", `{"name":"`+name+`"}`)
	var j protocol.JoinResponse
	if err := json.Unmarshal(body, &j); code != 200 || err != nil || j.Token == "" {
		t.Fatalf("join %d %s", code, body)
	}
	return j
}

func waitFile(t *testing.T, path string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(path); err == nil {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("%s never written", path)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// startSlow runs the server with a very long beat, so no beat ever saves.
func startSlow(t *testing.T, ctx context.Context, dir string) (string, <-chan error) {
	t.Helper()
	pr, pw := io.Pipe()
	done := make(chan error, 1)
	go func() {
		done <- run(ctx, []string{"-addr", "127.0.0.1:0", "-beat", "3600000", "-data", dir, "-min-bots", "0"}, strings.NewReader(""), pw, io.Discard)
		pw.Close()
	}()
	line, err := bufio.NewReader(pr).ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	go io.Copy(io.Discard, pr) //nolint:errcheck
	_, addr, _ := strings.Cut(strings.TrimSpace(line), "http://")
	return addr, done
}

func TestJoinAndEditSurviveShutdownWithoutAnyBeat(t *testing.T) {
	dir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	addr, done := startSlow(t, ctx, dir)
	j := joinAs(t, addr, "Ann")
	const edited = "function loop() { num[0] = 3; wait(); }"
	if code, body := api(t, "PUT", "http://"+addr+"/api/bot", j.Token, edited); code != 204 {
		t.Fatalf("put %d %s", code, body)
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	ctx2, cancel2 := context.WithCancel(context.Background())
	addr2, done2 := startSlow(t, ctx2, dir)
	defer func() { // the server flushes into dir on shutdown; wait before TempDir is removed
		cancel2()
		<-done2
	}()
	code, body := api(t, "GET", "http://"+addr2+"/api/bot", j.Token, "")
	if code != 200 || !strings.Contains(string(body), "num[0] = 3") {
		t.Errorf("after restart %d %s", code, body)
	}
}

func newServer(t *testing.T, h http.Handler, grace time.Duration) (server, *apptest.MemStore) {
	t.Helper()
	st := &apptest.MemStore{}
	hub := ws.New(ws.Options{}, app.Update{})
	a, err := app.NewArena(
		app.Config{Rules: sim.DefaultRules(), Map: defaultMap, Seed: 1},
		app.Deps{Store: st, Broadcaster: hub, IDs: &apptest.Seq{}},
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(a.Close)
	return server{handler: h, arena: a, hub: hub, beat: time.Hour, grace: grace}, st
}

func TestServeFailureStillStopsBeatsAndSavesPending(t *testing.T) {
	s, st := newServer(t, http.NotFoundHandler(), time.Second)
	if _, _, err := s.arena.Join("Ann"); err != nil {
		t.Fatal(err)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ln.Close() // Serve fails at once
	if err := s.serve(context.Background(), ln); err == nil {
		t.Fatal("want the listener error")
	}
	if len(st.Bots) != 1 {
		t.Errorf("pending join not saved: %+v", st.Bots)
	}
	if err := s.arena.Tick(); err != nil && !errors.Is(err, app.ErrClosed) {
		t.Error(err)
	}
}

func TestShutdownCutsOffRequestsAfterTheGrace(t *testing.T) {
	release := make(chan struct{})
	entered := make(chan struct{})
	h := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		close(entered)
		<-release
	})
	s, _ := newServer(t, h, 20*time.Millisecond)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- s.serve(ctx, ln) }()
	go func() {
		resp, err := http.Get("http://" + ln.Addr().String() + "/")
		if err == nil {
			resp.Body.Close()
		}
	}()
	<-entered
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("serve: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("serve hung on an open request")
	}
	close(release)
}

func TestWebDirIsAnnouncedOrWarnedAbout(t *testing.T) {
	var out, errOut strings.Builder
	dist := t.TempDir()
	_ = os.WriteFile(filepath.Join(dist, "index.html"), []byte("x"), 0o644)
	announceWeb(dist, &out, &errOut)
	if !strings.Contains(out.String(), dist) || errOut.Len() != 0 {
		t.Errorf("out %q err %q", out.String(), errOut.String())
	}
	out.Reset()
	announceWeb(filepath.Join(dist, "nope"), &out, &errOut)
	if out.Len() != 0 || !strings.Contains(errOut.String(), "warning") {
		t.Errorf("out %q err %q", out.String(), errOut.String())
	}
}

// The tuned arena (TUNING.md): the default room is the 19x13 map the tuning
// sweeps ran on, with no bots in it.
func TestDefaultMapIsTheTunedArena(t *testing.T) {
	m, err := geo.ParseMap(defaultMap)
	if err != nil {
		t.Fatal(err)
	}
	if m.Grid.Width() != 19 || m.Grid.Height() != 13 || len(m.Bots) != 0 {
		t.Errorf("default arena is %dx%d with %d bots", m.Grid.Width(), m.Grid.Height(), len(m.Bots))
	}
	tuned, err := os.ReadFile("../../testdata/maps/19x13.map")
	if err != nil || string(tuned) != defaultMap {
		t.Errorf("defaultMap differs from testdata/maps/19x13.map (%v)", err)
	}
}

func TestConsoleKickRevokesTheToken(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	inR, inW := io.Pipe()
	addr, done := startWith(t, ctx, t.TempDir(), inR)
	j := joinAs(t, addr, "Ann")
	if code, _ := api(t, "GET", "http://"+addr+"/api/bot", j.Token, ""); code != 200 {
		t.Fatalf("before kick: %d", code)
	}
	if _, err := io.WriteString(inW, "kick "+j.BotID+"\n"); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		code, _ := api(t, "GET", "http://"+addr+"/api/bot", j.Token, "")
		if code == http.StatusUnauthorized {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("token still works: %d", code)
		}
		time.Sleep(5 * time.Millisecond)
	}
	inW.Close() // EOF ends the console, the server keeps running
	if code, _ := api(t, "GET", "http://"+addr+"/api/leaderboard", "", ""); code != 200 {
		t.Errorf("server after EOF: %d", code)
	}
	cancel()
	<-done
}

func TestAdminIsOffWithoutThePasswordEnv(t *testing.T) {
	t.Setenv(adminPasswordEnv, "")
	ctx, cancel := context.WithCancel(context.Background())
	addr, done := start(t, ctx, t.TempDir())
	if code, _ := api(t, "POST", "http://"+addr+"/api/admin/login", "", `{"password":""}`); code != http.StatusNotFound {
		t.Errorf("login with admin off: %d", code)
	}
	cancel()
	<-done
}

func TestAdminLoginWorksWithThePasswordEnv(t *testing.T) {
	t.Setenv(adminPasswordEnv, "hunter2")
	ctx, cancel := context.WithCancel(context.Background())
	addr, done := start(t, ctx, t.TempDir())
	if code, _ := api(t, "POST", "http://"+addr+"/api/admin/login", "", `{"password":"hunter2"}`); code != http.StatusNoContent {
		t.Errorf("login: %d", code)
	}
	if code, _ := api(t, "POST", "http://"+addr+"/api/admin/login", "", `{"password":"wrong"}`); code != http.StatusUnauthorized {
		t.Errorf("wrong password: %d", code)
	}
	cancel()
	<-done
}

func TestAnnounceAdminNeverPrintsThePassword(t *testing.T) {
	var on, off strings.Builder
	announceAdmin("hunter2", &on)
	announceAdmin("", &off)
	if strings.Contains(on.String(), "hunter2") || !strings.Contains(on.String(), "/admin") || !strings.Contains(off.String(), adminPasswordEnv) {
		t.Errorf("on %q off %q", on.String(), off.String())
	}
}

func TestStartMapFallsBackToTheBuiltInRoom(t *testing.T) {
	if got, err := (config{}).startMap(&apptest.MemStore{}); err != nil || got != defaultMap {
		t.Errorf("empty store: %q %v", got, err)
	}
}

func TestStartMapUsesTheStoredMap(t *testing.T) {
	st := &apptest.MemStore{Map: "###\n#.#\n###\n"}
	if got, err := (config{}).startMap(st); err != nil || got != st.Map {
		t.Errorf("stored: %q %v", got, err)
	}
}

func TestStartMapFlagBeatsTheStoredMap(t *testing.T) {
	file := filepath.Join(t.TempDir(), "m.txt")
	if err := os.WriteFile(file, []byte("flag map"), 0o644); err != nil {
		t.Fatal(err)
	}
	st := &apptest.MemStore{Map: "stored"}
	if got, err := (config{mapFile: file}).startMap(st); err != nil || got != "flag map" {
		t.Errorf("flag: %q %v", got, err)
	}
	if _, err := (config{mapFile: file + ".nope"}).startMap(st); err == nil {
		t.Error("a missing -map file must fail")
	}
}

func TestStartMapReportsAStoreError(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "map.txt"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := (config{}).startMap(store.New(dir)); err == nil {
		t.Error("want an error")
	}
}

func TestAStoredMapIsPlayedAtStartup(t *testing.T) {
	dir := t.TempDir()
	if err := store.New(dir).SaveMap("###\n#.#\n###\n"); err != nil { // one floor cell: too small for -min-bots 2
		t.Fatal(err)
	}
	args := []string{"-addr", "127.0.0.1:0", "-data", dir, "-min-bots", "2"}
	if err := run(cancelled(), args, strings.NewReader(""), io.Discard, io.Discard); err == nil {
		t.Error("the stored map was not used")
	}
}

func TestAStoredMapSizesTheRunningArena(t *testing.T) {
	t.Setenv(adminPasswordEnv, "hunter2")
	dir := t.TempDir()
	stored := "###########\n#.........#\n#.........#\n#.........#\n###########\n" // 11x5, the built-in room is another size
	if err := store.New(dir).SaveMap(stored); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	addr, done := start(t, ctx, dir)
	st := adminStatus(t, addr, "hunter2")
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if st.MapWidth != 11 || st.MapHeight != 5 {
		t.Errorf("size %dx%d, want 11x5", st.MapWidth, st.MapHeight)
	}
}

// adminStatus logs in as the admin and reads the status.
func adminStatus(t *testing.T, addr, password string) protocol.AdminStatus {
	t.Helper()
	jar, _ := cookiejar.New(nil)
	c := &http.Client{Jar: jar}
	resp, err := c.Post("http://"+addr+"/api/admin/login", "application/json", strings.NewReader(`{"password":"`+password+`"}`))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	resp, err = c.Get("http://" + addr + "/api/admin/status")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var st protocol.AdminStatus
	if err := json.NewDecoder(resp.Body).Decode(&st); err != nil {
		t.Fatal(err)
	}
	return st
}

func TestSettingsAreReadFromTheEnvFile(t *testing.T) {
	t.Setenv(baseURLEnv, "")
	os.Unsetenv(baseURLEnv) //nolint:errcheck // t.Setenv restores it
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte(baseURLEnv+"=http://file.test\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	addr, done := start(t, ctx, dir)
	if code, body := api(t, "GET", "http://"+addr+"/api/config", "", ""); code != 200 || !strings.Contains(string(body), `"baseUrl":"http://file.test"`) {
		t.Errorf("config: %d %s", code, body)
	}
	cancel()
	<-done
}

func TestBadEnvFileFails(t *testing.T) {
	err := run(cancelled(), []string{"-addr", "127.0.0.1:0", "-data", t.TempDir(), "-env", t.TempDir()}, strings.NewReader(""), io.Discard, io.Discard)
	if err == nil {
		t.Error("want an error")
	}
}

func TestCleanBaseURLTrimsSlashesAndBlanks(t *testing.T) {
	for in, want := range map[string]string{
		"": "", "  ": "", "https://a.example/": "https://a.example", " http://a.example:8080// ": "http://a.example:8080",
	} {
		if got := cleanBaseURL(in); got != want {
			t.Errorf("cleanBaseURL(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestConfigAndCheckAreServed(t *testing.T) {
	t.Setenv(baseURLEnv, "http://example.test/")
	ctx, cancel := context.WithCancel(context.Background())
	addr, done := start(t, ctx, t.TempDir())
	if code, body := api(t, "GET", "http://"+addr+"/api/config", "", ""); code != 200 || !strings.Contains(string(body), `"baseUrl":"http://example.test"`) {
		t.Errorf("config: %d %s", code, body)
	}
	code, body := api(t, "POST", "http://"+addr+"/api/check", "", "function loop() { scan(E); }")
	if code != 200 || !strings.Contains(string(body), `"ok":true`) {
		t.Errorf("check: %d %s", code, body)
	}
	cancel()
	<-done
}
