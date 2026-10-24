package app

import (
	"errors"
	"fmt"

	"github.com/lezhnev/arena/engine/domain/brain"
	"github.com/lezhnev/arena/engine/domain/event"
	"github.com/lezhnev/arena/engine/domain/sim"
)

// joinOp is a scheduled Join with its brain already built.
type joinOp struct {
	beat  int
	bot   string
	brain brain.Brain
}

// Err is the first failure of a scheduled join, leave or edit (a Record that
// asks for the impossible, e.g. a join into a full arena). Step goes on; Replay
// checks Err after every beat.
func (m *Match) Err() error { return m.err }

func (m *Match) fail(err error) {
	m.err = errors.Join(m.err, err)
}

// Join adds a bot standing on the respawn spot (the free cell farthest from
// the others, seeded RNG). Match owns b from here on and closes it when the
// join fails. Call it between beats.
func (m *Match) Join(id string, b brain.Brain) error {
	if _, taken := m.brains[id]; taken || id == "" {
		closeBrain(b)
		return fmt.Errorf("join: bot id %q is empty or already in the arena", id)
	}
	w, ok := m.world.Admit(id, m.rules)
	if !ok {
		closeBrain(b)
		return fmt.Errorf("join: no free cell for bot %q", id)
	}
	m.world, m.brains[id] = w, b
	m.notes = append(m.notes, joinedAt(w, id))
	return nil
}

// Leave removes a bot, alive or waiting to respawn, and closes its brain.
func (m *Match) Leave(id string) error { return m.leave(id, false) }

func (m *Match) leave(id string, kicked bool) error {
	b, ok := m.brains[id]
	if !ok {
		return fmt.Errorf("leave: no bot %q in the arena", id)
	}
	closeBrain(b)
	delete(m.brains, id)
	m.world = m.world.Remove(id)
	m.notes = append(m.notes, event.Left{Bot: id})
	if kicked {
		m.notes = append(m.notes, event.Kicked{Bot: id})
	}
	return nil
}

func joinedAt(w sim.World, id string) event.Joined {
	for _, b := range w.Bots {
		if b.ID == id {
			return event.Joined{Bot: id, X: b.Pos.X, Y: b.Pos.Y}
		}
	}
	return event.Joined{Bot: id}
}

func (m *Match) applyLeaves(beat int) {
	rest := m.leaves[:0]
	for _, l := range m.leaves {
		if l.Beat == beat {
			m.fail(m.leave(l.Bot, l.Kicked))
		} else {
			rest = append(rest, l)
		}
	}
	m.leaves = rest
}

func (m *Match) applyJoins(beat int) {
	var rest []joinOp
	for _, j := range m.joins {
		if j.beat == beat {
			m.fail(m.Join(j.bot, j.brain))
		} else {
			rest = append(rest, j)
		}
	}
	m.joins = rest
}
