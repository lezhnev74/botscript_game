package sim

import (
	"github.com/lezhnev/arena/engine/domain/brain"
	"github.com/lezhnev/arena/engine/domain/event"
	"github.com/lezhnev/arena/engine/domain/geo"
)

// intent is a bot's wish to step into a cell.
type intent struct {
	idx int // index into World.Bots
	dir geo.Dir
	to  geo.Pos
}

// movePhase resolves all moves on the start-of-beat snapshot:
//   - a wall or an invalid direction: no move, no event, Last.OK=false;
//   - a cell occupied at the start of the beat (even by a bot that is moving
//     away this beat): the mover bumps, the occupant notices nothing;
//   - a free cell claimed by two or more bots (head-on or perpendicular): all
//     claimants bump. A swap is covered by occupancy: both targets are
//     occupied at the start;
//   - otherwise the bot moves.
//
// A bump carries the direction the mover tried to step.
func movePhase(c *beatCtx) {
	intents := c.moveIntents()
	claims := countClaims(intents)
	occupied := c.occupied()
	for _, in := range intents {
		switch {
		case occupied[in.to] || claims[in.to] > 1:
			c.bump(in)
		default:
			c.step(in)
		}
	}
}

// moveIntents lists, in bot-ID order, the moves that are not stopped by walls.
func (c *beatCtx) moveIntents() []intent {
	var out []intent
	for i, b := range c.w.Bots {
		a := c.acts[b.ID]
		if a.Kind != brain.Move {
			continue
		}
		if !a.Dir.Valid() || c.w.Grid.Blocked(b.Pos.Step(a.Dir)) {
			c.w.Bots[i].Last.OK = false
			continue
		}
		out = append(out, intent{idx: i, dir: a.Dir, to: b.Pos.Step(a.Dir)})
	}
	return out
}

func countClaims(intents []intent) map[geo.Pos]int {
	claims := make(map[geo.Pos]int, len(intents))
	for _, in := range intents {
		claims[in.to]++
	}
	return claims
}

// occupied is the set of cells holding a bot at the start of the beat.
func (c *beatCtx) occupied() map[geo.Pos]bool {
	cells := make(map[geo.Pos]bool, len(c.w.Bots))
	for _, b := range c.w.Bots {
		cells[b.Pos] = true
	}
	return cells
}

func (c *beatCtx) bump(in intent) {
	b := &c.w.Bots[in.idx]
	b.Last.OK, b.Last.Bumped, b.Last.BumpDir = false, true, in.dir
	c.emit(event.Bumped{Bot: b.ID, Dir: in.dir})
}

func (c *beatCtx) step(in intent) {
	b := &c.w.Bots[in.idx]
	b.Pos = in.to
	c.emit(event.Moved{Bot: b.ID, Dir: in.dir})
}
