package sim

import (
	"slices"
	"strings"

	"github.com/lezhnev/arena/engine/domain/brain"
)

// Rebooting is a bot taken off the map while its program restarts. Like a
// fallen bot it is off the grid: nothing can see, hit or bump it. Bot holds
// the state it comes back with (Pos is the cell it left). A bot that was dead
// when the reboot began (Dead) comes back as a normal respawn instead.
type Rebooting struct {
	Bot   Bot
	Left  int  // beats until it comes back
	Total int  // the length of the reboot, for progress bars
	Dead  bool // it was waiting to respawn when the reboot began
}

// Reboot takes a bot off the map for n beats (a player saving a new program).
// A dead bot comes back at the later of its respawn and the end of the reboot.
// It reports false, changing nothing, when the bot is not live or fallen or
// is already rebooting (the countdown is never restarted). w is not modified.
func (w World) Reboot(id string, n int) (World, bool) {
	if i := slices.IndexFunc(w.Bots, func(b Bot) bool { return b.ID == id }); i >= 0 {
		b := w.Bots[i]
		w.Bots = slices.Delete(slices.Clone(w.Bots), i, i+1)
		w.Rebooting = addRebooting(w.Rebooting, Rebooting{Bot: b, Left: n, Total: n})
		return w, true
	}
	i := slices.IndexFunc(w.Fallen, func(f Fallen) bool { return f.Bot.ID == id })
	if i < 0 {
		return w, false
	}
	f := w.Fallen[i]
	w.Fallen = slices.Delete(slices.Clone(w.Fallen), i, i+1)
	w.Rebooting = addRebooting(w.Rebooting, Rebooting{Bot: f.Bot, Left: max(f.Wait+1, n), Total: n, Dead: true})
	return w, true
}

// IsRebooting reports whether the bot is off the map for a reboot.
func (w World) IsRebooting(id string) bool {
	return slices.ContainsFunc(w.Rebooting, func(r Rebooting) bool { return r.Bot.ID == id })
}

func (w World) rebooting(id string) (Rebooting, bool) {
	for _, r := range w.Rebooting {
		if r.Bot.ID == id {
			return r, true
		}
	}
	return Rebooting{}, false
}

// addRebooting returns a copy of list with r added, sorted by bot id.
func addRebooting(list []Rebooting, r Rebooting) []Rebooting {
	out := append(slices.Clone(list), r)
	slices.SortFunc(out, func(a, b Rebooting) int { return strings.Compare(a.Bot.ID, b.Bot.ID) })
	return out
}

// rebootPhase counts the reboots down and puts back the bots whose time is
// up. A bot with no free cell to appear on stays off the map and tries again
// next beat.
func rebootPhase(c *beatCtx) {
	var waiting []Rebooting
	for _, r := range c.w.Rebooting {
		r.Left = max(r.Left-1, 0)
		if r.Left > 0 || !c.comeBack(r) {
			waiting = append(waiting, r)
		}
	}
	c.w.Rebooting = waiting
}

// comeBack puts a rebooted bot on the respawn spot. It keeps HP, the gun and
// its bonus shots; it gets the respawn shield and a new life, which restarts
// the script's loop() with a clear trail.
func (c *beatCtx) comeBack(r Rebooting) bool {
	if r.Dead {
		return c.respawn(r.Bot)
	}
	spot, ok := c.pickSpot(c.farthestCells())
	if !ok {
		return false
	}
	b := r.Bot
	b.Pos, b.Shield, b.Life = spot, c.rules.ShieldBeats, b.Life+1
	b.Last, b.Read, b.Heard = brain.Outcome{OK: true}, brain.Readings{}, brain.Found{}
	c.w.Bots = append(c.w.Bots, b)
	slices.SortFunc(c.w.Bots, func(x, y Bot) int { return strings.Compare(x.ID, y.ID) })
	return true
}
