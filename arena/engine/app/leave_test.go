package app_test

import (
	"errors"
	"testing"

	"github.com/lezhnev/arena/engine/app"
	"github.com/lezhnev/arena/engine/domain/event"
)

const idler = "function loop() { wait(); }"

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func (r *rig) bot(t *testing.T, token string) app.BotView {
	t.Helper()
	v, err := r.a.GetBot(token)
	must(t, err)
	return v
}

// wantBot checks the arena state GetBot reports.
func (r *rig) wantBot(t *testing.T, token string, in bool, leftBy string, stayLeft int) {
	t.Helper()
	v := r.bot(t, token)
	if v.InArena != in || v.LeftBy != leftBy || v.StayLeft != stayLeft {
		t.Errorf("in %v leftBy %q stayLeft %d, want %v %q %d", v.InArena, v.LeftBy, v.StayLeft, in, leftBy, stayLeft)
	}
}

func (r *rig) wantRoster(t *testing.T, want string) {
	t.Helper()
	if got := r.roster(); got != want {
		t.Errorf("roster %q, want %q", got, want)
	}
}

// wantCounts checks how many joins, leaves and edits the live record holds.
func (r *rig) wantCounts(t *testing.T, joins, leaves, edits int) {
	t.Helper()
	rec := r.a.Record()
	if len(rec.Joins) != joins || len(rec.Leaves) != leaves || len(rec.EditLog) != edits {
		t.Errorf("joins/leaves/edits %d/%d/%d, want %d/%d/%d", len(rec.Joins), len(rec.Leaves), len(rec.EditLog), joins, leaves, edits)
	}
}

func retryBeats(err error) int {
	var rl *app.RateLimitError
	if errors.As(err, &rl) {
		return rl.RetryBeats
	}
	return -1
}

func TestLeaveDetachesAndKeepsTokenProgramLookAndBoardLine(t *testing.T) {
	r := newRig(t, 0)
	token, id := r.join(t, "Ann")
	r.tick(t, 4)
	look := app.DefaultLook("x")
	must(t, r.a.SetProfile(token, "Ann", look))
	must(t, r.a.PutBot(token, idler))
	must(t, r.a.Leave(token))
	r.tick(t, 1)
	r.wantBot(t, token, false, app.LeftByPlayer, -1)
	if v := r.bot(t, token); v.Source != idler || v.Look != look || v.Name != "Ann" {
		t.Errorf("%+v", v)
	}
	r.wantRoster(t, "")
	if !boardHas(r.a.Leaderboard(), id) {
		t.Error("board line gone")
	}
}

func TestLeaveIsRecordedAndSavedAsDetached(t *testing.T) {
	r := newRig(t, 0)
	token, id := r.join(t, "Ann")
	r.tick(t, 4)
	must(t, r.a.Leave(token))
	r.tick(t, 1)
	r.wantCounts(t, 1, 1, 0)
	if leaves := r.a.Record().Leaves; leaves[0].Bot != id || leaves[0].Kicked {
		t.Errorf("leaves %v", leaves)
	}
	if saved := r.store.Bots; len(saved) != 1 || !saved[0].Detached || saved[0].LeftBy != app.LeftByPlayer {
		t.Errorf("saved %+v", saved)
	}
}

func TestLeavingADetachedBotAndReturningAnArenaBotAreNoOps(t *testing.T) {
	r := newRig(t, 0)
	token, _ := r.join(t, "Ann")
	r.tick(t, 1)
	must(t, r.a.Return(token))
	_ = r.a.Leave(token)
	r.tick(t, 1)
	must(t, r.a.Leave(token))
	r.tick(t, 1)
	r.wantCounts(t, 1, 1, 0)
	if err := r.a.Leave("nope"); !errors.Is(err, app.ErrUnauthorized) {
		t.Errorf("leave: %v", err)
	}
	if err := r.a.Return("nope"); !errors.Is(err, app.ErrUnauthorized) {
		t.Errorf("return: %v", err)
	}
}

func TestLeaveBeforeTheJoinBeatCancelsTheJoin(t *testing.T) {
	r := newRig(t, 0)
	token, _ := r.join(t, "Ann")
	must(t, r.a.Leave(token))
	r.tick(t, 1)
	r.wantCounts(t, 0, 0, 0)
	r.wantBot(t, token, false, app.LeftByPlayer, -1)
}

func capRig(t *testing.T, n int) *rig {
	t.Helper()
	r := newUnstartedRig(0, room)
	r.cfg.MaxPlayers = n
	r.start(t)
	return r
}

func TestDetachedBotsDoNotCountForThePlayerCap(t *testing.T) {
	r := capRig(t, 1)
	ann, _ := r.join(t, "Ann")
	r.tick(t, 1)
	if _, _, err := r.a.Join("Bob"); !errors.Is(err, app.ErrFull) {
		t.Fatalf("full: %v", err)
	}
	_ = r.a.Leave(ann)
	if _, _, err := r.a.Join("Bob"); err != nil {
		t.Errorf("after leave: %v", err)
	}
}

func TestReturnJoinsAtTheNextBeatWithTheSavedProgram(t *testing.T) {
	r := newRig(t, 0)
	token, id := r.join(t, "Ann")
	r.tick(t, 4)
	_ = r.a.PutBot(token, idler)
	_ = r.a.Leave(token)
	r.tick(t, 3)
	must(t, r.a.Return(token))
	r.wantBot(t, token, true, "", -1)
	r.tick(t, 1)
	r.wantRoster(t, id)
	joins := r.a.Record().Joins
	last := joins[len(joins)-1]
	if len(joins) != 2 || last.Bot != id || last.Brain.Src != idler || last.Beat != 8 {
		t.Errorf("joins %+v", joins)
	}
}

func TestReturnToAFullArenaIsRefused(t *testing.T) {
	r := capRig(t, 1)
	ann, _ := r.join(t, "Ann")
	r.tick(t, 1)
	_ = r.a.Leave(ann)
	r.tick(t, 1)
	r.join(t, "Bob")
	r.tick(t, 3)
	if err := r.a.Return(ann); !errors.Is(err, app.ErrFull) {
		t.Errorf("%v", err)
	}
	r.wantBot(t, ann, false, app.LeftByPlayer, -1)
}

func TestLeaveAndReturnAreLimitedLikeASwap(t *testing.T) {
	r := newRig(t, 0)
	token, _ := r.join(t, "Ann")
	r.tick(t, 5)
	_ = r.a.Leave(token)
	if got := retryBeats(r.a.Return(token)); got != 3 {
		t.Fatalf("retry %d", got)
	}
	r.tick(t, 2)
	if got := retryBeats(r.a.Return(token)); got != 1 {
		t.Fatalf("retry %d", got)
	}
	r.tick(t, 1)
	must(t, r.a.Return(token))
	if err := r.a.Leave(token); !errors.Is(err, app.ErrRateLimited) {
		t.Errorf("leave right after return: %v", err)
	}
}

func stoppedRig(t *testing.T) (*rig, string, string) {
	t.Helper()
	r := newRig(t, 0)
	token, id := r.join(t, "Ann")
	r.tick(t, 2)
	must(t, r.a.SetTimer(1))
	r.tick(t, 1)
	return r, token, id
}

func TestLeaveAndReturnAreNotLimitedWhileTheGameIsStopped(t *testing.T) {
	r, token, id := stoppedRig(t)
	for range 2 {
		must(t, r.a.Leave(token))
		must(t, r.a.Return(token))
	}
	_ = r.a.Leave(token)
	must(t, r.a.StartGame())
	r.tick(t, 1)
	r.wantBot(t, token, false, app.LeftByPlayer, -1)
	r.wantRoster(t, "")
	must(t, r.a.Return(token))
	r.tick(t, 1)
	r.wantRoster(t, id)
}

func TestReturnBeforeTheLeaveLandsOnAStoppedGameCancelsTheLeave(t *testing.T) {
	r, token, id := stoppedRig(t)
	_ = r.a.Leave(token)
	must(t, r.a.Return(token))
	must(t, r.a.StartGame())
	r.tick(t, 1)
	r.wantRoster(t, id)
	r.wantBot(t, token, true, "", -1)
}

func stayRig(t *testing.T, minBots, maxStay int) *rig {
	t.Helper()
	r := newUnstartedRig(minBots, room)
	r.cfg.MaxStay = maxStay
	r.start(t)
	return r
}

func TestAHumanLeavesByItselfAfterExactlyMaxStayBeatsInTheMatch(t *testing.T) {
	r := stayRig(t, 0, 10)
	token, id := r.join(t, "Ann")
	r.tick(t, 1) // beat 1: joined, stay 1 of 10
	r.wantBot(t, token, true, "", 9)
	r.tick(t, 8) // beats 2..9
	r.wantRoster(t, id)
	r.tick(t, 1) // beat 10 is the tenth beat in the match
	r.wantBot(t, token, false, app.LeftByTime, -1)
	r.tick(t, 1) // beat 11: the leave lands
	r.wantRoster(t, "")
	if leaves := r.a.Record().Leaves; len(leaves) != 1 || leaves[0].Beat != 11 {
		t.Errorf("leaves %+v", leaves)
	}
}

func TestAnAutoLeftBotReturnsAtOnceWithAFreshCount(t *testing.T) {
	r := stayRig(t, 0, 5)
	token, _ := r.join(t, "Ann")
	r.tick(t, 5)
	must(t, r.a.Return(token)) // the automatic leave has not landed yet: it is cancelled
	r.tick(t, 1)
	r.wantBot(t, token, true, "", 4)
	r.tick(t, 4)
	r.wantBot(t, token, false, app.LeftByTime, -1)
}

func TestAnAutoLeftBotThatReturnsAfterTheLeaveLandedGetsANewCount(t *testing.T) {
	r := stayRig(t, 0, 5)
	token, id := r.join(t, "Ann")
	r.tick(t, 6)
	r.wantRoster(t, "")
	must(t, r.a.Return(token))
	r.tick(t, 1)
	r.wantRoster(t, id)
	r.wantBot(t, token, true, "", 4)
}

func TestNoStayLimitMeansStayLeftIsMinusOne(t *testing.T) {
	r := newRig(t, 0)
	token, _ := r.join(t, "Ann")
	r.tick(t, 50)
	r.wantBot(t, token, true, "", -1)
}

func TestAStoppedGameDoesNotUseUpAStay(t *testing.T) {
	r := stayRig(t, 0, 10)
	token, _ := r.join(t, "Ann")
	_ = r.a.SetTimer(3)
	r.tick(t, 3)
	r.tick(t, 20)
	r.wantBot(t, token, true, "", 7)
}

func TestHouseBotsNeverDetachAndRefillWhenAHumanLeaves(t *testing.T) {
	r := stayRig(t, 2, 3)
	token, _ := r.join(t, "Ann")
	r.tick(t, 22)
	r.wantBot(t, token, false, app.LeftByTime, -1)
	if got := len(r.feed.Last().Roster); got != 2 {
		t.Errorf("roster size %d, want 2 house bots", got)
	}
	for _, b := range r.a.Bots() {
		if b.House && b.Detached {
			t.Errorf("house bot detached: %+v", b)
		}
	}
}

func detachedIn(bots []app.BotInfo, id string) bool {
	for _, b := range bots {
		if b.ID == id {
			return b.Detached
		}
	}
	return false
}

func TestKickDeletesADetachedBotAndRevokesItsToken(t *testing.T) {
	r := newRig(t, 0)
	token, id := r.join(t, "Ann")
	r.tick(t, 2)
	_ = r.a.Leave(token)
	r.tick(t, 1)
	if !detachedIn(r.a.Bots(), id) {
		t.Fatalf("not listed as detached: %+v", r.a.Bots())
	}
	must(t, r.a.Kick(id))
	r.tick(t, 1)
	if _, err := r.a.GetBot(token); !errors.Is(err, app.ErrUnauthorized) {
		t.Errorf("token: %v", err)
	}
	if boardHas(r.a.Leaderboard(), id) || len(r.a.Bots()) != 0 || len(r.store.Bots) != 0 {
		t.Errorf("board %v bots %v saved %v", r.a.Leaderboard(), r.a.Bots(), r.store.Bots)
	}
}

func TestKickOfABotWithAPendingLeaveRecordsOneKickedLeave(t *testing.T) {
	r := newRig(t, 0)
	token, id := r.join(t, "Ann")
	r.tick(t, 2)
	_ = r.a.Leave(token)
	must(t, r.a.Kick(id))
	r.tick(t, 1)
	if rec := r.a.Record(); len(rec.Leaves) != 1 || !rec.Leaves[0].Kicked {
		t.Errorf("%+v", rec.Leaves)
	}
	if err := r.a.Kick(id); !errors.Is(err, app.ErrUnknownBot) {
		t.Errorf("%v", err)
	}
}

func TestPutBotAndSetProfileWorkWhileDetached(t *testing.T) {
	r := newRig(t, 0)
	token, id := r.join(t, "Ann")
	r.tick(t, 2)
	_ = r.a.Leave(token)
	r.tick(t, 1)
	look := app.DefaultLook("zzz")
	must(t, r.a.PutBot(token, idler))
	must(t, r.a.SetProfile(token, "Anna", look))
	r.tick(t, 3)
	r.wantCounts(t, 1, 1, 0)
	if v := r.bot(t, token); v.Source != idler || v.Name != "Anna" || v.Look != look {
		t.Errorf("%+v", v)
	}
	must(t, r.a.Return(token))
	r.tick(t, 1)
	joins := r.a.Record().Joins
	if joins[len(joins)-1].Brain.Src != idler {
		t.Errorf("%+v", joins)
	}
	r.wantRoster(t, id)
}

func TestDetachedStateSurvivesARestartAndAnArenaBotKeepsItsCount(t *testing.T) {
	r := stayRig(t, 0, 100)
	gone, goneID := r.join(t, "Gone")
	stay, stayID := r.join(t, "Stay")
	r.tick(t, 30)
	_ = r.a.Leave(gone)
	r.tick(t, 10)
	must(t, r.a.Flush())
	r.a.Close()
	r.start(t)
	r.tick(t, 1)
	r.wantRoster(t, stayID)
	r.wantBot(t, gone, false, app.LeftByPlayer, -1)
	r.wantBot(t, stay, true, "", 59) // 41 beats in the match
	must(t, r.a.Return(gone))
	r.tick(t, 1)
	r.wantRoster(t, goneID+","+stayID)
}

func TestTheStayCountIsSavedAtLeastEverySixtyBeats(t *testing.T) {
	r := stayRig(t, 0, 1000)
	r.join(t, "Ann")
	r.tick(t, 60)
	if len(r.store.Bots) != 1 || r.store.Bots[0].StayBeats < 59 {
		t.Errorf("%+v", r.store.Bots)
	}
}

func TestReplayOfASessionWithLeavesAndReturnsGivesTheSameHash(t *testing.T) {
	r := stayRig(t, 2, 40)
	var live []event.Logged
	step := func(n int) {
		for range n {
			r.tick(t, 1)
			live = append(live, r.feed.Last().Events...)
		}
	}
	ann, _ := r.join(t, "Ann")
	bob, _ := r.join(t, "Bob")
	step(10)
	_ = r.a.PutBot(ann, firer)
	_ = r.a.Leave(bob)
	step(10)
	_ = r.a.Return(bob)
	step(50) // both reach the stay limit
	_ = r.a.Return(ann)
	step(30)
	rec := r.a.Record()
	replayed, err := app.Replay(rec)
	must(t, err)
	want, _ := app.HashLog(live)
	if got, _ := app.HashLog(replayed); got != want || len(rec.Leaves) < 3 || len(rec.Joins) < 5 {
		t.Errorf("hash %s, live %s, %d leaves, %d joins", got, want, len(rec.Leaves), len(rec.Joins))
	}
}

func TestADetachedBotStaysDetachedAcrossANewMapAndCanReturn(t *testing.T) {
	r := newRig(t, 0)
	ann, _ := r.join(t, "Ann")
	bob, bobID := r.join(t, "Bob")
	r.tick(t, 3)
	_ = r.a.Leave(bob)
	r.tick(t, 1)
	applyAndTick(t, r, room)
	r.wantBot(t, bob, false, app.LeftByPlayer, -1)
	r.wantRoster(t, "p1")
	r.tick(t, 3)
	must(t, r.a.Return(bob))
	r.tick(t, 1)
	r.wantRoster(t, "p1,"+bobID)
	r.wantBot(t, ann, true, "", -1)
}
