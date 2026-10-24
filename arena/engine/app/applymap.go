package app

import (
	"fmt"
	"slices"

	"github.com/lezhnev/arena/engine/domain/brain"
	"github.com/lezhnev/arena/engine/domain/geo"
	"github.com/lezhnev/arena/engine/domain/sim"
)

// nextMap is a map accepted by ApplyMap, waiting for the next beat boundary.
type nextMap struct {
	text string
	m    geo.Map
}

// ApplyMap starts a new match on the map at the next beat boundary. The
// players stay (same ids, tokens and programs) and rejoin like at a server
// start; positions, HP, reload, memory and trails are fresh, house bots are kept
// (and topped up to MinBots) and the leaderboard is kept. The new match has the
// old seed + 1 and its own live Record, so each game replays exactly: Record
// always describes the current game only. A call before the boundary replaces
// an earlier one. The text must be a room without bots, with at least as many
// floor cells as there are bots (and MinBots).
func (a *Arena) ApplyMap(text string) error {
	m, err := geo.ParseMap(text)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrBadMap, err)
	}
	if len(m.Bots) > 0 {
		return fmt.Errorf("%w: the map must have no bots, found %d", ErrBadMap, len(m.Bots))
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closed {
		return ErrClosed
	}
	if floorCells(m.Grid) < max(a.botCount(), a.cfg.MinBots) {
		return ErrMapTooSmall
	}
	a.nextMap = &nextMap{text: text, m: m}
	return nil
}

// botCount counts the bots in the arena or waiting to join, leaving and detached ones excluded.
func (a *Arena) botCount() int {
	n := 0
	for _, p := range a.players {
		if !p.leaving && !p.detached {
			n++
		}
	}
	return n
}

// startGame swaps in the pending map: a fresh match and Record with the seed
// raised by one, every human player queued to rejoin. Callers hold the lock.
func (a *Arena) startGame() {
	n := a.nextMap
	a.nextMap = nil
	a.match.Close()
	a.cfg.Map, a.cfg.Seed = n.text, a.cfg.Seed+1
	a.floor = floorCells(n.m.Grid)
	a.match = NewMatch(sim.NewWorld(n.m, a.cfg.Rules, a.cfg.Seed), a.cfg.Rules, map[string]brain.Brain{})
	a.rec = Record{Version: RecordVersion, Rules: a.cfg.Rules, Map: a.cfg.Map, Seed: a.cfg.Seed}
	a.reseat()
	a.mapDirty = true
}

// reseat queues every player of the old match to join again in their
// original order. Kicked players are dropped, detached ones stay out. House bots keep their ids, names and
// programs (so their leaderboard lines carry on); only as many as MinBots still
// asks for are kept, the oldest first.
func (a *Arena) reseat() {
	keep := max(0, a.cfg.MinBots-a.dropKicked())
	a.queue = queue{}
	for _, p := range a.bySeq() {
		if p.house && !takeSeat(&keep) {
			delete(a.players, p.id)
			continue
		}
		p.inMatch, p.leaving, p.swappedAt, p.movedAt, p.badgePending = false, false, 0, 0, false
		if !p.detached { // a detached player stays out until it returns
			a.queue.joins = append(a.queue.joins, p.id)
		}
	}
}

// dropKicked forgets the players kicked before the new game and counts the
// human players that stay in the arena.
func (a *Arena) dropKicked() (humans int) {
	for id, p := range a.players {
		if p.kicked {
			delete(a.players, id)
		} else if !p.house && !p.detached {
			humans++
		}
	}
	return humans
}

// bySeq lists the players in creation order.
func (a *Arena) bySeq() []*player {
	all := make([]*player, 0, len(a.players))
	for _, p := range a.players {
		all = append(all, p)
	}
	slices.SortFunc(all, func(x, y *player) int { return x.seq - y.seq })
	return all
}

// takeSeat uses one of the house seats left and reports whether there was one.
func takeSeat(left *int) bool {
	if *left == 0 {
		return false
	}
	*left--
	return true
}

// currentFloor is the floor of the map the arena will play next: the pending
// map when one is queued, else the map in play. Callers hold the lock.
func (a *Arena) currentFloor() int {
	if a.nextMap != nil {
		return floorCells(a.nextMap.m.Grid)
	}
	return a.floor
}
