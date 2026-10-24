package sim

import (
	"github.com/lezhnev/arena/engine/domain/event"
	"github.com/lezhnev/arena/engine/domain/geo"
)

// hit is one bullet reaching one bot.
type hit struct {
	victim  int     // index into World.Bots
	from    geo.Dir // where the bullet came from (opposite of its flight)
	shooter string
}

// bulletPhase runs after new shots (and before scans, which see where it leaves each bullet): every bullet, including those fired this
// beat, flies up to Rules.BulletSpeed cells, one cell at a time, so nothing
// is jumped over. It ends at the first wall (removed silently) or bot (a
// contact for hitPhase). Bots have already moved, so a target that stepped
// out of the line is simply not there. Bullets pass through each other.
func bulletPhase(c *beatCtx) {
	flying := make([]Bullet, 0, len(c.w.Bullets))
	for i, b := range c.w.Bullets {
		if i < c.carried && c.strike(b, b.Pos) {
			continue // a bot walked into this bullet's cell
		}
		if end, alive := c.fly(b); alive {
			b.Pos = end
			flying = append(flying, b)
		}
	}
	c.w.Bullets = flying
}

// fly advances b and reports where it ends and whether it is still in the air.
func (c *beatCtx) fly(b Bullet) (geo.Pos, bool) {
	p := b.Pos
	for range c.rules.BulletSpeed {
		p = p.Step(b.Dir)
		if c.w.Grid.Blocked(p) || c.strike(b, p) {
			return p, false
		}
	}
	return p, true
}

// strike records a contact if a bot stands in cell p, and reports it.
func (c *beatCtx) strike(b Bullet, p geo.Pos) bool {
	for i, bot := range c.w.Bots {
		if bot.Pos == p {
			c.hits = append(c.hits, hit{victim: i, from: b.Dir.Back(), shooter: b.Owner})
			return true
		}
	}
	return false
}

// hitPhase deals 1 damage per contact, emits Hit and tells the victim where
// it came from. With several hits in one beat the victim sees the first one.
// A shielded bot takes nothing, and a bot that already died this beat is no
// target: the bullet is spent and no Hit is emitted. A
// bot whose HP reaches 0 is Killed (credit to that bullet's shooter) and
// leaves the grid once all of the beat's hits are in.
func hitPhase(c *beatCtx) {
	var dead []int
	for _, h := range c.hits {
		if c.applyHit(h) {
			dead = append(dead, h.victim)
		}
	}
	c.bury(dead)
}

// applyHit applies one contact and reports whether it killed the victim.
func (c *beatCtx) applyHit(h hit) bool {
	b := &c.w.Bots[h.victim]
	if b.Shield > 0 || b.HP == 0 {
		return false // spent on a shield, or on a bot that already died this beat
	}
	b.HP--
	if !b.Last.Hit {
		b.Last.Hit, b.Last.HitDir = true, h.from
	}
	c.emit(event.Hit{Bot: b.ID, From: h.from, Shooter: h.shooter})
	if b.HP > 0 {
		return false
	}
	c.emit(event.Killed{Killer: h.shooter, Victim: b.ID})
	return true
}
