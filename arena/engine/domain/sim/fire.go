package sim

import (
	"github.com/lezhnev/arena/engine/domain/brain"
	"github.com/lezhnev/arena/engine/domain/event"
	"github.com/lezhnev/arena/engine/domain/geo"
)

// firePhase runs after moves. A bot that fires with a ready gun spawns a
// bullet in its own cell (bulletPhase moves it this same beat, cell by cell,
// starting with the neighbouring cell) and starts the reload. With the
// gun reloading it spends a stored bonus shot instead, which leaves the
// reload untouched; with neither it clicks and the beat is wasted.
func firePhase(c *beatCtx) {
	for i, b := range c.w.Bots {
		a := c.acts[b.ID]
		if a.Kind != brain.Fire {
			continue
		}
		if !a.Dir.Valid() {
			c.w.Bots[i].Last.OK = false
			continue
		}
		c.fire(i, a.Dir)
	}
}

func (c *beatCtx) fire(i int, d geo.Dir) {
	b := &c.w.Bots[i]
	if !b.spendShot(c.rules.Reload) {
		b.Last.OK = false
		c.emit(event.Clicked{Bot: b.ID, Dir: d})
		return
	}
	c.w.Bullets = append(c.w.Bullets, Bullet{Pos: b.Pos, Dir: d, Owner: b.ID})
	c.shots = append(c.shots, shot{bot: b.ID, pos: b.Pos})
	c.emit(event.Fired{Bot: b.ID, Dir: d})
}

// spendShot uses the regular shot if ready (and starts the reload), else a
// bonus shot. It reports whether a shot was available.
func (b *Bot) spendShot(reload int) bool {
	switch {
	case b.Cooldown == 0:
		b.Cooldown = reload
	case b.Bonus > 0:
		b.Bonus--
	default:
		return false
	}
	return true
}

// reloadPhase counts every reload down by one beat, including the one that
// just started: a shot on beat t is ready again on beat t+Reload.
func reloadPhase(c *beatCtx) {
	for i := range c.w.Bots {
		if c.w.Bots[i].Cooldown > 0 {
			c.w.Bots[i].Cooldown--
		}
	}
}
