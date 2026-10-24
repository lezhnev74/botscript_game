package sim

import (
	"slices"
	"strings"

	"github.com/lezhnev/arena/engine/domain/brain"
)

// Admit adds a new bot between beats (a player joining the live arena). It
// stands on the same spot a respawn would pick - the free cell farthest from
// every bot, ties broken by the world's seeded RNG - with full HP and the
// respawn shield. It returns false when no cell is free. w is not modified.
func (w World) Admit(id string, r Rules) (World, bool) {
	w.Bots = slices.Clone(w.Bots)
	c := &beatCtx{w: w, rules: r}
	spot, ok := c.pickSpot(c.farthestCells())
	if !ok {
		return w, false
	}
	c.w.Bots = append(c.w.Bots, Bot{ID: id, Pos: spot, HP: r.HP, Shield: r.ShieldBeats, Last: brain.Outcome{OK: true}})
	slices.SortFunc(c.w.Bots, func(a, b Bot) int { return strings.Compare(a.ID, b.ID) })
	return c.w, true
}

// Remove takes a bot out of the arena, alive, waiting to respawn or rebooting (a player
// leaving). Its bullets in flight keep flying. w is not modified.
func (w World) Remove(id string) World {
	w.Bots = slices.DeleteFunc(slices.Clone(w.Bots), func(b Bot) bool { return b.ID == id })
	w.Fallen = slices.DeleteFunc(slices.Clone(w.Fallen), func(f Fallen) bool { return f.Bot.ID == id })
	w.Rebooting = slices.DeleteFunc(slices.Clone(w.Rebooting), func(r Rebooting) bool { return r.Bot.ID == id })
	return w
}
