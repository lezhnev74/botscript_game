package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"net/netip"
	"path/filepath"
	"time"

	"github.com/lezhnev/arena/engine/adapters/httpapi"
	"github.com/lezhnev/arena/engine/app"
)

// config is the parsed command line.
type config struct {
	addr    string
	mapFile string // empty: defaultMap
	seed    uint64
	beat    time.Duration
	minBots int
	maxPlay int           // cap on human players; 0 = only the floor limits them
	maxStay time.Duration // longest stay of a human bot in the arena; 0 = no limit
	reboot  time.Duration // time a saved program keeps its bot off the map; 0 = off (plain hot-swap)
	coinPts int           // leaderboard points per coin
	dataDir string
	webDir  string
	origins []string       // extra browser origins allowed on /ws (dev servers)
	envFile string         // dotenv file; the process environment wins over it
	proxies []netip.Prefix // reverse proxies whose X-Forwarded-For/-Proto are believed
}

func parseConfig(args []string, stderr io.Writer) (config, error) {
	c := config{}
	var beatMS int
	fl := flag.NewFlagSet("arena", flag.ContinueOnError)
	fl.SetOutput(stderr)
	fl.StringVar(&c.addr, "addr", ":8080", "listen address")
	fl.StringVar(&c.mapFile, "map", "", "arena map file, an ASCII room without bots (default: the map the admin applied last, else the built-in room)")
	fl.Uint64Var(&c.seed, "seed", 1, "seed of the simulation")
	fl.IntVar(&beatMS, "beat", 1000, "beat length in milliseconds")
	fl.IntVar(&c.minBots, "min-bots", 5, "house bots fill the arena up to this many bots (tuned: TUNING.md)")
	fl.IntVar(&c.maxPlay, "max-players", 50, "most human players that may join (0 = as many as the room has floor cells)")
	fl.DurationVar(&c.maxStay, "max-stay", 24*time.Hour, "longest stay of a human bot in the arena before it leaves by itself, e.g. 24h (0 = no limit)")
	fl.DurationVar(&c.reboot, "reboot", 10*time.Second, "time a bot is off the map after its player saves a new program, e.g. 10s (0 = off: the program is hot-swapped at the next beat)")
	fl.IntVar(&c.coinPts, "coin-points", app.DefaultPoints().Coin, "leaderboard points per coin (VISION section 9; not part of the event log)")
	fl.StringVar(&c.dataDir, "data", "data", "directory of the leaderboard and bots files")
	fl.StringVar(&c.webDir, "web", filepath.Join("..", "web", "dist"), "directory of the built web app")
	fl.StringVar(&c.envFile, "env", ".env", "dotenv file with ARENA_* settings (optional; environment variables win over it)")
	fl.Func("trust-proxy", "address or CIDR of a reverse proxy (e.g. nginx) whose X-Forwarded-For names the client (repeatable)", func(s string) error {
		p, err := parsePrefix(s)
		c.proxies = append(c.proxies, p)
		return err
	})
	fl.Func("origin", "extra allowed browser origin for /ws, e.g. localhost:5173 (repeatable)", func(s string) error {
		c.origins = append(c.origins, s)
		return nil
	})
	if err := fl.Parse(args); err != nil {
		return c, err
	}
	c.beat = time.Duration(beatMS) * time.Millisecond
	return c, c.validate(fl.NArg())
}

func (c config) validate(extraArgs int) error {
	switch {
	case extraArgs > 0:
		return errors.New("unexpected arguments")
	case c.beat <= 0:
		return fmt.Errorf("-beat must be positive")
	case c.coinPts < 0:
		return fmt.Errorf("-coin-points must not be negative")
	case c.maxPlay < 0:
		return fmt.Errorf("-max-players must not be negative")
	case c.maxStay < 0:
		return fmt.Errorf("-max-stay must not be negative")
	case c.minBots < 0:
		return fmt.Errorf("-min-bots must not be negative")
	}
	return c.checkReboot()
}

func (c config) checkReboot() error {
	if c.reboot < 0 {
		return fmt.Errorf("-reboot must not be negative")
	}
	return nil
}

// stayBeats is -max-stay in beats, rounded up (0 = no limit).
func (c config) stayBeats() int {
	return httpapi.MinutesToBeats(c.maxStay.Minutes(), c.beat)
}

// rebootBeats is -reboot in beats, rounded up (0 = off).
func (c config) rebootBeats() int {
	return httpapi.MinutesToBeats(c.reboot.Minutes(), c.beat)
}

// parsePrefix reads a CIDR, or a single address as a one-address prefix.
func parsePrefix(s string) (netip.Prefix, error) {
	if addr, err := netip.ParseAddr(s); err == nil {
		return netip.PrefixFrom(addr, addr.BitLen()), nil
	}
	return netip.ParsePrefix(s)
}

// defaultMap is the built-in arena (see app.DefaultRoom).
const defaultMap = app.DefaultRoom
