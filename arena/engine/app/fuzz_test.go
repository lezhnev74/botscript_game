package app

import (
	"errors"
	"testing"

	"github.com/lezhnev/arena/engine/domain/event"
)

// playChecked runs the record beat by beat, checking the invariants after
// every beat, and returns the log hash.
func playChecked(t *testing.T, rec Record) string {
	t.Helper()
	m, err := rec.NewMatch()
	if err != nil {
		t.Fatalf("record does not build: %v\n%s", err, rec.Map)
	}
	defer m.Close()
	var log []event.Logged
	board := NewBoard(nil)
	for beat := 1; beat <= rec.Beats; beat++ {
		evs := m.Step()
		if err := m.Err(); err != nil {
			t.Fatalf("beat %d: %v\nrecord: %+v", beat, err, rec)
		}
		log = append(log, evs...)
		board.Apply(evs)
		if err := errors.Join(CheckInvariants(m.World(), rec.Rules, log), CheckBoard(board, log)); err != nil {
			t.Fatalf("beat %d: %v\nmap:\n%s", beat, err, rec.Map)
		}
	}
	checkBoardTotals(t, board, MatchStats(log, rec.Beats))
	h, err := HashLog(log)
	if err != nil {
		t.Fatal(err)
	}
	return h
}

// checkBoardTotals ties the board to Stats, which counts the log on its own:
// board kills == kills, board deaths == deaths (kills + self-kills + unowned).
func checkBoardTotals(t *testing.T, b *Board, s Stats) {
	t.Helper()
	var kills, deaths, coins int
	for _, e := range b.Top() {
		kills, deaths, coins = kills+e.Kills, deaths+e.Deaths, coins+e.Coins
	}
	if kills != s.Kills || deaths != s.Deaths || coins != s.Coins {
		t.Fatalf("board totals %d/%d/%d, stats %d/%d/%d", kills, deaths, coins, s.Kills, s.Deaths, s.Coins)
	}
}

// FuzzMatch is proof 2: random small matches run twice give one hash (also
// through Replay) and keep every invariant, including leaderboard = projection
// of the event log (CheckBoard). Bots are scripts or house programs, with random
// hot-swaps.
// Seed corpus: testdata/fuzz/FuzzMatch.
func FuzzMatch(f *testing.F) {
	f.Add(uint64(1), uint8(5), uint8(5), uint8(2), uint8(40))
	f.Add(uint64(2), uint8(9), uint8(9), uint8(4), uint8(120))
	f.Add(uint64(3), uint8(7), uint8(6), uint8(3), uint8(80))
	f.Fuzz(func(t *testing.T, seed uint64, w, h, bots, beats uint8) {
		rec := RandomRecord(seed, RandomParams{W: int(w), H: int(h), Bots: int(bots), Beats: int(beats)})
		first, second := playChecked(t, rec), playChecked(t, rec)
		if first != second {
			t.Fatalf("run twice differs: %s vs %s\nrecord: %+v", first, second, rec)
		}
		log, err := Replay(rec)
		if err != nil {
			t.Fatal(err)
		}
		if replayed, _ := HashLog(log); replayed != first {
			t.Fatalf("replay differs: %s vs %s", replayed, first)
		}
	})
}
