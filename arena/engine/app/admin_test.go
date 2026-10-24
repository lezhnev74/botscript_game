package app_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/lezhnev/arena/engine/app"
)

func scored(es []app.Entry) bool {
	for _, e := range es {
		if e.Score != 0 || e.Kills != 0 || e.Deaths != 0 || e.Coins != 0 {
			return true
		}
	}
	return false
}

// tame reports whether no line of the board holds more than what one beat after a wipe can add.
func tame(es []app.Entry) bool {
	for _, e := range es {
		if e.Score > 3 || e.Kills > 1 {
			return false
		}
	}
	return true
}

func TestResetBoardWipesScoresKeepsNamesAndIsSaved(t *testing.T) {
	r := newRig(t, 4)
	_, id := r.join(t, "Ann")
	r.tick(t, 300)
	if !scored(r.a.Leaderboard()) {
		t.Fatal("nobody scored in 300 beats; the test needs a played board")
	}
	r.a.ResetBoard()
	if scored(r.a.Leaderboard()) || !boardHas(r.a.Leaderboard(), id) {
		t.Errorf("live board after the wipe: %+v", r.a.Leaderboard())
	}
	r.tick(t, 1)
	if !tame(r.feed.Last().Board) {
		t.Errorf("broadcast board not reset: %+v", r.feed.Last().Board)
	}
	r.a.Close()
	r.start(t)
	if !tame(r.store.Board) {
		t.Errorf("wipe not persisted: %+v", r.store.Board)
	}
}

func TestResetBoardPersistedWithoutOtherChanges(t *testing.T) {
	r := newRig(t, 0)
	r.join(t, "Ann")
	r.tick(t, 1)
	r.store.Board = append(r.store.Board, app.Entry{Bot: "old", Score: 9})
	r.a.ResetBoard()
	r.tick(t, 1)
	if boardHas(r.store.Board, "old") {
		t.Errorf("store still has %+v", r.store.Board)
	}
}

func TestSetMinBotsUpAndDown(t *testing.T) {
	r := newRig(t, 2)
	r.tick(t, 1)
	if got := r.roster(); got != "house-1,house-2" {
		t.Fatalf("start: %q", got)
	}
	if err := r.a.SetMinBots(4); err != nil {
		t.Fatal(err)
	}
	if r.a.MinBots() != 4 {
		t.Errorf("MinBots %d", r.a.MinBots())
	}
	r.tick(t, 1)
	if n := len(strings.Split(r.roster(), ",")); n != 4 {
		t.Errorf("up: %d bots", n)
	}
	if err := r.a.SetMinBots(0); err != nil {
		t.Fatal(err)
	}
	r.tick(t, 1)
	if got := r.roster(); got != "" {
		t.Errorf("down: %q", got)
	}
}

func TestSetMinBotsLeavesHumansAlone(t *testing.T) {
	r := newRig(t, 3)
	_, id := r.join(t, "Ann")
	r.tick(t, 1)
	if err := r.a.SetMinBots(0); err != nil {
		t.Fatal(err)
	}
	r.tick(t, 1)
	if got := r.roster(); got != id {
		t.Errorf("roster %q, want only %q", got, id)
	}
}

func TestSetMinBotsRejectsOutOfRange(t *testing.T) {
	r := newRig(t, 2)
	for _, n := range []int{-1, 36} {
		if err := r.a.SetMinBots(n); !errors.Is(err, app.ErrMinBots) {
			t.Errorf("%d: %v", n, err)
		}
	}
	if r.a.MinBots() != 2 {
		t.Errorf("MinBots changed to %d", r.a.MinBots())
	}
}
