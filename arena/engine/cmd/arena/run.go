package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lezhnev/arena/engine/adapters/clock"
	"github.com/lezhnev/arena/engine/adapters/httpapi"
	"github.com/lezhnev/arena/engine/adapters/ids"
	"github.com/lezhnev/arena/engine/adapters/store"
	"github.com/lezhnev/arena/engine/adapters/ws"
	"github.com/lezhnev/arena/engine/app"
	"github.com/lezhnev/arena/engine/domain/sim"
)

// adminPasswordEnv names the environment variable that turns the web admin on.
// Its value is never printed or logged.
const adminPasswordEnv = "ARENA_ADMIN_PASSWORD"

// baseURLEnv names the environment variable with the public address of the
// server (e.g. https://arena.example.com). It is optional: GET /api/config
// reports it and the rulebook for AI uses it in the check URLs.
const baseURLEnv = "ARENA_BASE_URL"

const shutdownGrace = 5 * time.Second

// run wires the arena and serves until ctx ends, then shuts down in order:
// stop accepting requests, disconnect WebSockets, stop the beats, close the arena.
func run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	c, err := parseConfig(args, stderr)
	if err != nil {
		return err
	}
	env, err := loadEnv(c.envFile, os.LookupEnv)
	if err != nil {
		return err
	}
	st := store.New(c.dataDir)
	mapText, err := c.startMap(st)
	if err != nil {
		return err
	}
	hub := ws.New(ws.Options{OriginPatterns: c.origins, Beat: c.beat}, app.Update{Remaining: -1})
	arena, err := app.NewArena(
		app.Config{Rules: sim.DefaultRules(), Map: mapText, Seed: c.seed, MinBots: c.minBots, MaxPlayers: c.maxPlay, MaxStay: c.stayBeats(), RebootBeats: c.rebootBeats(), Points: c.points()},
		app.Deps{
			Store: st, Broadcaster: hub, IDs: ids.New(),
			OnError: func(err error) { fmt.Fprintln(stderr, "arena: beat:", err) },
		},
	)
	if err != nil {
		return err
	}
	defer arena.Close()
	hub.Reset(app.Update{World: arena.World(), Board: arena.Leaderboard(), Remaining: -1})
	ln, err := net.Listen("tcp", c.addr)
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "listening on http://%s\n", ln.Addr())
	announceWeb(c.webDir, stdout, stderr)
	if stdin != nil {
		go console(stdin, stdout, arena, c.beat)
	}
	pass := env(adminPasswordEnv)
	announceAdmin(pass, stdout)
	h := httpapi.New(arena, httpapi.Options{
		WS: hub, WebDir: c.webDir, Beat: c.beat, Admin: httpapi.AdminOptions{Service: arena, Password: pass, Beat: c.beat},
		Check: app.Check, BaseURL: cleanBaseURL(env(baseURLEnv)), TrustedProxies: c.proxies,
	})
	return server{handler: h, arena: arena, hub: hub, beat: c.beat, grace: shutdownGrace}.serve(ctx, ln)
}

// startMap picks the room of the first game: the -map file when given, else
// the map the admin applied last (kept in the store), else the built-in room.
func (c config) startMap(st app.Store) (string, error) {
	if c.mapFile != "" {
		b, err := os.ReadFile(c.mapFile)
		return string(b), err
	}
	saved, err := st.LoadMap()
	if err != nil || saved == "" {
		return defaultMap, err
	}
	return saved, nil
}

// server is everything serve runs: the HTTP handler, the arena that plays
// the beats and the hub that is closed at shutdown.
type server struct {
	handler http.Handler
	arena   *app.Arena
	hub     *ws.Hub
	beat    time.Duration
	grace   time.Duration // how long open requests may take to finish at shutdown
}

// serve plays beats and answers HTTP until ctx ends or the listener fails. It
// always waits for the beat loop to stop and then saves what is still pending,
// so a join or edit accepted between two beats survives the shutdown.
func (s server) serve(ctx context.Context, ln net.Listener) error {
	tick := clock.New(s.beat)
	defer tick.Stop()
	runCtx, stopRun := context.WithCancel(ctx)
	defer stopRun()
	runDone := make(chan struct{})
	go func() { _ = s.arena.Run(runCtx, tick); close(runDone) }()
	srv := &http.Server{
		Handler: s.handler, ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout: 30 * time.Second, IdleTimeout: 2 * time.Minute,
	}
	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.Serve(ln) }()
	var err error
	select {
	case err = <-serveErr:
	case <-ctx.Done():
		err = s.shutdown(srv)
	}
	s.hub.Close()
	stopRun()
	<-runDone
	return errors.Join(err, s.arena.Flush())
}

// shutdown stops the HTTP server, giving open requests the grace period and
// then cutting them off.
func (s server) shutdown(srv *http.Server) error {
	sctx, cancel := context.WithTimeout(context.Background(), s.grace)
	defer cancel()
	err := srv.Shutdown(sctx)
	if errors.Is(err, context.DeadlineExceeded) {
		return srv.Close()
	}
	return err
}

// announceWeb says which web app directory is served, or warns that there is none.
func announceWeb(dir string, stdout, stderr io.Writer) {
	if _, err := os.Stat(filepath.Join(dir, "index.html")); err != nil {
		fmt.Fprintf(stderr, "arena: warning: no index.html in web dir %q (build the web app or pass -web); serving a placeholder page\n", dir)
		return
	}
	fmt.Fprintf(stdout, "serving web app from %s\n", dir)
}

// points is the leaderboard scoring table: the defaults with the -coin-points override.
func (c config) points() *app.Points {
	p := app.DefaultPoints()
	p.Coin = c.coinPts
	return &p
}

// announceAdmin says whether the web admin is on, without ever echoing the password.
func announceAdmin(password string, stdout io.Writer) {
	if password == "" {
		fmt.Fprintf(stdout, "web admin is off: set %s to enable /admin\n", adminPasswordEnv)
		return
	}
	fmt.Fprintln(stdout, "web admin is on at /admin")
}

// cleanBaseURL trims blanks and trailing slashes off the configured base URL.
func cleanBaseURL(s string) string { return strings.TrimRight(strings.TrimSpace(s), "/") }
