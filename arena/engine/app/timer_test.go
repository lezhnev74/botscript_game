package app_test

import (
	"errors"
	"reflect"
	"runtime"
	"slices"
	"testing"
	"time"

	"github.com/lezhnev/arena/engine/app"
	"github.com/lezhnev/arena/engine/domain/event"
)

func TestNoTimerMeansRemainingMinusOne(t *testing.T) {
	r := newRig(t, 0)
	r.join(t, "Ann")
	r.tick(t, 2)
	if u := r.feed.Last(); u.Remaining != -1 || u.GameOver {
		t.Errorf("remaining %d gameover %v", u.Remaining, u.GameOver)
	}
	wantState(t, r, true, -1)
}

func TestTimerCountsDownAndStopsTheGame(t *testing.T) {
	r := newRig(t, 0)
	r.join(t, "Ann")
	if err := r.a.SetTimer(3); err != nil {
		t.Fatal(err)
	}
	r.tick(t, 2)
	wantUpdate(t, r, 2, 1, false)
	r.tick(t, 1)
	wantUpdate(t, r, 3, 0, true)
	wantState(t, r, false, 0)
}

// is fails the test when got differs from want.
func is(t *testing.T, what string, got, want any) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("%s: got %+v, want %+v", what, got, want)
	}
}

func wantUpdate(t *testing.T, r *rig, beat, remaining int, over bool) {
	t.Helper()
	if u := r.feed.Last(); u.Beat != beat || u.Remaining != remaining || u.GameOver != over {
		t.Fatalf("update: beat %d remaining %d gameover %v, want %d %d %v", u.Beat, u.Remaining, u.GameOver, beat, remaining, over)
	}
}

func wantState(t *testing.T, r *rig, running bool, remaining int) {
	t.Helper()
	if gotRunning, gotRem := r.a.GameState(); gotRunning != running || gotRem != remaining {
		t.Errorf("state %v %d, want %v %d", gotRunning, gotRem, running, remaining)
	}
}

func TestCancelTimerKeepsTheGameRunning(t *testing.T) {
	r := newRig(t, 0)
	r.join(t, "Ann")
	if err := r.a.SetTimer(2); err != nil {
		t.Fatal(err)
	}
	r.tick(t, 1)
	if err := r.a.SetTimer(0); err != nil {
		t.Fatal(err)
	}
	r.tick(t, 5)
	if u := r.feed.Last(); u.GameOver || u.Remaining != -1 || u.Beat != 6 {
		t.Errorf("beat %d remaining %d gameover %v", u.Beat, u.Remaining, u.GameOver)
	}
}

func TestSetTimerRejectsNegativeAndStoppedGame(t *testing.T) {
	r := newRig(t, 0)
	if err := r.a.SetTimer(-1); !errors.Is(err, app.ErrBadTimer) {
		t.Errorf("negative: %v", err)
	}
	stop(t, r)
	if err := r.a.SetTimer(5); !errors.Is(err, app.ErrNotRunning) {
		t.Errorf("stopped: %v", err)
	}
}

// stop runs a one-beat timer game to its end.
func stop(t *testing.T, r *rig) {
	t.Helper()
	if err := r.a.SetTimer(1); err != nil {
		t.Fatal(err)
	}
	r.tick(t, 1)
}

func TestNoBeatsPlayWhileStopped(t *testing.T) {
	r := newRig(t, 0)
	r.join(t, "Ann")
	r.tick(t, 1)
	stop(t, r)
	n, beat := len(r.feed.Updates), r.a.Beat()
	r.tick(t, 5)
	if len(r.feed.Updates) != n || r.a.Beat() != beat {
		t.Errorf("updates %d -> %d, beat %d -> %d", n, len(r.feed.Updates), beat, r.a.Beat())
	}
}

func TestStoppedGameLeavesNoBotGoroutines(t *testing.T) {
	r := newRig(t, 3)
	r.tick(t, 2)
	before := runtime.NumGoroutine()
	stop(t, r)
	settled := false
	for i := 0; i < 1000 && !settled; i++ {
		settled = runtime.NumGoroutine() < before
		time.Sleep(time.Millisecond)
	}
	if !settled {
		t.Errorf("bot goroutines still running: %d before stop, %d after", before, runtime.NumGoroutine())
	}
}

func TestGameOverSavesTheLastBoardAndReplacesTheOldOne(t *testing.T) {
	r := newRig(t, 0)
	r.join(t, "Ann")
	r.tick(t, 1)
	is(t, "last board before any game ends", len(r.a.LastBoard()), 0)
	stop(t, r)
	first := r.a.LastBoard()
	is(t, "names", names(first), []string{"Ann"})
	is(t, "stored", r.store.Last, first)
	if err := r.a.StartGame(); err != nil {
		t.Fatal(err)
	}
	r.join(t, "Bob")
	r.tick(t, 1)
	stop(t, r)
	is(t, "second game board", names(r.a.LastBoard()), []string{"Ann", "Bob"})
	is(t, "stored second", r.store.Last, r.a.LastBoard())
}

func names(es []app.Entry) []string {
	out := []string{}
	for _, e := range es {
		out = append(out, e.Name)
	}
	slices.Sort(out)
	return out
}

func TestLastBoardIsLoadedAtStartAndCopied(t *testing.T) {
	r := newUnstartedRig(0, room)
	r.store.Last = []app.Entry{{Bot: "x", Name: "Old", Score: 7}}
	r.start(t)
	got := r.a.LastBoard()
	if len(got) != 1 || got[0].Name != "Old" {
		t.Fatalf("loaded %+v", got)
	}
	got[0].Name = "changed"
	if r.a.LastBoard()[0].Name != "Old" {
		t.Error("LastBoard exposes internal state")
	}
}

func TestStartGameNeedsAStoppedGame(t *testing.T) {
	r := newRig(t, 0)
	if err := r.a.StartGame(); !errors.Is(err, app.ErrRunning) {
		t.Errorf("got %v", err)
	}
}

func TestStartGameGivesAnEmptyBoardAndKeepsPlayers(t *testing.T) {
	r := newRig(t, 0)
	token, id := r.join(t, "Ann")
	if err := r.a.PutBot(token, walker); err != nil {
		t.Fatal(err)
	}
	r.tick(t, 5)
	stop(t, r)
	if err := r.a.StartGame(); err != nil {
		t.Fatal(err)
	}
	wantState(t, r, true, -1)
	is(t, "board right after start", len(r.a.Leaderboard()), 0)
	r.tick(t, 1)
	wantUpdate(t, r, 1, -1, false)
	v, err := r.a.GetBot(token)
	is(t, "roster", r.roster(), id)
	is(t, "bot source", v.Source, walker)
	is(t, "bot error", err, nil)
	is(t, "board after a beat", []int{len(r.a.Leaderboard()), r.a.Leaderboard()[0].Score}, []int{1, 0})
	is(t, "seed", r.a.Record().Seed, uint64(12))
}

func TestStartGameStartsOnAPendingMap(t *testing.T) {
	r := newRig(t, 0)
	r.join(t, "Ann")
	stop(t, r)
	if err := r.a.ApplyMap(bigRoom); err != nil {
		t.Fatal(err)
	}
	r.tick(t, 3)
	if w, _ := r.a.MapSize(); w != 9 {
		t.Fatalf("map changed while stopped: %d", w)
	}
	if err := r.a.StartGame(); err != nil {
		t.Fatal(err)
	}
	r.tick(t, 1)
	if w, h := r.a.MapSize(); w != 11 || h != 9 {
		t.Errorf("size %dx%d", w, h)
	}
}

func TestCommandsWhileStoppedApplyOnStart(t *testing.T) {
	r := newRig(t, 0)
	ann, annID := r.join(t, "Ann")
	bobToken, bobID := r.join(t, "Bob")
	r.tick(t, 3)
	stop(t, r)

	_, cyID := r.join(t, "Cy")
	for _, err := range []error{r.a.PutBot(ann, walker), r.a.Kick(bobID), r.a.SetMinBots(2)} {
		is(t, "command while stopped", err, nil)
	}
	r.tick(t, 2)
	is(t, "roster while stopped", r.roster(), annID+","+bobID)

	is(t, "start", r.a.StartGame(), nil)
	r.tick(t, 1)
	_, kickedErr := r.a.GetBot(bobToken)
	edited, _ := r.a.GetBot(ann)
	is(t, "roster after start", r.roster(), annID+","+cyID)
	is(t, "kicked token", errors.Is(kickedErr, app.ErrUnauthorized), true)
	is(t, "edit", edited.Source, walker)
}

func TestInspectWhileStoppedReportsNothing(t *testing.T) {
	r := newRig(t, 0)
	token, _ := r.join(t, "Ann")
	r.tick(t, 2)
	stop(t, r)
	if st, err := r.a.Inspect(token); err != nil || st.InArena {
		t.Errorf("inspect %+v err %v", st, err)
	}
}

func TestLastBoardSaveFailureIsRetried(t *testing.T) {
	r := newRig(t, 0)
	r.join(t, "Ann")
	r.tick(t, 1)
	r.store.Err = errors.New("disk full")
	if err := r.a.SetTimer(1); err != nil {
		t.Fatal(err)
	}
	if err := r.a.Tick(); err == nil {
		t.Fatal("want the save error")
	}
	r.store.Err = nil
	r.tick(t, 1) // the stopped arena still retries its saves
	if len(r.store.Last) != 1 {
		t.Errorf("stored %+v", r.store.Last)
	}
}

func TestEditsWhileStoppedAreNotRateLimitedAndTheLastWins(t *testing.T) {
	r := newRig(t, 0)
	ann, _ := r.join(t, "Ann")
	r.tick(t, 3)
	stop(t, r)
	for _, src := range []string{walker, firer, walker, firer} {
		is(t, "edit while stopped", r.a.PutBot(ann, src), nil)
	}
	is(t, "start", r.a.StartGame(), nil)
	r.tick(t, 1)
	b, _ := r.a.GetBot(ann)
	is(t, "last edit", b.Source, firer)
}

func TestResetBoardWhileStoppedIsRejected(t *testing.T) {
	r := newRig(t, 0)
	r.join(t, "Ann")
	r.tick(t, 1)
	stop(t, r)
	before := r.a.Leaderboard()
	is(t, "reset", errors.Is(r.a.ResetBoard(), app.ErrNotRunning), true)
	is(t, "board untouched", r.a.Leaderboard(), before)
}

// playLogged ticks n beats and appends the published events to log.
func playLogged(t *testing.T, r *rig, log []event.Logged, n int) []event.Logged {
	t.Helper()
	for range n {
		r.tick(t, 1)
		log = append(log, r.feed.Last().Events...)
	}
	return log
}

func TestTimedGamesReplayToTheirLiveLogAndMatchTheLastBoard(t *testing.T) {
	r := newRig(t, 3)
	ann, _ := r.join(t, "Ann")
	is(t, "timer", r.a.SetTimer(40), nil)
	log1 := playLogged(t, r, nil, 40)
	rec1 := r.a.Record()
	checkReplayHash(t, "game 1", rec1, log1)

	is(t, "edit while stopped", r.a.PutBot(ann, firer), nil)
	r.join(t, "Bob")
	is(t, "start", r.a.StartGame(), nil)
	is(t, "timer 2", r.a.SetTimer(60), nil)
	log2 := playLogged(t, r, nil, 60)
	rec2 := r.a.Record()
	checkReplayHash(t, "game 2", rec2, log2)

	replayed, err := app.Replay(rec2)
	if err != nil {
		t.Fatal(err)
	}
	if err := app.CheckBoard(app.NewBoard(r.a.LastBoard()), replayed); err != nil {
		t.Error(err)
	}
}

func checkReplayHash(t *testing.T, what string, rec app.Record, live []event.Logged) {
	t.Helper()
	replayed, err := app.Replay(rec)
	if err != nil {
		t.Fatalf("%s: %v", what, err)
	}
	want, _ := app.HashLog(live)
	if got, _ := app.HashLog(replayed); got != want {
		t.Errorf("%s: replay hash %s, live hash %s", what, got, want)
	}
}
