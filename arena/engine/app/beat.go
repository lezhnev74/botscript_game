package app

import (
	"slices"
	"strconv"

	"github.com/lezhnev/arena/engine/domain/sim"
	"github.com/lezhnev/arena/engine/lang"
)

// beginBeat applies everything queued since the last beat, in the fixed order
// of the Record: leaves, then joins, then edits, each in arrival order. House
// bots are balanced first so their leaves and joins fall in the same lists.
func (a *Arena) beginBeat(beat int) {
	q := a.queue
	a.queue = queue{}
	q.leaves, q.joins = a.balance(q.leaves, q.joins)
	for _, id := range q.leaves {
		a.applyLeave(beat, id)
	}
	var waiting []string
	for _, id := range q.joins {
		if !a.applyJoin(beat, id) {
			waiting = append(waiting, id)
		}
	}
	a.queue.joins = waiting
	for _, e := range q.edits {
		a.applyEdit(beat, e)
	}
}

// balance keeps the arena at MinBots or more: house bots leave (oldest first)
// as humans arrive, and new ones join when humans leave. Joins still waiting
// for room count as present.
func (a *Arena) balance(leaves, joins []string) ([]string, []string) {
	houses := a.houses()
	humans, houseJoins := a.countJoins(joins)
	for _, p := range a.players {
		if p.inMatch && !p.house {
			humans++
		}
	}
	need := max(0, a.cfg.MinBots-(humans-len(leaves)))
	if drop := len(houses) + houseJoins - need; drop > 0 {
		leaves = append(leaves, houses[:min(drop, len(houses))]...)
	}
	for range need - len(houses) - houseJoins {
		joins = append(joins, a.newHouse().id)
	}
	return leaves, joins
}

// countJoins splits the queued joins into human and house ones.
func (a *Arena) countJoins(joins []string) (humans, houses int) {
	for _, id := range joins {
		if a.players[id].house {
			houses++
		} else {
			humans++
		}
	}
	return humans, houses
}

// houses lists the house bots in the arena, oldest first.
func (a *Arena) houses() []string {
	var ps []*player
	for _, p := range a.players {
		if p.house && p.inMatch {
			ps = append(ps, p)
		}
	}
	slices.SortFunc(ps, func(x, y *player) int { return x.seq - y.seq })
	ids := make([]string, len(ps))
	for i, p := range ps {
		ids[i] = p.id
	}
	return ids
}

// newHouse makes the next house bot, cycling through the house programs.
func (a *Arena) newHouse() *player {
	names := a.cfg.Houses
	if len(names) == 0 {
		names = lang.HouseBotNames()
	}
	src, _ := lang.HouseBot(names[a.houseN%len(names)]) // names are checked in NewArena
	a.houseN++
	id := "house-" + strconv.Itoa(a.houseN)
	p := &player{id: id, name: a.nextHouseName(a.houseN), source: src, house: true}
	a.register(p)
	return p
}

func (a *Arena) applyLeave(beat int, id string) {
	p := a.players[id]
	kicked := p.kicked
	_ = a.match.leave(id, kicked) // queued only for bots that joined
	if p.detached && !kicked {
		p.inMatch, p.leaving, p.badgePending = false, false, false // out of the arena, but still a player
	} else {
		delete(a.players, id)
	}
	a.rec.Leaves = append(a.rec.Leaves, Leave{Beat: beat, Bot: id, Kicked: kicked})
}

// applyJoin tries to seat a queued player. With no free cell it reports
// false and the join stays queued for the next beat: nothing is recorded, and
// the player gets no leaderboard line until the bot is really in the arena.
func (a *Arena) applyJoin(beat int, id string) bool {
	p := a.players[id]
	spec := BrainSpec{Kind: KindProgram, Src: p.source}
	br, err := NewBrain(spec, a.cfg.Seed, id)
	if err == nil {
		err = a.match.Join(id, br)
	}
	if err != nil {
		return false
	}
	p.inMatch = true
	a.boardDirty = a.board.SetName(id, p.name) || a.boardDirty
	a.rec.Joins = append(a.rec.Joins, Join{Beat: beat, Bot: id, Brain: spec})
	return true
}

func (a *Arena) applyEdit(beat int, e pendingEdit) {
	p, ok := a.players[e.id]
	if !ok || !p.inMatch {
		return // the bot left on this same beat
	}
	n := a.cfg.RebootBeats
	p.badgePending = n > 0 // the pencil badge shows on the beat the bot is back
	if n > 0 {
		_ = a.match.Reboot(e.id, e.prog, n)
	} else {
		_ = a.match.Swap(e.id, e.prog) // every bot in the arena runs a program
	}
	a.rec.EditLog = append(a.rec.EditLog, Edit{Beat: beat, Bot: e.id, Brain: BrainSpec{Kind: KindProgram, Src: e.src}, Reboot: n})
}

// landBadges stamps the bots that are back on the map after a reboot: the
// pencil badge shows from the beat they return, however late that is.
func (a *Arena) landBadges(beat int) {
	for _, p := range a.players {
		if p.badgePending && onMap(a.match.World(), p.id) {
			p.swappedAt, p.badgePending = beat, false
		}
	}
}

func onMap(w sim.World, id string) bool {
	return slices.ContainsFunc(w.Bots, func(b sim.Bot) bool { return b.ID == id })
}
