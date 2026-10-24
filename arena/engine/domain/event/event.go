// Package event holds the typed facts a beat produces. Every event has a
// stable textual form ("Moved B S") used by scenario expectations and a
// canonical JSON form used by goldens and hashing.
package event

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/lezhnev/arena/engine/domain/geo"
)

// Event is something that happened during a beat.
type Event interface {
	Name() string   // type name, e.g. "Moved"
	Args() []string // stable textual arguments, e.g. ["B", "S"]
}

// Logged is an event tagged with the beat (1-based) it happened in.
type Logged struct {
	Beat  int
	Event Event
}

// Moved: a bot stepped one cell in Dir.
type Moved struct {
	Bot string
	Dir geo.Dir
}

// Bumped: a bot tried to step in Dir and ran into another bot.
type Bumped struct {
	Bot string
	Dir geo.Dir
}

// Fired: a bot shot in Dir.
type Fired struct {
	Bot string
	Dir geo.Dir
}

// Clicked: a bot tried to fire in Dir with nothing loaded; the beat is wasted.
type Clicked struct {
	Bot string
	Dir geo.Dir
}

// Hit: a bullet that came from direction From hit a bot. Shooter (the
// bullet's owner) is in the JSON payload only, for kill credit and the TV.
type Hit struct {
	Bot     string
	From    geo.Dir
	Shooter string
}

// Killed: Victim's HP reached 0. Killer is the shooter of the lethal bullet
// (equal to Victim for a self-kill, empty for an ownerless scenario bullet).
type Killed struct {
	Killer, Victim string
}

// Respawned: a dead bot is back on the map at X,Y, shielded.
type Respawned struct {
	Bot  string
	X, Y int
}

// CoinTaken: a bot stepped onto the coin at X,Y and took it.
type CoinTaken struct {
	Bot  string
	X, Y int
}

// CoinSpawned: a new coin appeared at X,Y.
type CoinSpawned struct {
	X, Y int
}

// Joined: a player's bot entered the arena at X,Y between beats (live arena).
type Joined struct {
	Bot  string
	X, Y int
}

// Left: a bot left the arena between beats, alive or waiting to respawn.
type Left struct {
	Bot string
}

// Kicked: the admin removed a bot; follows its Left. The leaderboard drops the
// bot's line for good.
type Kicked struct {
	Bot string
}

// Sighting kinds a scan can report.
const (
	KindEnemy  = "enemy"
	KindCoin   = "coin"
	KindBullet = "bullet"
)

// Cell is a map cell in a scan record.
type Cell struct {
	X int `json:"x"`
	Y int `json:"y"`
}

// Sighting is one object inside a scan cone and the level it was given.
type Sighting struct {
	Kind  string `json:"kind"`
	X     int    `json:"x"`
	Y     int    `json:"y"`
	Level int    `json:"level"`
}

// Scanned is the scan record: what the bot reported (the best level per
// kind) plus the cone cells and every object seen in it. The TV draws the
// x-ray from this record and never recomputes it.
type Scanned struct {
	Bot                 string
	Dir                 geo.Dir
	Enemy, Coin, Bullet int
	Cells               []Cell
	Seen                []Sighting
}

func (Moved) Name() string   { return "Moved" }
func (Bumped) Name() string  { return "Bumped" }
func (Fired) Name() string   { return "Fired" }
func (Clicked) Name() string { return "Clicked" }
func (Hit) Name() string     { return "Hit" }
func (Scanned) Name() string { return "Scanned" }

func (e Moved) Args() []string   { return []string{e.Bot, e.Dir.String()} }
func (e Bumped) Args() []string  { return []string{e.Bot, e.Dir.String()} }
func (e Fired) Args() []string   { return []string{e.Bot, e.Dir.String()} }
func (e Clicked) Args() []string { return []string{e.Bot, e.Dir.String()} }
func (e Hit) Args() []string     { return []string{e.Bot, e.From.String()} }

func (Killed) Name() string      { return "Killed" }
func (Respawned) Name() string   { return "Respawned" }
func (CoinTaken) Name() string   { return "CoinTaken" }
func (CoinSpawned) Name() string { return "CoinSpawned" }

func (Joined) Name() string { return "Joined" }
func (Left) Name() string   { return "Left" }
func (Kicked) Name() string { return "Kicked" }

func (e Joined) Args() []string { return []string{e.Bot, strconv.Itoa(e.X), strconv.Itoa(e.Y)} }
func (e Left) Args() []string   { return []string{e.Bot} }
func (e Kicked) Args() []string { return []string{e.Bot} }

func (e Killed) Args() []string      { return []string{e.Killer, e.Victim} }
func (e Respawned) Args() []string   { return []string{e.Bot, strconv.Itoa(e.X), strconv.Itoa(e.Y)} }
func (e CoinTaken) Args() []string   { return []string{e.Bot, strconv.Itoa(e.X), strconv.Itoa(e.Y)} }
func (e CoinSpawned) Args() []string { return []string{strconv.Itoa(e.X), strconv.Itoa(e.Y)} }

func (e Scanned) Args() []string {
	return []string{e.Bot, e.Dir.String(), strconv.Itoa(e.Enemy), strconv.Itoa(e.Coin), strconv.Itoa(e.Bullet)}
}

// Detailed is implemented by events with a payload beyond their text args;
// the payload goes into the JSON form only.
type Detailed interface {
	Data() any
}

type scanData struct {
	Cells []Cell     `json:"cells"`
	Seen  []Sighting `json:"seen"`
}

// Data is the scan record payload.
func (e Scanned) Data() any {
	d := scanData{Cells: e.Cells, Seen: e.Seen}
	if d.Cells == nil {
		d.Cells = []Cell{}
	}
	if d.Seen == nil {
		d.Seen = []Sighting{}
	}
	return d
}

type hitData struct {
	By string `json:"by"`
}

// Data is the hit payload: who fired the bullet.
func (e Hit) Data() any { return hitData{By: e.Shooter} }

// ArgKind says what an event argument is.
type ArgKind int

// Argument kinds.
const (
	ArgBot ArgKind = iota // a bot id
	ArgDir                // N, E, S or W
	ArgInt                // an integer
)

// specs is the registry of event types and their argument kinds. Register
// every new event type here: scenario expectations are validated against it.
var specs = map[string][]ArgKind{
	"Moved":   {ArgBot, ArgDir},
	"Bumped":  {ArgBot, ArgDir},
	"Fired":   {ArgBot, ArgDir},
	"Clicked": {ArgBot, ArgDir},
	"Hit":     {ArgBot, ArgDir},
	"Scanned": {ArgBot, ArgDir, ArgInt, ArgInt, ArgInt},

	"Killed":      {ArgBot, ArgBot},
	"Respawned":   {ArgBot, ArgInt, ArgInt},
	"CoinTaken":   {ArgBot, ArgInt, ArgInt},
	"CoinSpawned": {ArgInt, ArgInt},
	"Joined":      {ArgBot, ArgInt, ArgInt},
	"Left":        {ArgBot},
	"Kicked":      {ArgBot},
}

// Known reports whether name is a registered event type name.
func Known(name string) bool {
	_, ok := specs[name]
	return ok
}

// ValidatePattern checks an expectation pattern (event name plus a prefix of
// its arguments). isBot says whether a bot id exists.
func ValidatePattern(words []string, isBot func(string) bool) error {
	if len(words) == 0 {
		return errors.New("empty event pattern")
	}
	kinds, ok := specs[words[0]]
	if !ok {
		return fmt.Errorf("unknown event %q", words[0])
	}
	args := words[1:]
	if len(args) > len(kinds) {
		return fmt.Errorf("too many arguments for %s: got %d, max %d", words[0], len(args), len(kinds))
	}
	for i, a := range args {
		if err := checkArg(kinds[i], a, isBot); err != nil {
			return fmt.Errorf("%s: %w", words[0], err)
		}
	}
	return nil
}

func checkArg(k ArgKind, a string, isBot func(string) bool) error {
	switch k {
	case ArgBot:
		if !isBot(a) {
			return fmt.Errorf("no bot %q on the map", a)
		}
	case ArgDir:
		if _, ok := geo.ParseDir(a); !ok {
			return fmt.Errorf("bad direction %q (want N, E, S or W)", a)
		}
	case ArgInt:
		if _, err := strconv.Atoi(a); err != nil {
			return fmt.Errorf("bad integer %q", a)
		}
	}
	return nil
}

// Text is the stable textual form: the name followed by the arguments.
func Text(e Event) string {
	return strings.Join(append([]string{e.Name()}, e.Args()...), " ")
}

type jsonLine struct {
	Beat int      `json:"beat"`
	Type string   `json:"type"`
	Args []string `json:"args"`
	Data any      `json:"data,omitempty"`
}

// JSONLine is the canonical JSON of one logged event (no trailing newline).
func JSONLine(l Logged) ([]byte, error) {
	line := jsonLine{Beat: l.Beat, Type: l.Event.Name(), Args: l.Event.Args()}
	if d, ok := l.Event.(Detailed); ok {
		line.Data = d.Data()
	}
	return json.Marshal(line)
}
