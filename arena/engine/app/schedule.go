package app

import (
	"fmt"
	"slices"

	"github.com/lezhnev/arena/engine/domain/brain"
	"github.com/lezhnev/arena/engine/domain/geo"
)

// schedule validates the joins, edit log and leaves of the record and queues
// them on the match. known maps every bot id that is ever in the arena to
// whether it is a program bot (the only kind an edit may target).
func (rec Record) schedule(match *Match, m geo.Map) error {
	known := map[string]bool{}
	for id, b := range match.brains {
		_, known[id] = b.(swappable)
	}
	if err := rec.queueJoins(match, m, known); err != nil {
		return err
	}
	swaps, err := rec.swaps(known)
	if err != nil {
		return err
	}
	match.swaps = swaps
	return rec.queueLeaves(match, known)
}

func (rec Record) queueJoins(match *Match, m geo.Map, known map[string]bool) error {
	for i, j := range rec.Joins {
		b, err := rec.joinBrain(j, m, known)
		if err != nil {
			return fmt.Errorf("joins[%d]: %w", i, err)
		}
		_, known[j.Bot] = b.(swappable)
		match.joins = append(match.joins, joinOp{beat: j.Beat, bot: j.Bot, brain: b})
	}
	return nil
}

func (rec Record) joinBrain(j Join, m geo.Map, known map[string]bool) (brain.Brain, error) {
	_, listed := known[j.Bot]
	switch {
	case j.Beat < 1:
		return nil, fmt.Errorf("beat %d: joins start at beat 1", j.Beat)
	case j.Bot == "" || (listed && !rec.hasLeft(j.Bot, j.Beat)) || hasBot(m, j.Bot):
		return nil, fmt.Errorf("bot id %q is empty, a map bot or joins twice", j.Bot)
	}
	b, err := NewBrain(j.Brain, rec.Seed, j.Bot)
	if err != nil {
		return nil, fmt.Errorf("bot %s: %w", j.Bot, err)
	}
	return b, nil
}

// hasLeft reports whether the bot has a Leave at or before the beat, so it may
// join again (a bot that detached and returned).
func (rec Record) hasLeft(bot string, beat int) bool {
	return slices.ContainsFunc(rec.Leaves, func(l Leave) bool { return l.Bot == bot && l.Beat <= beat })
}

func (rec Record) queueLeaves(match *Match, known map[string]bool) error {
	for i, l := range rec.Leaves {
		if _, ok := known[l.Bot]; !ok || l.Beat < 1 {
			return fmt.Errorf("leaves[%d]: beat %d, bot %q: want beat >= 1 and a bot of the arena", i, l.Beat, l.Bot)
		}
		match.leaves = append(match.leaves, l)
	}
	return nil
}
