package app_test

import (
	"encoding/json"
	"testing"

	"github.com/lezhnev/arena/engine/app"
	"github.com/lezhnev/arena/engine/domain/event"
)

const firer = "function loop() { fire(E); move(pick(N, S)); }"
const walker = "function loop() { move(pick(N, E, S, W)); }"

// playSession runs a scripted live session: joins, edits, leaves and house
// filler, and returns the whole event log the arena published.
func playSession(t *testing.T, r *rig) []event.Logged {
	t.Helper()
	var log []event.Logged
	step := func(n int) {
		for range n {
			r.tick(t, 1)
			log = append(log, r.feed.Last().Events...)
		}
	}
	step(5)
	ann, _ := r.join(t, "Ann")
	bob, _ := r.join(t, "Bob")
	step(4)
	if err := r.a.PutBot(ann, firer); err != nil {
		t.Fatal(err)
	}
	step(3)
	if err := r.a.PutBot(bob, walker); err != nil {
		t.Fatal(err)
	}
	_ = r.a.Leave(ann)
	step(30)
	r.join(t, "Cy")
	step(60)
	_ = r.a.Leave(bob)
	step(120)
	return log
}

func TestTheSessionIsRichEnough(t *testing.T) {
	r := newRig(t, 3)
	playSession(t, r)
	rec := r.a.Record()
	if len(rec.Joins) < 5 || len(rec.Leaves) < 2 || len(rec.EditLog) < 2 || rec.Beats != 222 {
		t.Errorf("%d joins, %d leaves, %d edits, %d beats", len(rec.Joins), len(rec.Leaves), len(rec.EditLog), rec.Beats)
	}
}

func TestReplayOfALiveSessionGivesTheSameHash(t *testing.T) {
	r := newRig(t, 3)
	live := playSession(t, r)
	want, err := app.HashLog(live)
	if err != nil {
		t.Fatal(err)
	}
	replayed, err := app.Replay(r.a.Record())
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := app.HashLog(replayed); got != want {
		t.Errorf("replay hash %s, live hash %s", got, want)
	}
}

func TestLiveRecordSurvivesJSONAndStillReplays(t *testing.T) {
	r := newRig(t, 3)
	live := playSession(t, r)
	data, err := json.Marshal(r.a.Record())
	if err != nil {
		t.Fatal(err)
	}
	rec, err := app.DecodeRecord(data)
	if err != nil {
		t.Fatal(err)
	}
	replayed, err := app.Replay(rec)
	if err != nil {
		t.Fatal(err)
	}
	want, _ := app.HashLog(live)
	if got, _ := app.HashLog(replayed); got != want {
		t.Errorf("hash %s, want %s", got, want)
	}
}

func TestLiveBoardEqualsTheProjectionOfTheReplayedLog(t *testing.T) {
	r := newRig(t, 3)
	playSession(t, r)
	replayed, err := app.Replay(r.a.Record())
	if err != nil {
		t.Fatal(err)
	}
	board := app.NewBoard(r.a.Leaderboard())
	if err := app.CheckBoard(board, replayed); err != nil {
		t.Error(err)
	}
}
