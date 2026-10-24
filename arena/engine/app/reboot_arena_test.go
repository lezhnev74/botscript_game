package app_test

import (
	"errors"
	"testing"

	"github.com/lezhnev/arena/engine/app"
	"github.com/lezhnev/arena/engine/domain/event"
	"github.com/lezhnev/arena/engine/domain/sim"
)

const rebootBeats = 4

func rebootRig(t *testing.T) *rig {
	t.Helper()
	r := newUnstartedRig(0, room)
	r.cfg.RebootBeats = rebootBeats
	r.start(t)
	return r
}

func (r *rig) rebooting(id string) bool { return r.a.World().IsRebooting(id) }

func (r *rig) hasBot(id string) bool {
	for _, b := range r.a.World().Bots {
		if b.ID == id {
			return true
		}
	}
	return false
}

func (r *rig) wantRebooting(t *testing.T, token, id string, rebooting bool, left int) {
	t.Helper()
	if r.rebooting(id) != rebooting || r.hasBot(id) == rebooting || r.bot(t, token).RebootLeft != left {
		t.Errorf("rebooting %v on map %v left %d, want rebooting %v left %d", r.rebooting(id), r.hasBot(id), r.bot(t, token).RebootLeft, rebooting, left)
	}
}

func TestASavedProgramTakesTheBotOffTheMapForRebootBeatsThenBringsItBack(t *testing.T) {
	r := rebootRig(t)
	token, id := r.join(t, "Ann")
	r.tick(t, 3)
	must(t, r.a.PutBot(token, idler))
	if v := r.bot(t, token); v.RebootLeft != rebootBeats {
		t.Errorf("queued reboot reports %d beats left", v.RebootLeft)
	}
	r.tick(t, 1) // beat 4: the reboot begins
	r.wantRebooting(t, token, id, true, rebootBeats-1)
	r.wantRoster(t, id) // the roster keeps it
	r.tick(t, rebootBeats-1)
	r.wantRebooting(t, token, id, false, -1)
}

func TestARebootIsRecordedInTheEditLog(t *testing.T) {
	r := rebootRig(t)
	token, _ := r.join(t, "Ann")
	r.tick(t, 3)
	must(t, r.a.PutBot(token, idler))
	r.tick(t, 1)
	edits := r.a.Record().EditLog
	if len(edits) != 1 || edits[0].Beat != 4 || edits[0].Reboot != rebootBeats {
		t.Errorf("edit log %+v", edits)
	}
}

func TestRebootKeepsHPAndNeedsNoRateLimit(t *testing.T) {
	r := rebootRig(t)
	token, id := r.join(t, "Ann")
	r.tick(t, 2)
	must(t, r.a.PutBot(token, idler))
	must(t, r.a.PutBot(token, waiter)) // the reboot is the limit: no 429
	r.tick(t, 1+rebootBeats)
	var hp int
	for _, b := range r.a.World().Bots {
		if b.ID == id {
			hp = b.HP
		}
	}
	if hp != sim.DefaultRules().HP {
		t.Errorf("hp %d", hp)
	}
	if v := r.bot(t, token); v.Source != waiter {
		t.Errorf("source %q: the latest save should win", v.Source)
	}
}

func TestASaveDuringARebootReplacesTheProgramButKeepsTheCountdown(t *testing.T) {
	r := rebootRig(t)
	token, id := r.join(t, "Ann")
	r.tick(t, 2)
	must(t, r.a.PutBot(token, idler))
	r.tick(t, 2) // beats 3, 4: two beats into the reboot
	must(t, r.a.PutBot(token, waiter))
	r.tick(t, 1)
	left := r.bot(t, token).RebootLeft
	r.tick(t, left)
	if r.rebooting(id) || !r.hasBot(id) || left != 1 {
		t.Errorf("rebooting %v back %v left %d: the countdown must go on", r.rebooting(id), r.hasBot(id), left)
	}
	if edits := r.a.Record().EditLog; len(edits) != 2 || edits[1].Brain.Src != waiter {
		t.Errorf("edit log %+v", edits)
	}
}

func TestReplayOfASessionWithRebootsGivesTheLiveHash(t *testing.T) {
	r := rebootRig(t)
	token, _ := r.join(t, "Ann")
	var live []event.Logged
	step := func(n int) {
		for range n {
			r.tick(t, 1)
			live = append(live, r.feed.Last().Events...)
		}
	}
	step(3)
	must(t, r.a.PutBot(token, firer))
	step(2)
	must(t, r.a.PutBot(token, walker))
	step(20)
	replayed, err := app.Replay(r.a.Record())
	if err != nil {
		t.Fatal(err)
	}
	want, _ := app.HashLog(live)
	if got, _ := app.HashLog(replayed); got != want {
		t.Errorf("replay hash %s, live hash %s", got, want)
	}
}

func TestReplayOfASessionWithARebootOfADeadBotGivesTheLiveHash(t *testing.T) {
	r := rebootRig(t)
	ann, _ := r.join(t, "Ann")
	bob, _ := r.join(t, "Bob")
	fire := "function loop() { if (pick(0, 1) == 1) { fire(pick(N, E, S, W)); } else { move(pick(N, E, S, W)); } }"
	var live []event.Logged
	r.tick(t, 1)
	live = append(live, r.feed.Last().Events...)
	must(t, r.a.PutBot(ann, fire))
	must(t, r.a.PutBot(bob, fire))
	saved := false
	for range 200 {
		r.tick(t, 1)
		live = append(live, r.feed.Last().Events...)
		if w := r.a.World(); !saved && len(w.Fallen) > 0 {
			must(t, r.a.PutBot(ann, walker)) // both are alike: a save for the fallen one, whoever it is
			must(t, r.a.PutBot(bob, walker))
			saved = true
		}
	}
	if !saved {
		t.Fatal("nobody died")
	}
	replayed, err := app.Replay(r.a.Record())
	if err != nil {
		t.Fatal(err)
	}
	want, _ := app.HashLog(live)
	if got, _ := app.HashLog(replayed); got != want {
		t.Errorf("replay hash %s, live hash %s", got, want)
	}
}

func TestThePencilBadgeAppearsOnTheBeatTheRebootedBotIsBack(t *testing.T) {
	r := rebootRig(t)
	token, id := r.join(t, "Ann")
	r.tick(t, 3)
	must(t, r.a.PutBot(token, idler))
	for r.a.World().Beat < 20 {
		r.tick(t, 1)
		_, badge := r.feed.Last().Probes[id+".edited"]
		if back := r.hasBot(id); back != badge {
			t.Fatalf("beat %d: on map %v, badge %v", r.a.World().Beat, back, badge)
		}
		if badge {
			if got := r.feed.Last().Probes[id+".edited"]; got != r.a.World().Beat {
				t.Fatalf("badge beat %d, want %d", got, r.a.World().Beat)
			}
			return
		}
	}
	t.Fatal("the bot never came back")
}

func TestProgramSavesOutsideAGameDoNotReboot(t *testing.T) {
	r := rebootRig(t)
	token, id := r.join(t, "Ann")
	must(t, r.a.PutBot(token, idler)) // the join has not landed yet
	r.tick(t, 1)
	if r.rebooting(id) || !r.hasBot(id) {
		t.Fatal("a bot still waiting to join must not reboot")
	}
	must(t, r.a.Leave(token))
	r.tick(t, 1)
	must(t, r.a.PutBot(token, waiter)) // detached: only stored
	r.tick(t, 1)
	if v := r.bot(t, token); v.Source != waiter || v.RebootLeft != -1 || r.rebooting(id) {
		t.Errorf("detached save: %+v", v)
	}
	if len(r.a.Record().EditLog) != 0 {
		t.Errorf("edit log %+v", r.a.Record().EditLog)
	}
}

func TestSavingWhileTheGameIsStoppedAppliesAtStartWithoutAReboot(t *testing.T) {
	r := rebootRig(t)
	token, id := r.join(t, "Ann")
	r.tick(t, 2)
	must(t, r.a.SetTimer(1))
	r.tick(t, 1)
	must(t, r.a.PutBot(token, waiter))
	must(t, r.a.StartGame())
	r.tick(t, 1)
	if r.rebooting(id) || !r.hasBot(id) || r.bot(t, token).Source != waiter {
		t.Errorf("rebooting %v on map %v", r.rebooting(id), r.hasBot(id))
	}
}

func TestLeavingDuringARebootCancelsItAndAReturnWaitsRebootBeats(t *testing.T) {
	r := rebootRig(t)
	token, id := r.join(t, "Ann")
	r.tick(t, 2)
	must(t, r.a.PutBot(token, idler))
	r.tick(t, 1)
	landed := r.a.World().Beat + 1
	must(t, r.a.Leave(token))
	r.tick(t, 2)
	if r.rebooting(id) || r.hasBot(id) {
		t.Error("leave should cancel the reboot")
	}
	want := landed + rebootBeats - (r.a.World().Beat + 1)
	if got := retryBeats(r.a.Return(token)); got != want || want <= 0 {
		t.Fatalf("retry %d, want %d", got, want)
	}
	r.tick(t, want)
	must(t, r.a.Return(token))
	r.tick(t, 1)
	if r.rebooting(id) || !r.hasBot(id) {
		t.Error("the returning bot joins, not rebooting")
	}
}

func TestLeaveSaveAndReturnCannotPutANewProgramOnTheMapSoonerThanRebootBeats(t *testing.T) {
	r := rebootRig(t)
	token, id := r.join(t, "Ann")
	r.tick(t, 3)
	landed := r.a.World().Beat + 1
	must(t, r.a.Leave(token))
	r.tick(t, 1)
	must(t, r.a.PutBot(token, waiter)) // detached: only stored
	for r.a.World().Beat+1 < landed+rebootBeats {
		if err := r.a.Return(token); retryBeats(err) <= 0 {
			t.Fatalf("return at beat %d: %v", r.a.World().Beat, err)
		}
		r.tick(t, 1)
	}
	must(t, r.a.Return(token))
	r.tick(t, 1)
	if got := r.a.World().Beat; got < landed+rebootBeats || !r.hasBot(id) {
		t.Errorf("back on beat %d (on map %v), leave landed on %d", got, r.hasBot(id), landed)
	}
}

func TestAnAutomaticLeaveDoesNotDelayTheReturn(t *testing.T) {
	r := rebootRig(t)
	r.cfg.MaxStay = 3
	r.start(t)
	token, id := r.join(t, "Ann")
	r.tick(t, 6)
	if v := r.bot(t, token); v.InArena {
		t.Fatal("the stay should be over")
	}
	must(t, r.a.Return(token))
	r.tick(t, 1)
	if !r.hasBot(id) {
		t.Error("an auto-left bot returns at once")
	}
}

func TestKickDuringARebootRemovesTheBot(t *testing.T) {
	r := rebootRig(t)
	token, id := r.join(t, "Ann")
	r.tick(t, 2)
	must(t, r.a.PutBot(token, idler))
	r.tick(t, 1)
	must(t, r.a.Kick(id))
	r.tick(t, 1)
	if r.rebooting(id) || r.hasBot(id) {
		t.Error("kick should cancel the reboot")
	}
	if _, err := r.a.GetBot(token); !errors.Is(err, app.ErrUnauthorized) {
		t.Errorf("token still valid: %v", err)
	}
}

func TestANewGameCancelsARunningReboot(t *testing.T) {
	r := rebootRig(t)
	token, id := r.join(t, "Ann")
	r.tick(t, 2)
	must(t, r.a.PutBot(token, waiter))
	r.tick(t, 1)
	must(t, r.a.ApplyMap(bigRoom))
	r.tick(t, 1)
	if r.rebooting(id) || !r.hasBot(id) || r.bot(t, token).Source != waiter {
		t.Errorf("rebooting %v on map %v", r.rebooting(id), r.hasBot(id))
	}
}

func TestTheStayCountRunsDuringAReboot(t *testing.T) {
	r := rebootRig(t)
	r.cfg.MaxStay = 10
	r.start(t)
	token, _ := r.join(t, "Ann")
	r.tick(t, 2)
	must(t, r.a.PutBot(token, waiter))
	r.tick(t, 3)
	if v := r.bot(t, token); v.StayLeft != 5 {
		t.Errorf("stay left %d, want 5", v.StayLeft)
	}
}

func TestHouseBotsNeverReboot(t *testing.T) {
	r := newUnstartedRig(3, room)
	r.cfg.RebootBeats = rebootBeats
	r.start(t)
	token, _ := r.join(t, "Ann")
	r.tick(t, 2)
	must(t, r.a.PutBot(token, waiter))
	r.tick(t, 2)
	if got := len(r.a.World().Rebooting); got != 1 {
		t.Errorf("%d bots rebooting, want only the human", got)
	}
}

func TestWithoutRebootBeatsASaveIsAPlainHotSwapWithTheRateLimit(t *testing.T) {
	r := newRig(t, 0)
	token, id := r.join(t, "Ann")
	r.tick(t, 3)
	must(t, r.a.PutBot(token, idler))
	if err := r.a.PutBot(token, waiter); retryBeats(err) <= 0 {
		t.Errorf("second save: %v", err)
	}
	r.tick(t, 1)
	if r.a.World().IsRebooting(id) || r.bot(t, token).RebootLeft != -1 {
		t.Error("RebootBeats 0 must not reboot")
	}
}
