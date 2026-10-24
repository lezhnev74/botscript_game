package app

import (
	"slices"
	"strings"

	"github.com/lezhnev/arena/engine/domain/event"
)

// NewUpdate assembles what is published after a beat from the match as it
// stands. edited maps a bot to the beat of its latest hot-swap (bots never
// edited are absent); it becomes the "<bot>.edited" probe. The Arena and the
// replay exporter both build their Updates here, so a fixture is exactly what
// a live session would have sent.
func NewUpdate(m *Match, evs []event.Logged, roster []Player, edited map[string]int, board []Entry) Update {
	w := m.World()
	probes := m.Probes()
	for id, beat := range edited {
		probes[id+".edited"] = beat
	}
	SortPlayers(roster)
	return Update{Beat: w.Beat, World: w, Events: evs, Roster: roster, Board: board, Remaining: -1, Probes: probes}
}

// SortPlayers orders a roster by bot id.
func SortPlayers(ps []Player) {
	slices.SortFunc(ps, func(x, y Player) int { return strings.Compare(x.ID, y.ID) })
}
