// Package brain is the port between the simulation and whatever decides a
// bot's actions (scripted lists in scenarios, BOTSCRIPT programs in play).
package brain

import (
	"fmt"
	"strings"

	"github.com/lezhnev/arena/engine/domain/geo"
)

// Kind is the type of an action. One action per bot per beat.
type Kind int

// The action set from VISION: nothing, step, look, shoot.
const (
	Wait Kind = iota
	Move
	Scan
	Fire
)

var kindNames = [...]string{"wait", "move", "scan", "fire"}

func (k Kind) String() string { return kindNames[k] }

// hasDir says whether the action takes a direction (wait does not).
func (k Kind) hasDir() bool { return k != Wait }

// Action is what a bot does this beat; Dir is ignored for Wait.
type Action struct {
	Kind Kind
	Dir  geo.Dir
}

func (a Action) String() string {
	if !a.Kind.hasDir() {
		return a.Kind.String()
	}
	return a.Kind.String() + " " + a.Dir.String()
}

// Outcome is how the bot's previous beat went. Before beat 1 it is
// Outcome{OK: true}.
type Outcome struct {
	Act     Action
	OK      bool    // move: I actually moved; scan/wait: always true
	Bumped  bool    // I walked into a bot (walls never bump)
	BumpDir geo.Dir // valid only when Bumped: the direction I bumped towards
	Hit     bool    // a bullet hit me (a hit beats a bump when both happen)
	HitDir  geo.Dir // valid only when Hit: where the bullet came from
}

// Found is a place a bot remembers (the heard sound): Set is false for "none".
type Found struct {
	Pos geo.Pos
	Set bool
}

// Readings are the result of the bot's last scan: Dir is the scan direction,
// the levels (0-3) are the best alignment per kind. They persist until the
// next scan overwrites them; before any scan they are all zero.
type Readings struct {
	Dir                 geo.Dir
	Enemy, Coin, Bullet int
}

// Perception is what a bot knows when it decides.
type Perception struct {
	Beat int // the beat about to be played (1-based)
	Pos  geo.Pos
	HP   int
	Last Outcome  // the result of the previous beat
	Read Readings // the last scan's readings

	Loaded bool // a fire would shoot: reloaded, or a bonus shot is stored
	Bonus  int  // stored bonus shots

	// Life counts respawns: it changes exactly on the first beat of a new life,
	// even when RespawnBeats is 0 and no beat number is skipped.
	Life int

	Heard Found // the place of the last sound heard (Set false = none, or nothing heard yet)

	Width, Height int      // arena size (WIDTH, HEIGHT)
	Grid          geo.Grid // the match terrain (read-only value), for wall(x, y)
	Open          [4]bool  // per Dir: the neighbouring cell is not a wall (bots do not count)
}

// Brain decides a bot's action. It must be deterministic for the same inputs.
type Brain interface {
	Decide(p Perception) Action
}

// ParseAction parses "wait", "move S", "scan N" or "fire E".
func ParseAction(s string) (Action, error) {
	f := strings.Fields(s)
	if len(f) == 0 {
		return Action{}, fmt.Errorf("empty action")
	}
	for k, n := range kindNames {
		if n == f[0] {
			return withDir(Kind(k), f[1:])
		}
	}
	return Action{}, fmt.Errorf("unknown action %q (want wait, move, scan or fire)", f[0])
}

func withDir(k Kind, args []string) (Action, error) {
	if !k.hasDir() {
		if len(args) != 0 {
			return Action{}, fmt.Errorf("%s takes no direction", k)
		}
		return Action{Kind: k}, nil
	}
	if len(args) != 1 {
		return Action{}, fmt.Errorf("%s needs exactly one direction", k)
	}
	d, ok := geo.ParseDir(args[0])
	if !ok {
		return Action{}, fmt.Errorf("bad direction %q (want N, E, S or W)", args[0])
	}
	return Action{Kind: k, Dir: d}, nil
}

// Scripted plays a fixed action list, then waits forever.
type Scripted struct {
	actions []Action
	next    int
}

// NewScripted returns a brain that plays actions in order.
func NewScripted(actions []Action) *Scripted { return &Scripted{actions: actions} }

// Decide returns the next scripted action, or Wait once the list is spent.
func (s *Scripted) Decide(Perception) Action {
	if s.next >= len(s.actions) {
		return Action{Kind: Wait}
	}
	a := s.actions[s.next]
	s.next++
	return a
}

// ParseScript parses a comma-separated action list ("fire E, move S"); a blank
// list is an empty script.
func ParseScript(src string) ([]Action, error) {
	if strings.TrimSpace(src) == "" {
		return nil, nil
	}
	var out []Action
	for _, part := range strings.Split(src, ",") {
		a, err := ParseAction(part)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, nil
}

// FormatScript is the inverse of ParseScript.
func FormatScript(acts []Action) string {
	parts := make([]string, len(acts))
	for i, a := range acts {
		parts[i] = a.String()
	}
	return strings.Join(parts, ", ")
}
