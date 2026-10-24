package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/lezhnev/arena/engine/app"
	"github.com/lezhnev/arena/engine/domain/sim"
	"github.com/lezhnev/arena/engine/lang"
)

// randomName is the label of bots that play a random action script.
const randomName = "random"

// config is the parsed command line.
type config struct {
	mapFile       string // empty: a random room per match
	w, h          int
	lineup        []string // bot i plays lineup[i]: a house bot name or "random"
	matches, beat int
	seed          uint64
	rules         sim.Rules
	points        app.Points        // leaderboard scoring for the score column
	scripts       map[string]string // bot id -> script source, overriding the lineup
	programs      map[string]string // bot id -> house bot name, overriding the lineup
	sweeps        []sweepAxis
	dodgers       []string // programs the sweep's dodge column covers
	explicit      bool     // -bots was a program list, so it must match the map's bots
}

func parseConfig(args []string, stderr io.Writer) (config, error) {
	c := config{dodgers: []string{"lighthouse", "tracker"}, rules: sim.DefaultRules(), points: app.DefaultPoints(), scripts: map[string]string{}, programs: map[string]string{}}
	var size, bots string
	fl := flag.NewFlagSet("sim", flag.ContinueOnError)
	fl.SetOutput(stderr)
	fl.StringVar(&c.mapFile, "map", "", "map file (default: a random room per match); a map without bots gets them placed at random")
	fl.StringVar(&size, "size", "7x7", "random room size WxH (5..9)")
	fl.StringVar(&bots, "bots", "3", "a count of random-script bots, or a list like rookie,lighthouse,tracker,random (bot i plays item i)")
	fl.IntVar(&c.matches, "matches", 20, "number of matches")
	fl.IntVar(&c.beat, "beats", 200, "beats per match")
	fl.Uint64Var(&c.seed, "seed", 1, "seed of the first match; match i uses seed+i")
	fl.IntVar(&c.points.Coin, "coin-points", c.points.Coin, "leaderboard points per coin, for the score column")
	fl.Func("dodgers", "programs the sweep dodge column covers (default lighthouse,tracker: the bots that react to bullet readings)", c.setDodgers)
	fl.Func("rule", "rule override key=value, e.g. reload=4 (repeatable)", c.setRule)
	fl.Func("script", "scripted bot ID=actions, e.g. 'A=fire E, move S' (repeatable); other bots keep their lineup", c.setScript)
	fl.Func("program", "house bot ID=name, e.g. A=tracker (repeatable)", c.setProgram)
	fl.Func("sweep", "rule sweep key=lo..hi or key=a,b,c (repeatable): one table row per combination", c.addSweep)
	if err := fl.Parse(args); err != nil {
		return c, err
	}
	if err := c.setSize(size); err != nil {
		return c, err
	}
	if err := c.setLineup(bots); err != nil {
		return c, err
	}
	if c.matches < 1 {
		return c, fmt.Errorf("-matches must be at least 1")
	}
	if c.points.Coin < 0 {
		return c, fmt.Errorf("-coin-points must not be negative")
	}
	return c, nil
}

func (c *config) setRule(kv string) error {
	key, val, ok := strings.Cut(kv, "=")
	n, err := strconv.Atoi(val)
	if !ok || err != nil {
		return fmt.Errorf("rule %q: want key=number", kv)
	}
	return c.rules.Set(key, n)
}

func (c *config) setDodgers(list string) error {
	c.dodgers = nil
	for _, name := range strings.Split(list, ",") {
		if _, ok := lang.HouseBot(name); !ok && name != randomName {
			return fmt.Errorf("-dodgers: no house bot %q", name)
		}
		c.dodgers = append(c.dodgers, name)
	}
	return nil
}

func (c *config) setScript(kv string) error {
	id, src, ok := strings.Cut(kv, "=")
	if !ok || id == "" {
		return fmt.Errorf("script %q: want ID=actions", kv)
	}
	if _, err := app.NewBrain(app.BrainSpec{Kind: app.KindScript, Src: src}, 0, id); err != nil {
		return fmt.Errorf("script %s: %w", id, err)
	}
	c.scripts[id] = src
	return nil
}

func (c *config) setProgram(kv string) error {
	id, name, ok := strings.Cut(kv, "=")
	if !ok || id == "" {
		return fmt.Errorf("program %q: want ID=name", kv)
	}
	if _, ok := lang.HouseBot(name); !ok {
		return fmt.Errorf("program %s: no house bot %q (have %v)", id, name, lang.HouseBotNames())
	}
	c.programs[id] = name
	return nil
}

func (c *config) setSize(size string) error {
	ws, hs, ok := strings.Cut(size, "x")
	w, errW := strconv.Atoi(ws)
	h, errH := strconv.Atoi(hs)
	if !ok || errW != nil || errH != nil {
		return fmt.Errorf("-size %q: want WxH, e.g. 7x7", size)
	}
	c.w, c.h = w, h
	return nil
}

// setLineup reads -bots: a count of random bots or a comma list of programs.
func (c *config) setLineup(bots string) error {
	if n, err := strconv.Atoi(bots); err == nil {
		return c.randomLineup(n)
	}
	c.explicit = true
	for _, item := range strings.Split(bots, ",") {
		name, count, err := splitCount(item)
		if err != nil {
			return err
		}
		if _, ok := lang.HouseBot(name); !ok && name != randomName {
			return fmt.Errorf("-bots: no house bot %q (have %v or %q)", name, lang.HouseBotNames(), randomName)
		}
		for range count {
			c.lineup = append(c.lineup, name)
		}
	}
	return nil
}

func (c *config) randomLineup(n int) error {
	if n < 1 {
		return fmt.Errorf("-bots %d: want at least 1", n)
	}
	c.lineup = make([]string, n)
	for i := range c.lineup {
		c.lineup[i] = randomName
	}
	return nil
}

// splitCount reads "name" or "name:count".
func splitCount(item string) (string, int, error) {
	name, cnt, ok := strings.Cut(item, ":")
	if !ok {
		return name, 1, nil
	}
	n, err := strconv.Atoi(cnt)
	if err != nil || n < 1 {
		return "", 0, fmt.Errorf("-bots %q: want name:count with count >= 1", item)
	}
	return name, n, nil
}

// match is one prepared match: its record and what each bot plays.
type match struct {
	rec   app.Record
	names map[string]string // bot id -> program name
}

// record builds match i: its map, the lineup, then the -program and -script overrides.
func (c config) record(i int) (match, error) {
	seed := c.seed + uint64(i)
	mapText, err := c.mapText(seed)
	if err != nil {
		return match{}, err
	}
	rec, err := app.RandomRecordOn(seed, c.rules, mapText, c.beat)
	if err != nil {
		return match{}, err
	}
	m := match{rec: rec, names: map[string]string{}}
	if err := m.assign(c); err != nil {
		return m, err
	}
	return m, nil
}

// assign gives every bot its program: lineup order, then overrides.
func (m *match) assign(c config) error {
	if c.explicit && len(m.rec.Bots) != len(c.lineup) {
		return fmt.Errorf("-bots lists %d programs but the map has %d bots", len(c.lineup), len(m.rec.Bots))
	}
	for i := range m.rec.Bots {
		m.setProgram(i, c.lineup[min(i, len(c.lineup)-1)])
	}
	return m.override(c)
}

// override applies -program, then -script.
func (m *match) override(c config) error {
	for id, name := range c.programs {
		i, err := m.index(id)
		if err != nil {
			return err
		}
		m.setProgram(i, name)
	}
	for id, src := range c.scripts {
		i, err := m.index(id)
		if err != nil {
			return err
		}
		m.rec.Bots[i].Brain = app.BrainSpec{Kind: app.KindScript, Src: src}
		m.names[id] = "script"
	}
	return nil
}

func (m *match) index(id string) (int, error) {
	for i := range m.rec.Bots {
		if m.rec.Bots[i].ID == id {
			return i, nil
		}
	}
	return 0, fmt.Errorf("override for %q: no such bot on the map", id)
}

func (m *match) setProgram(i int, name string) {
	b := &m.rec.Bots[i]
	m.names[b.ID] = name
	if name == randomName {
		return
	}
	src, _ := lang.HouseBot(name) // names were checked
	b.Brain = app.BrainSpec{Kind: app.KindProgram, Src: src}
}

// mapText is the room of match seed: the map file (bots placed if it has none)
// or a random room.
func (c config) mapText(seed uint64) (string, error) {
	if c.mapFile == "" {
		if len(c.lineup) > 4 {
			return "", fmt.Errorf("random rooms hold at most 4 bots; use -map for %d", len(c.lineup))
		}
		return app.RandomMap(seed, c.w, c.h, len(c.lineup)), nil
	}
	b, err := os.ReadFile(c.mapFile)
	if err != nil {
		return "", err
	}
	return app.PlaceBots(string(b), seed, len(c.lineup))
}
