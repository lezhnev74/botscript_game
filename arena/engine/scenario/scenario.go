// Package scenario is the .scn harness: it parses scenario files, runs them
// through the beat loop and checks expectations against the event log.
package scenario

import (
	"github.com/lezhnev/arena/engine/domain/brain"
	"github.com/lezhnev/arena/engine/domain/geo"
	"github.com/lezhnev/arena/engine/domain/sim"
)

// ExpKind says what an expectation asserts.
type ExpKind int

// Expectation kinds.
const (
	ExpEvent ExpKind = iota // an event occurs (ordered subsequence)
	ExpNo                   // no matching event in the beat
	ExpState                // a world value after a beat (or at the end)
)

// Expectation is one line of the expect block. Words are the event pattern
// (token prefix of an event's text) or, for ExpState, "<bot>.<field> <value>".
type Expectation struct {
	Line  int
	Kind  ExpKind
	Beat  int  // 1-based; for ExpState the beat whose resulting world is read
	Final bool // ExpState only: read the world after the last beat
	Words []string
}

// Frame is an ASCII snapshot of the world after Beat (0 = the initial world).
type Frame struct {
	Line int
	Beat int
	Text string
}

// BotSpec is a bot's behaviour in a scenario.
type BotSpec struct {
	Line    int // line of the bot declaration in the .scn file
	ID      string
	Script  []brain.Action
	Program string // BOTSCRIPT source; non-empty makes this a program bot (Script is then unused)
}

// EditSpec is a hot-swap from the `edits:` block: at the start of Beat the
// program bot Bot continues with Program (BOTSCRIPT source). With Reboot > 0
// the bot first leaves the map for that many beats.
type EditSpec struct {
	Line    int
	Beat    int
	Bot     string
	Program string
	Reboot  int
}

// StartField is one `key=value` of a start override.
type StartField struct {
	Key   string
	Value int
}

// StartSpec overrides a bot's initial state (hp, bonus, cooldown).
type StartSpec struct {
	Line   int
	ID     string
	Fields []StartField
}

// BulletSpec is a bullet in flight at the start (`bullets:` block). Owner is
// the shooter's bot id, or empty.
type BulletSpec struct {
	Line  int
	Pos   geo.Pos
	Dir   geo.Dir
	Owner string
}

// Scenario is a parsed .scn file.
type Scenario struct {
	Name    string
	Rules   sim.Rules
	Seed    uint64
	Map     geo.Map
	Bots    []BotSpec // sorted by ID
	Start   []StartSpec
	Bullets []BulletSpec
	Edits   []EditSpec // file order
	Beats   int
	Expect  []Expectation
	Frames  []Frame
}
