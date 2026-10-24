package sim

import (
	"slices"
	"strings"

	"github.com/lezhnev/arena/engine/domain/brain"
	"github.com/lezhnev/arena/engine/domain/event"
	"github.com/lezhnev/arena/engine/domain/geo"
)

// bury takes the bots at the given indices off the grid: they lose their
// bonus shots and wait RespawnBeats for respawnPhase to bring them back.
func (c *beatCtx) bury(idx []int) {
	if len(idx) == 0 {
		return
	}
	alive := make([]Bot, 0, len(c.w.Bots))
	for i, b := range c.w.Bots {
		if !slices.Contains(idx, i) {
			alive = append(alive, b)
			continue
		}
		b.Bonus = 0
		c.w.Fallen = append(c.w.Fallen, Fallen{Bot: b, Wait: c.rules.RespawnBeats})
	}
	c.w.Bots = alive
	slices.SortFunc(c.w.Fallen, func(a, b Fallen) int { return strings.Compare(a.Bot.ID, b.Bot.ID) })
}

// respawnPhase ticks shields down, then brings back every dead bot whose wait
// is over (in ID order, so each sees the ones placed before it). A bot with no
// free cell to appear on stays dead and tries again next beat.
func respawnPhase(c *beatCtx) {
	for i := range c.w.Bots {
		c.w.Bots[i].Shield = max(c.w.Bots[i].Shield-1, 0)
	}
	var waiting []Fallen
	for _, f := range c.w.Fallen {
		switch {
		case f.Wait > 0:
			f.Wait--
			waiting = append(waiting, f)
		case !c.respawn(f.Bot):
			waiting = append(waiting, f)
		}
	}
	c.w.Fallen = waiting
}

// respawn puts the bot back on the best free cell with full HP and a shield.
func (c *beatCtx) respawn(b Bot) bool {
	spot, ok := c.pickSpot(c.farthestCells())
	if !ok {
		return false
	}
	b = Bot{ID: b.ID, Pos: spot, HP: c.rules.HP, Shield: c.rules.ShieldBeats, Life: b.Life + 1, Last: brain.Outcome{OK: true}}
	c.w.Bots = append(c.w.Bots, b)
	slices.SortFunc(c.w.Bots, func(x, y Bot) int { return strings.Compare(x.ID, y.ID) })
	c.emit(event.Respawned{Bot: b.ID, X: spot.X, Y: spot.Y})
	return true
}

// farthestCells are the free cells whose nearest bot is as far away as
// possible (Manhattan distance), in row-major order.
func (c *beatCtx) farthestCells() []geo.Pos {
	var best []geo.Pos
	far := -1
	for _, p := range c.freeCells() {
		switch d := c.nearestBot(p); {
		case d > far:
			best, far = []geo.Pos{p}, d
		case d == far:
			best = append(best, p)
		}
	}
	return best
}

// nearestBot is the distance from p to the closest bot, or 0 with no bots.
func (c *beatCtx) nearestBot(p geo.Pos) int {
	nearest := 0
	for i, b := range c.w.Bots {
		if d := manhattan(p, b.Pos); i == 0 || d < nearest {
			nearest = d
		}
	}
	return nearest
}

func manhattan(a, b geo.Pos) int { return abs(a.X-b.X) + abs(a.Y-b.Y) }

// pickSpot draws one of the candidates with the world's seeded RNG.
func (c *beatCtx) pickSpot(cells []geo.Pos) (geo.Pos, bool) {
	if len(cells) == 0 {
		return geo.Pos{}, false
	}
	i, r := c.w.rng().intn(len(cells))
	c.w.RNG = r.state
	return cells[i], true
}

// freeCells lists the floor cells holding no bot, coin or bullet, row by row.
func (c *beatCtx) freeCells() []geo.Pos {
	var cells []geo.Pos
	for y := range c.w.Grid.Height() {
		for x := range c.w.Grid.Width() {
			if p := (geo.Pos{X: x, Y: y}); c.isFree(p) {
				cells = append(cells, p)
			}
		}
	}
	return cells
}

func (c *beatCtx) isFree(p geo.Pos) bool {
	return !c.w.Grid.Blocked(p) && !slices.Contains(c.w.Coins, p) &&
		!slices.ContainsFunc(c.w.Bots, func(b Bot) bool { return b.Pos == p }) &&
		!slices.ContainsFunc(c.w.Bullets, func(b Bullet) bool { return b.Pos == p })
}
