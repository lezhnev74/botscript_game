package sim

import (
	"github.com/lezhnev/arena/engine/domain/brain"
	"github.com/lezhnev/arena/engine/domain/geo"
)

// shot is a sound made this beat (a real, non-click shot) at the
// source's cell after moves.
type shot struct {
	bot string
	pos geo.Pos
}

// hearPhase runs after the fire phase. Every bot hears the nearest real shot
// fired this beat by another bot within HearRange (Manhattan, walls do not
// block sound; ties go to the lowest bot ID) and remembers its cell
// until a newer sound is heard.
func hearPhase(c *beatCtx) {
	for i := range c.w.Bots {
		b := &c.w.Bots[i]
		if s, ok := c.nearestShot(*b); ok {
			b.Heard = brain.Found{Pos: s.pos, Set: true}
		}
	}
}

// nearestShot is the closest shot to b within range, ignoring b's own.
func (c *beatCtx) nearestShot(b Bot) (shot, bool) {
	var best shot
	found := false
	bestD := 0
	for _, s := range c.shots {
		d := manhattan(b.Pos, s.pos)
		if s.bot == b.ID || d > c.rules.HearRange || (found && !closer(d, s.bot, bestD, best.bot)) {
			continue
		}
		best, bestD, found = s, d, true
	}
	return best, found
}

// closer orders shots by distance, then by shooter ID.
func closer(d int, id string, bestD int, bestID string) bool {
	return d < bestD || (d == bestD && id < bestID)
}
