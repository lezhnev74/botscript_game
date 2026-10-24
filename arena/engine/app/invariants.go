package app

import (
	"errors"
	"fmt"
	"slices"

	"github.com/lezhnev/arena/engine/domain/event"
	"github.com/lezhnev/arena/engine/domain/geo"
	"github.com/lezhnev/arena/engine/domain/sim"
)

// CheckInvariants verifies what must hold after every beat, whatever the bots
// do. log is the whole event log so far. Violations are joined into one error.
//
//   - live bots: on floor cells, one bot per cell, 1 <= HP <= Rules.HP, bonus <= Rules.BonusMax
//   - bullets: inside the grid and not in a wall
//   - fallen and rebooting bots are off the grid (no live bot with the same id)
//   - deaths balance: Killed events == Respawned events + bots that left while dead + bots still waiting to respawn
func CheckInvariants(w sim.World, r sim.Rules, log []event.Logged) error {
	return errors.Join(
		checkBots(w, r),
		checkBullets(w),
		checkFallen(w),
		checkDeaths(w, log),
	)
}

func checkBots(w sim.World, r sim.Rules) error {
	var errs []error
	cells := make(map[geo.Pos]string, len(w.Bots))
	for _, b := range w.Bots {
		if w.Grid.Blocked(b.Pos) {
			errs = append(errs, fmt.Errorf("bot %s in a wall or outside at %v", b.ID, b.Pos))
		}
		if other, taken := cells[b.Pos]; taken {
			errs = append(errs, fmt.Errorf("bots %s and %s share cell %v", other, b.ID, b.Pos))
		}
		cells[b.Pos] = b.ID
		errs = append(errs, checkBotStats(b, r))
	}
	return errors.Join(errs...)
}

func checkBotStats(b sim.Bot, r sim.Rules) error {
	var errs []error
	if b.HP < 1 || b.HP > r.HP {
		errs = append(errs, fmt.Errorf("bot %s hp %d outside 1..%d", b.ID, b.HP, r.HP))
	}
	if b.Bonus < 0 || b.Bonus > r.BonusMax {
		errs = append(errs, fmt.Errorf("bot %s bonus %d outside 0..%d", b.ID, b.Bonus, r.BonusMax))
	}
	return errors.Join(errs...)
}

func checkBullets(w sim.World) error {
	var errs []error
	for _, b := range w.Bullets {
		if w.Grid.Blocked(b.Pos) {
			errs = append(errs, fmt.Errorf("bullet of %q in a wall or outside at %v", b.Owner, b.Pos))
		}
	}
	return errors.Join(errs...)
}

func checkFallen(w sim.World) error {
	var errs []error
	for _, f := range w.Fallen {
		errs = append(errs, checkOffGrid(w, f.Bot.ID, "fallen"))
	}
	for _, r := range w.Rebooting {
		errs = append(errs, checkOffGrid(w, r.Bot.ID, "rebooting"))
	}
	return errors.Join(errs...)
}

func checkOffGrid(w sim.World, id, what string) error {
	if slices.ContainsFunc(w.Bots, func(b sim.Bot) bool { return b.ID == id }) {
		return fmt.Errorf("%s bot %s is also on the grid", what, id)
	}
	return nil
}

// waitingToRespawn counts the dead bots: fallen ones and those that died
// before a reboot began.
func waitingToRespawn(w sim.World) int {
	n := len(w.Fallen)
	for _, r := range w.Rebooting {
		if r.Dead {
			n++
		}
	}
	return n
}

func checkDeaths(w sim.World, log []event.Logged) error {
	var killed, respawned, leftDead int
	dead := map[string]bool{}
	for _, l := range log {
		switch e := l.Event.(type) {
		case event.Killed:
			killed++
			dead[e.Victim] = true
		case event.Respawned:
			respawned++
			delete(dead, e.Bot)
		case event.Left:
			if dead[e.Bot] {
				leftDead++
			}
			delete(dead, e.Bot)
		}
	}
	if killed != respawned+leftDead+waitingToRespawn(w) {
		return fmt.Errorf("deaths do not balance: %d Killed, %d Respawned, %d left while dead, %d waiting",
			killed, respawned, leftDead, waitingToRespawn(w))
	}
	return nil
}
