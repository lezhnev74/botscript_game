package lang

import (
	"github.com/lezhnev/arena/engine/domain/brain"
	"github.com/lezhnev/arena/engine/domain/geo"
)

// TrailLen is how many beats the trail remembers (LANGUAGE.md section 5).
const TrailLen = 3

// ActIdle is the trail act of a missed beat (the IDLE constant, 4). It never
// reaches the sim: a missed beat is a plain wait there.
const ActIdle ActionKind = 4

// TrailEntry is what happened on one beat: the fields of trail[i]. Act is an
// ActionKind or, for a missed beat, 4 (IDLE). Dir, Hit and Bump are -1 when
// not applicable.
type TrailEntry struct {
	X, Y      int
	Act       ActionKind
	Dir       int
	OK        bool
	Hit, Bump int
}

// newEntry builds the entry for the beat that played issued and ended with
// outcome o at position pos.
func newEntry(issued Action, o brain.Outcome, pos geo.Pos) TrailEntry {
	e := TrailEntry{X: pos.X, Y: pos.Y, Act: issued.Kind, Dir: -1, OK: o.OK, Hit: -1, Bump: -1}
	if issued.Kind != ActWait && issued.Kind != ActIdle {
		e.Dir = issued.Dir
	}
	if o.Hit {
		e.Hit = int(o.HitDir)
	}
	if o.Bumped {
		e.Bump = int(o.BumpDir)
	}
	return e
}

// pushed returns the trail with e as the newest entry, capped at TrailLen.
func pushed(trail []TrailEntry, e TrailEntry) []TrailEntry {
	out := append([]TrailEntry{e}, trail...)
	return out[:min(len(out), TrailLen)]
}

var trailGetters = map[string]func(TrailEntry) int{
	"x":    func(e TrailEntry) int { return e.X },
	"y":    func(e TrailEntry) int { return e.Y },
	"act":  func(e TrailEntry) int { return int(e.Act) },
	"dir":  func(e TrailEntry) int { return e.Dir },
	"ok":   func(e TrailEntry) int { return b2i(e.OK) },
	"hit":  func(e TrailEntry) int { return e.Hit },
	"bump": func(e TrailEntry) int { return e.Bump },
}
