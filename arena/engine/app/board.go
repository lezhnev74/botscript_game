package app

import (
	"cmp"
	"fmt"
	"maps"
	"slices"

	"github.com/lezhnev/arena/engine/domain/event"
)

// Points is the scoring table (VISION section 9). It is leaderboard config, not
// part of the event log or its hash.
type Points struct {
	Kill   int // a kill
	Repeat int // a kill of a victim this killer already killed before
	Coin   int // a coin
	Death  int // a death (negative)
}

// DefaultPoints are the VISION section 9 values.
func DefaultPoints() Points { return Points{Kill: 3, Repeat: 1, Coin: 1, Death: -1} }

// Entry is one bot's all-time line on the leaderboard.
//
// Score follows VISION section 9: a kill is +3, or +1 when this killer has
// killed that victim before (ever); a coin +1; a death -1 (DefaultPoints). The running total
// never drops below 0, applied event by event in log order. A self-kill or a
// kill by an ownerless bullet is a death without a kill. Kills, Deaths and
// Coins are plain counters; Prey counts kills per victim (it decides "repeat").
type Entry struct {
	Bot    string         `json:"bot"`
	Name   string         `json:"name,omitempty"`
	Score  int            `json:"score"`
	Kills  int            `json:"kills"`
	Deaths int            `json:"deaths"`
	Coins  int            `json:"coins"`
	Prey   map[string]int `json:"prey,omitempty"`
}

func (e Entry) clone() Entry {
	e.Prey = maps.Clone(e.Prey)
	return e
}

// addScore applies points to the running total, floored at 0.
func (e *Entry) addScore(points int) { e.Score = max(e.Score+points, 0) }

// Board is the leaderboard: a projection over the event log, folded one beat
// at a time with Apply. It is not safe for concurrent use.
type Board struct {
	entries map[string]Entry
	pts     Points
	removed map[string]bool // kicked bots: later events no longer give them a line
}

// NewBoard starts from stored entries (the Store's saved board).
func NewBoard(stored []Entry) *Board {
	b := &Board{entries: make(map[string]Entry, len(stored)), pts: DefaultPoints()}
	for _, e := range stored {
		b.entries[e.Bot] = e.clone()
	}
	return b
}

// WithPoints sets the scoring table (default DefaultPoints) and returns b.
func (b *Board) WithPoints(p Points) *Board {
	b.pts = p
	return b
}

// ProjectBoard folds a whole event log into a fresh board.
func ProjectBoard(log []event.Logged) *Board {
	b := NewBoard(nil)
	b.Apply(log)
	return b
}

// Apply folds events into the board and reports whether anything changed.
func (b *Board) Apply(log []event.Logged) bool {
	changed := false
	for _, l := range log {
		changed = b.apply(l.Event) || changed
	}
	return changed
}

func (b *Board) apply(ev event.Event) bool {
	switch e := ev.(type) {
	case event.Killed:
		b.update(e.Victim, func(x *Entry) { x.Deaths++; x.addScore(b.pts.Death) })
		if e.Killer != "" && e.Killer != e.Victim {
			b.update(e.Killer, func(x *Entry) { x.credit(e.Victim, b.pts) })
		}
	case event.Kicked:
		b.Remove(e.Bot)
	case event.CoinTaken:
		b.update(e.Bot, func(x *Entry) { x.Coins++; x.addScore(b.pts.Coin) })
	default:
		return false
	}
	return true
}

// credit books a kill of victim: +3 the first time, +1 for each repeat.
func (e *Entry) credit(victim string, p Points) {
	if e.Prey[victim] > 0 {
		e.addScore(p.Repeat)
	} else {
		e.addScore(p.Kill)
	}
	if e.Prey == nil {
		e.Prey = map[string]int{}
	}
	e.Prey[victim]++
	e.Kills++
}

func (b *Board) update(id string, f func(*Entry)) {
	if b.removed[id] {
		return
	}
	e := b.entries[id]
	e.Bot = id
	f(&e)
	b.entries[id] = e
}

// SetName records the display name of a bot (creating its line) and reports
// whether the board changed.
func (b *Board) SetName(id, name string) bool {
	e, known := b.entries[id]
	if known && e.Name == name {
		return false
	}
	b.update(id, func(e *Entry) { e.Name = name })
	return true
}

// Remove deletes the bot's line for good: events that still name it (a bullet
// in flight) do not bring it back.
func (b *Board) Remove(id string) {
	delete(b.entries, id)
	if b.removed == nil {
		b.removed = map[string]bool{}
	}
	b.removed[id] = true
}

// Reset wipes every line. Removed (kicked) bots stay removed.
func (b *Board) Reset() { clear(b.entries) }

// Known reports whether the bot has, or once had and lost, a line.
func (b *Board) Known(id string) bool { return b.Has(id) || b.removed[id] }

// Has reports whether the bot has a line on the board.
func (b *Board) Has(id string) bool {
	_, ok := b.entries[id]
	return ok
}

// Top lists every entry, best first: score, then kills (both descending),
// then bot id. The entries are copies.
func (b *Board) Top() []Entry {
	out := make([]Entry, 0, len(b.entries))
	for _, e := range b.entries {
		out = append(out, e.clone())
	}
	slices.SortFunc(out, func(x, y Entry) int {
		return cmp.Or(cmp.Compare(y.Score, x.Score), cmp.Compare(y.Kills, x.Kills), cmp.Compare(x.Bot, y.Bot))
	})
	return out
}

// CheckBoard verifies the leaderboard invariant: for every bot, the board's
// score, kills, deaths and coins equal those of a fresh projection of log. A bot
// missing on one side counts as zeros (joined bots start with no stats).
func CheckBoard(b *Board, log []event.Logged) error {
	want := NewBoard(nil).WithPoints(b.pts)
	want.Apply(log)
	ids := maps.Clone(b.entries)
	maps.Copy(ids, want.entries)
	for _, id := range slices.Sorted(maps.Keys(ids)) {
		if g, w := b.entries[id], want.entries[id]; g.stats() != w.stats() {
			return fmt.Errorf("leaderboard of %s is %v, the event log gives %v", id, g.stats(), w.stats())
		}
	}
	return nil
}

func (e Entry) stats() [4]int { return [4]int{e.Score, e.Kills, e.Deaths, e.Coins} }
