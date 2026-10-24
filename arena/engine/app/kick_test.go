package app_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/lezhnev/arena/engine/app"
	"github.com/lezhnev/arena/engine/app/apptest"
	"github.com/lezhnev/arena/engine/domain/event"
)

func TestKickRemovesAHumanAtTheNextBeatAndRevokesTheToken(t *testing.T) {
	r := newRig(t, 2)
	token, id := r.join(t, "Ann")
	r.tick(t, 2)
	if err := r.a.Kick(id); err != nil {
		t.Fatal(err)
	}
	if _, err := r.a.GetBot(token); !errors.Is(err, app.ErrUnauthorized) {
		t.Errorf("token after kick: %v", err)
	}
	r.tick(t, 1)
	rec := r.a.Record()
	if len(rec.Leaves) < 1 || rec.Leaves[len(rec.Leaves)-1].Bot != id {
		t.Errorf("leaves %v", rec.Leaves)
	}
	if got := r.roster(); got != "house-1,house-2" && got != "house-2,house-3" {
		t.Errorf("a house bot should refill: %q", got)
	}
}

func TestKickErrors(t *testing.T) {
	r := newRig(t, 2)
	_, id := r.join(t, "Ann")
	r.tick(t, 1)
	if err := r.a.Kick("nobody"); !errors.Is(err, app.ErrUnknownBot) {
		t.Errorf("unknown: %v", err)
	}
	if err := r.a.Kick("house-1"); !errors.Is(err, app.ErrHouseBot) {
		t.Errorf("house: %v", err)
	}
	if err := r.a.Kick(id); err != nil {
		t.Fatal(err)
	}
	if err := r.a.Kick(id); !errors.Is(err, app.ErrUnknownBot) {
		t.Errorf("twice: %v", err)
	}
}

func TestKickBeforeTheJoinBeatCancelsTheJoin(t *testing.T) {
	r := newRig(t, 0)
	_, id := r.join(t, "Ann")
	if err := r.a.Kick(id); err != nil {
		t.Fatal(err)
	}
	r.tick(t, 1)
	if rec := r.a.Record(); len(rec.Joins)+len(rec.Leaves) != 0 {
		t.Errorf("%+v", rec)
	}
}

func TestKickDeletesTheBoardLineAndItSurvivesARestart(t *testing.T) {
	r := newRig(t, 0)
	_, id := r.join(t, "Ann")
	r.join(t, "Bob")
	r.tick(t, 2)
	if err := r.a.Kick(id); err != nil {
		t.Fatal(err)
	}
	r.tick(t, 1)
	if boardHas(r.a.Leaderboard(), id) {
		t.Error("live: line of the kicked bot is back")
	}
	r.a.Close()
	r.start(t)
	if boardHas(r.a.Leaderboard(), id) || boardHas(r.store.Board, id) {
		t.Error("restart: line of the kicked bot is back")
	}
	for _, b := range r.store.Bots {
		if b.ID == id {
			t.Errorf("saved bots still have %+v", b)
		}
	}
}

func boardHas(board []app.Entry, id string) bool {
	return slices.ContainsFunc(board, func(e app.Entry) bool { return e.Bot == id })
}

func TestBotsListsEveryBotOldestFirstWithScores(t *testing.T) {
	r := newRig(t, 2)
	r.tick(t, 1)
	r.join(t, "Ann")
	got := r.a.Bots()
	if len(got) != 3 || got[0].ID != "house-1" || !got[0].House || got[2].ID != "p1" || got[2].House || got[2].Name != "Ann" {
		t.Errorf("%+v", got)
	}
}

func TestReplayOfASessionWithAKickGivesTheSameHash(t *testing.T) {
	r := newRig(t, 3)
	var live []event.Logged
	step := func(n int) {
		for range n {
			r.tick(t, 1)
			live = append(live, r.feed.Last().Events...)
		}
	}
	_, id := r.join(t, "Ann")
	step(20)
	if err := r.a.Kick(id); err != nil {
		t.Fatal(err)
	}
	step(40)
	rec := r.a.Record()
	replayed, err := app.Replay(rec)
	if err != nil {
		t.Fatal(err)
	}
	want, _ := app.HashLog(live)
	if got, _ := app.HashLog(replayed); got != want || len(rec.Leaves) == 0 {
		t.Errorf("hash %s, live %s, leaves %v", got, want, rec.Leaves)
	}
}

func TestLeaderboardAfterAKickIsTheProjectionOfTheEventLog(t *testing.T) {
	r := newRig(t, 3)
	var live []event.Logged
	step := func(n int) {
		for range n {
			r.tick(t, 1)
			live = append(live, r.feed.Last().Events...)
		}
	}
	_, id := r.join(t, "Ann")
	step(200)
	if err := r.a.Kick(id); err != nil {
		t.Fatal(err)
	}
	step(5)
	replayed, err := app.Replay(r.a.Record())
	if err != nil {
		t.Fatal(err)
	}
	if err := app.CheckBoard(app.NewBoard(r.a.Leaderboard()), replayed); err != nil {
		t.Error(err)
	}
	if boardHas(app.ProjectBoard(replayed).Top(), id) || !slices.Contains(live, event.Logged{Beat: r.a.Record().Leaves[len(r.a.Record().Leaves)-1].Beat, Event: event.Kicked{Bot: id}}) {
		t.Errorf("projection keeps the kicked bot or the log has no Kicked")
	}
}

func TestBoardKickedEventDeletesTheLineAndIgnoresLaterEvents(t *testing.T) {
	log := []event.Logged{
		{Beat: 1, Event: event.Killed{Killer: "a", Victim: "b"}},
		{Beat: 2, Event: event.Kicked{Bot: "b"}},
		{Beat: 3, Event: event.Killed{Killer: "a", Victim: "b"}},
		{Beat: 3, Event: event.CoinTaken{Bot: "b"}},
	}
	b := app.ProjectBoard(log)
	if boardHas(b.Top(), "b") || !boardHas(b.Top(), "a") || !b.Known("b") {
		t.Errorf("%+v", b.Top())
	}
}

// replayIDs hands out p1 twice, then p2: a generator that repeats an id.
type replayIDs struct {
	apptest.Seq
	n int
}

func (f *replayIDs) BotID() string {
	f.n++
	return []string{"p1", "p1", "p2"}[min(f.n, 3)-1]
}

func TestKickedIDIsNotDrawnAgain(t *testing.T) {
	r := newUnstartedRig(0, room)
	ids := &replayIDs{}
	a, err := app.NewArena(r.cfg, app.Deps{Store: r.store, Broadcaster: r.feed, IDs: ids})
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	_, id, _ := a.Join("Ann")
	_ = a.Tick()
	_ = a.Tick()
	if err := a.Kick(id); err != nil {
		t.Fatal(err)
	}
	_ = a.Tick()
	if _, got, err := a.Join("Bob"); err != nil || got != "p2" {
		t.Errorf("kicked id %q drawn again: got %q, %v", id, got, err)
	}
}
