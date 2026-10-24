// Package sim is the pure game simulation: World state and the Resolve
// function that turns one beat of actions into a new World plus events.
package sim

import (
	"fmt"
	"strings"

	"github.com/lezhnev/arena/engine/domain/brain"
	"github.com/lezhnev/arena/engine/domain/geo"
)

// Bot is one bot's state in the World.
type Bot struct {
	ID   string
	Pos  geo.Pos
	HP   int
	Last brain.Outcome  // result of the previous beat, shown to Decide
	Read brain.Readings // the last scan's readings, kept until the next scan

	Cooldown int // beats until the regular shot is ready again; 0 = loaded
	Bonus    int // stored bonus shots, usable while reloading
	Shield   int // beats of immunity to bullets left (after a respawn)
	Life     int // how many times the bot has respawned

	Heard brain.Found // the cell of the nearest sound on the latest beat anything was heard; reset on respawn
}

// Fallen is a dead bot waiting to respawn. It is off the grid: nothing can
// see, hit or bump it. Wait is the beats left before it comes back (0 = this
// beat, as soon as a free cell exists).
type Fallen struct {
	Bot  Bot
	Wait int
}

// Loaded says whether a fire action would shoot: the gun is reloaded or a
// bonus shot is stored.
func (b Bot) Loaded() bool { return b.Cooldown == 0 || b.Bonus > 0 }

// AddBonus stores one more bonus shot, up to max.
func (b *Bot) AddBonus(max int) { b.Bonus = min(b.Bonus+1, max) }

// Bullet is a flying shot, spawned in the shooter's cell. Owner may be empty
// for bullets placed by a scenario.
type Bullet struct {
	Pos   geo.Pos
	Dir   geo.Dir
	Owner string // id of the shooter
}

// Glyph is the character that draws the bot in ASCII frames. Only
// single-character IDs have one; any other ID renders as '?'.
func (b Bot) Glyph() byte {
	if len(b.ID) != 1 {
		return '?'
	}
	return b.ID[0]
}

// World is the complete game state between beats. Bots are sorted by ID.
//
// World is copied by value between beats and Resolve must stay pure, so any
// future RNG state (the seeded PCG) must be stored as plain values (e.g. the
// PCG seed words plus a draw counter), never as a pointer or *rand.Rand that
// copies would share.
type World struct {
	Grid      geo.Grid
	Coins     []geo.Pos
	Bullets   []Bullet
	Bots      []Bot
	Fallen    []Fallen    // dead bots, sorted by ID
	Rebooting []Rebooting // bots off the map for a reboot, sorted by ID
	Seed      uint64
	RNG       uint64 // state of the seeded PCG (see rng)
	Beat      int    // beats resolved so far
	MaxHP     int    // Rules.HP, a full health bar (for views)
	Reload    int    // Rules.Reload, a full reload (for views)
}

func (w World) rng() rng { return rng{w.RNG} }

// NewWorld builds the initial world from a parsed map.
func NewWorld(m geo.Map, r Rules, seed uint64) World {
	bots := make([]Bot, len(m.Bots))
	for i, b := range m.Bots {
		bots[i] = Bot{ID: string(b.Letter), Pos: b.Pos, HP: r.HP, Last: brain.Outcome{OK: true}}
	}
	return World{Grid: m.Grid, Coins: m.Coins, Bots: bots, Seed: seed, RNG: newRNG(seed).state, MaxHP: r.HP, Reload: r.Reload}
}

// Render draws the world as ASCII (same alphabet as map files).
func (w World) Render() string {
	marks := make(map[geo.Pos]byte, len(w.Coins)+len(w.Bots)+len(w.Bullets))
	for _, b := range w.Bullets {
		marks[b.Pos] = '*'
	}
	for _, c := range w.Coins {
		marks[c] = '$'
	}
	for _, b := range w.Bots {
		marks[b.Pos] = b.Glyph()
	}
	return w.Grid.Render(marks)
}

func (w World) bot(id string) (Bot, bool) {
	for _, b := range w.Bots {
		if b.ID == id {
			return b, true
		}
	}
	return Bot{}, false
}

// Perception is what the bot with the given id sees before the given beat.
func (w World) Perception(id string, beat int) brain.Perception {
	b, _ := w.bot(id)
	return brain.Perception{
		Beat: beat, Pos: b.Pos, HP: b.HP, Last: b.Last, Read: b.Read,
		Loaded: b.Loaded(), Bonus: b.Bonus,
		Life: b.Life, Heard: b.Heard, Width: w.Grid.Width(), Height: w.Grid.Height(), Open: w.open(b.Pos), Grid: w.Grid,
	}
}

func (w World) open(p geo.Pos) (open [4]bool) {
	for d := geo.N; d <= geo.W; d++ {
		open[d] = !w.Grid.Blocked(p.Step(d))
	}
	return open
}

// Query reads a state value addressed as "<bot>.<field>" (field: x, y, hp,
// bonus, cooldown, shield, loaded, hit, alive, rebooting (beats left, 0 = not); loaded and alive are 1 or 0, hit
// is the direction of the last beat's hit or -1; heard_* is the place: _set is 1/0 and _x / _y the cell, -1 for none). A dead bot answers with the
// state it died in (hp 0, bonus 0), x and y being where it fell.
func (w World) Query(spec string) (int, error) {
	id, field, ok := strings.Cut(spec, ".")
	if !ok {
		return 0, fmt.Errorf("want <bot>.<field>, got %q", spec)
	}
	b, isAlive := w.bot(id)
	if !isAlive {
		var found bool
		if b, found = w.offMap(id); !found {
			return 0, fmt.Errorf("no bot %q", id)
		}
	}
	switch field {
	case "alive":
		return boolInt(isAlive), nil
	case "rebooting":
		r, _ := w.rebooting(id)
		return r.Left, nil
	}
	if v, ok := placeField(b, field); ok {
		return v, nil
	}
	return botField(b, field)
}

// placeField reads <place>_set (1/0) and <place>_x / <place>_y (-1 for none)
// of the heard place.
func placeField(b Bot, field string) (int, bool) {
	name, attr, _ := strings.Cut(field, "_")
	if name != "heard" {
		return 0, false
	}
	f := b.Heard
	switch attr {
	case "set":
		return boolInt(f.Set), true
	case "x":
		return foundCoord(f, f.Pos.X), true
	case "y":
		return foundCoord(f, f.Pos.Y), true
	}
	return 0, false
}

// foundCoord is one coordinate of a place, -1 when the place is none.
func foundCoord(f brain.Found, v int) int {
	if !f.Set {
		return -1
	}
	return v
}

// offMap finds a bot that is not on the grid: dead, or rebooting.
func (w World) offMap(id string) (Bot, bool) {
	if f, ok := w.fallen(id); ok {
		return f.Bot, true
	}
	r, ok := w.rebooting(id)
	return r.Bot, ok
}

func (w World) fallen(id string) (Fallen, bool) {
	for _, f := range w.Fallen {
		if f.Bot.ID == id {
			return f, true
		}
	}
	return Fallen{}, false
}

func botField(b Bot, field string) (int, error) {
	switch field {
	case "x":
		return b.Pos.X, nil
	case "y":
		return b.Pos.Y, nil
	case "loaded":
		return boolInt(b.Loaded()), nil
	case "hit":
		return hitField(b), nil
	}
	if f, ok := stateFields[field]; ok {
		return *f(&b), nil
	}
	return 0, fmt.Errorf("unknown field %q (want x, y, hp, bonus, cooldown, shield, loaded, hit, heard_set, heard_x, heard_y, alive or rebooting)", field)
}

// hitField is the direction the last beat's hit came from (N=0 E=1 S=2 W=3),
// or -1 when the bot was not hit.
func hitField(b Bot) int {
	if !b.Last.Hit {
		return -1
	}
	return int(b.Last.HitDir)
}

func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

// stateFields are the bot values a scenario may set up and read.
var stateFields = map[string]func(*Bot) *int{
	"hp":       func(b *Bot) *int { return &b.HP },
	"bonus":    func(b *Bot) *int { return &b.Bonus },
	"cooldown": func(b *Bot) *int { return &b.Cooldown },
	"shield":   func(b *Bot) *int { return &b.Shield },
}

// SetBot overrides one state field (hp, bonus, cooldown, shield) of a bot; it is how
// scenarios start a bot in a non-default state. It changes w in place.
func (w World) SetBot(id, field string, v int) error {
	f, ok := stateFields[field]
	if !ok {
		return fmt.Errorf("cannot set %q (want hp, bonus, cooldown or shield)", field)
	}
	for i := range w.Bots {
		if w.Bots[i].ID == id {
			*f(&w.Bots[i]) = v
			return nil
		}
	}
	return fmt.Errorf("no bot %q", id)
}
