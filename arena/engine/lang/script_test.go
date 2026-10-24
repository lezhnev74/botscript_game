package lang_test

import (
	"runtime"
	"testing"

	"github.com/lezhnev/arena/engine/domain/brain"
	"github.com/lezhnev/arena/engine/domain/geo"
	"github.com/lezhnev/arena/engine/lang"
)

var _ brain.Brain = (*lang.ScriptBrain)(nil)

func TestActionsYieldOnePerBeat(t *testing.T) {
	h := newHarness(t, "function loop() { move(E); fire(N); scan(S); wait(); }")
	h.want(h.beat(), act(brain.Move, geo.E))
	h.want(h.beat(), act(brain.Fire, geo.N))
	h.want(h.beat(), act(brain.Scan, geo.S))
	h.want(h.beat(), wait)
	h.want(h.beat(), act(brain.Move, geo.E)) // loop called again
}

func TestThinkingIsFreeWithinABeat(t *testing.T) {
	h := newHarness(t, "function loop() { num[0]++; num[0]++; num[1] = num[0] * 10; move(E); }")
	h.want(h.beat(), act(brain.Move, geo.E))
	h.mem(2, 20, 0)
}

func TestLoopWithoutActionIsWait(t *testing.T) {
	h := newHarness(t, "function loop() { num[0]++; }")
	for i := 1; i <= 3; i++ {
		h.want(h.beat(), wait)
		h.mem(i, 0, 0)
	}
}

func TestEarlyReturnWaitsThenActs(t *testing.T) {
	h := newHarness(t, "function loop() { if (num[0] < 2) { num[0]++; return; } move(E); }")
	h.want(h.beat(), wait)
	h.want(h.beat(), wait)
	h.want(h.beat(), act(brain.Move, geo.E))
	h.want(h.beat(), act(brain.Move, geo.E))
}

func TestLoopThatActsDoesNotWaitInTheSameBeat(t *testing.T) {
	h := newHarness(t, "function loop() { num[0]++; scan(N); }")
	h.want(h.beat(), act(brain.Scan, geo.N))
	h.want(h.beat(), act(brain.Scan, geo.N)) // resumed, returned, restarted, acted
	h.mem(2, 0, 0)
}

func TestEndlessLoopIsIdleEveryBeatAndResumes(t *testing.T) {
	h := newHarness(t, "function loop() { while (1) { num[0]++; if (num[0] > 99) { num[0] = 0; num[1]++; } } }")
	var laps []int
	for i := 0; i < 3; i++ {
		h.want(h.beat(), wait)
		laps = append(laps, h.b.Num()[1])
	}
	if laps[0] == 0 || laps[1] <= laps[0] || laps[2] <= laps[1] {
		t.Errorf("not resumed mid-loop beat after beat: laps %v", laps)
	}
}

func TestIdleBeatsShowInTheTrail(t *testing.T) {
	h := newHarness(t, "function loop() { while (1) { if (trail.length > 0) { num[1] = trail[0].act; num[2] = trail[0].dir; } } }")
	h.beat()
	h.beat()
	h.mem(0, 4, -1) // IDLE, dir -1
}

func TestBudgetCountsFromTheLastAction(t *testing.T) {
	// Thinking worth most of the budget per beat fits; the action resets the count.
	src := "function loop() { for (num[0] = 0; num[0] < 3000; num[0]++) { num[1] = 1; } move(E); }"
	h := newHarness(t, src)
	for i := 0; i < 3; i++ {
		h.want(h.beat(), act(brain.Move, geo.E))
	}
}

const hitProg = `
function loop() { move(E); fire(E); }
function onHit(from) { num[0] = from + 10; move(S); }
`

func TestHitRunsHandlerThenLoopStartsFresh(t *testing.T) {
	h := newHarness(t, hitProg)
	h.want(h.beat(), act(brain.Move, geo.E))
	h.hitFrom(geo.W)
	h.want(h.beat(), act(brain.Move, geo.S))
	h.mem(13, 0, 0)
	h.calm()
	h.want(h.beat(), act(brain.Move, geo.E)) // fresh loop, not fire E
	h.want(h.beat(), act(brain.Fire, geo.E))
}

func TestHitWithoutHandlerIsIgnored(t *testing.T) {
	h := newHarness(t, "function loop() { move(E); fire(E); }")
	h.want(h.beat(), act(brain.Move, geo.E))
	h.hitFrom(geo.W)
	h.want(h.beat(), act(brain.Fire, geo.E))
}

func TestBumpRunsOnBump(t *testing.T) {
	h := newHarness(t, "function loop() { move(E); fire(E); } function onBump(from) { move(back(from)); }")
	h.want(h.beat(), act(brain.Move, geo.E))
	h.bumpFrom(geo.E)
	h.want(h.beat(), act(brain.Move, geo.W))
	h.calm()
	h.want(h.beat(), act(brain.Move, geo.E))
}

func TestHitBeatsBump(t *testing.T) {
	src := "function loop() { wait(); } function onHit(from) { move(N); } function onBump(from) { move(S); }"
	h := newHarness(t, src)
	h.beat()
	h.then(brain.Outcome{Hit: true, HitDir: geo.W, Bumped: true, BumpDir: geo.E})
	h.want(h.beat(), act(brain.Move, geo.N))
}

func TestBumpStillRunsWhenThereIsNoOnHit(t *testing.T) {
	h := newHarness(t, "function loop() { wait(); } function onBump(from) { move(S); }")
	h.beat()
	h.then(brain.Outcome{Hit: true, HitDir: geo.W, Bumped: true, BumpDir: geo.E})
	h.want(h.beat(), act(brain.Move, geo.S))
}

const nestedProg = `
function loop() { move(E); fire(E); }
function onHit(from) { move(from); fire(from); }
function onBump(from) { scan(from); wait(); }
`

func TestHitDuringOnHitRestartsIt(t *testing.T) {
	h := newHarness(t, nestedProg)
	h.beat()
	h.hitFrom(geo.N)
	h.want(h.beat(), act(brain.Move, geo.N))
	h.hitFrom(geo.E)
	h.want(h.beat(), act(brain.Move, geo.E)) // restarted with the new direction
	h.calm()
	h.want(h.beat(), act(brain.Fire, geo.E)) // and then runs on
	h.want(h.beat(), act(brain.Move, geo.E)) // handler returned: fresh loop
}

func TestBumpDuringOnHitIsIgnored(t *testing.T) {
	h := newHarness(t, nestedProg)
	h.beat()
	h.hitFrom(geo.N)
	h.beat()
	h.bumpFrom(geo.S)
	h.want(h.beat(), act(brain.Fire, geo.N))
}

func TestHitDuringOnBumpRunsOnHit(t *testing.T) {
	h := newHarness(t, nestedProg)
	h.beat()
	h.bumpFrom(geo.S)
	h.want(h.beat(), act(brain.Scan, geo.S))
	h.hitFrom(geo.W)
	h.want(h.beat(), act(brain.Move, geo.W))
}

func TestBumpDuringOnBumpRestartsIt(t *testing.T) {
	h := newHarness(t, nestedProg)
	h.beat()
	h.bumpFrom(geo.S)
	h.want(h.beat(), act(brain.Scan, geo.S))
	h.bumpFrom(geo.N)
	h.want(h.beat(), act(brain.Scan, geo.N))
}

func TestInterruptUnwindsNestedCallsAndKeepsMemory(t *testing.T) {
	src := `
function loop() { num[0] = 7; deep(1); num[1] = 9; }
function deep(d) { inner(d); }
function inner(d) { move(d); num[2] = 5; }
function onHit(from) { fire(from); }
`
	h := newHarness(t, src)
	h.want(h.beat(), act(brain.Move, geo.E))
	h.hitFrom(geo.S)
	h.want(h.beat(), act(brain.Fire, geo.S))
	h.mem(7, 0, 0) // c = 5 and b = 9 never ran
	h.calm()
	h.want(h.beat(), act(brain.Move, geo.E))
	h.mem(7, 0, 0)
}

func TestHandlerWithoutActionFallsThroughToLoopInTheSameBeat(t *testing.T) {
	h := newHarness(t, "function loop() { move(E); } function onHit(from) { num[0] = 5; }")
	h.beat()
	h.hitFrom(geo.W)
	h.want(h.beat(), act(brain.Move, geo.E))
	h.mem(5, 0, 0)
}

func TestHandlerAndLoopWithoutActionWait(t *testing.T) {
	h := newHarness(t, "function loop() { num[0]++; } function onHit(from) { num[1] = from; }")
	h.beat()
	h.hitFrom(geo.S)
	h.want(h.beat(), wait)
	h.mem(2, 2, 0)
}

func TestInterruptBreaksAnEndlessLoop(t *testing.T) {
	h := newHarness(t, "function loop() { while (1) { num[0]++; } } function onHit(from) { move(from); }")
	h.want(h.beat(), wait)
	h.hitFrom(geo.N)
	h.want(h.beat(), act(brain.Move, geo.N))
}

func TestInterruptOnTheFirstBeat(t *testing.T) {
	h := newHarness(t, hitProg)
	h.hitFrom(geo.E)
	h.want(h.beat(), act(brain.Move, geo.S))
	h.mem(11, 0, 0)
}

func TestTrailRecordsBeatsNewestFirst(t *testing.T) {
	h := newHarness(t, "function loop() { move(E); fire(N); scan(W); wait(); }")
	h.beat()
	if n := len(h.b.Trail()); n != 0 {
		t.Fatalf("trail before any result has %d entries", n)
	}
	h.p.Pos = geo.Pos{X: 2, Y: 1}
	h.beat() // result of move E
	h.p.Last = brain.Outcome{OK: false}
	h.beat() // result of fire N: nothing loaded
	h.p.Last = brain.Outcome{OK: true, Bumped: true, BumpDir: geo.E}
	h.beat()
	h.p.Last = brain.Outcome{OK: true, Hit: true, HitDir: geo.S}
	h.p.Pos = geo.Pos{X: 3, Y: 1}
	h.beat()
	want := []lang.TrailEntry{
		{X: 3, Y: 1, Act: lang.ActWait, Dir: -1, OK: true, Hit: 2, Bump: -1},
		{X: 2, Y: 1, Act: lang.ActScan, Dir: 3, OK: true, Hit: -1, Bump: 1},
		{X: 2, Y: 1, Act: lang.ActFire, Dir: 0, OK: false, Hit: -1, Bump: -1},
	}
	got := h.b.Trail()
	if len(got) != 3 {
		t.Fatalf("trail %+v, want 3 entries (the oldest dropped)", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("trail[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestTrailIsReadableFromTheScript(t *testing.T) {
	src := "function loop() { move(E); if (trail.length > 0) { num[0] = trail[0].x; num[1] = trail[0].act; num[2] = trail[0].ok; } }"
	h := newHarness(t, src)
	h.beat()
	h.p.Pos = geo.Pos{X: 4, Y: 1}
	h.beat()
	h.mem(4, int(lang.ActMove), 1)
}

func TestTrailRestartsAfterAGapInBeats(t *testing.T) {
	h := newHarness(t, "function loop() { wait(); }")
	h.beat()
	h.beat()
	h.p.Beat += 5 // dead for a while, respawned
	h.beat()
	if n := len(h.b.Trail()); n != 0 {
		t.Errorf("trail has %d entries across a gap", n)
	}
}

func TestScriptReadings(t *testing.T) {
	cases := map[string]int{
		"enemy": 3, "coin": 2, "bullet": 1, "last": 2, "hp": 2, "loaded": 1, "bonus": 2,
		"x": 4, "y": 3, "WIDTH": 7, "HEIGHT": 5,
	}
	for name, want := range cases {
		h := newHarness(t, "function loop() { num[0] = "+name+"; wait(); }")
		h.p.Read = brain.Readings{Dir: geo.S, Enemy: 3, Coin: 2, Bullet: 1}
		h.p.HP, h.p.Bonus, h.p.Pos = 2, 2, geo.Pos{X: 4, Y: 3}
		h.beat()
		if got := h.b.Num()[0]; got != want {
			t.Errorf("%s = %d, want %d", name, got, want)
		}
	}
}

func TestLoadedReadsZeroWhenEmpty(t *testing.T) {
	h := newHarness(t, "function loop() { num[0] = loaded + 5; wait(); }")
	h.p.Loaded = false
	h.beat()
	h.mem(5, 0, 0)
}

func TestReadingsUpdateEveryBeat(t *testing.T) {
	h := newHarness(t, "function loop() { scan(N); num[0] = enemy; }")
	h.p.Read.Enemy = 1
	h.beat()
	h.p.Read.Enemy = 3
	h.beat()
	h.mem(3, 0, 0)
}

func TestPickAvoidsWallsFromThePerception(t *testing.T) {
	h := newHarness(t, "function loop() { move(pick(N, E, S, W)); }")
	h.p.Open = [4]bool{false, true, false, false}
	for i := 0; i < 20; i++ {
		h.want(h.beat(), act(brain.Move, geo.E))
	}
}

const diceProg = "function loop() { move(pick(N, E, S, W)); }"

func diceRun(t *testing.T, seed uint64) []brain.Action {
	b := lang.NewScriptBrain(mustProgram(t, diceProg), seed)
	defer b.Close()
	h := &harness{t: t, b: b, p: brain.Perception{Open: [4]bool{true, true, true, true}}}
	return h.beats(40)
}

func TestDiceAreSeededPerBrain(t *testing.T) {
	a, b, c := diceRun(t, 5), diceRun(t, 5), diceRun(t, 6)
	same, differ := true, false
	for i := range a {
		same = same && a[i] == b[i]
		differ = differ || a[i] != c[i]
	}
	if !same || !differ {
		t.Errorf("same seed same=%v, other seed differs=%v", same, differ)
	}
}

func TestSwapStartsAFreshLoopAndKeepsMemoryAndTrail(t *testing.T) {
	h := newHarness(t, "function loop() { num[0] = 3; move(E); num[0] = 99; }")
	h.beat()
	h.p.Pos = geo.Pos{X: 2, Y: 1}
	h.b.Swap(mustProgram(t, "function loop() { num[1] = num[0] + 1; fire(N); }"))
	h.want(h.beat(), act(brain.Fire, geo.N))
	h.mem(3, 4, 0) // a = 99 never ran; slots survived
	if len(h.b.Trail()) != 1 {
		t.Errorf("trail not kept: %+v", h.b.Trail())
	}
	h.want(h.beat(), act(brain.Fire, geo.N))
}

func TestSwapBeforeTheFirstBeat(t *testing.T) {
	h := newHarness(t, "function loop() { move(E); }")
	h.b.Swap(mustProgram(t, "function loop() { move(W); }"))
	h.want(h.beat(), act(brain.Move, geo.W))
}

func TestSwapReplacesAnEndlessLoop(t *testing.T) {
	h := newHarness(t, "function loop() { while (1) { num[0]++; } }")
	h.want(h.beat(), wait)
	h.b.Swap(mustProgram(t, "function loop() { move(S); }"))
	h.want(h.beat(), act(brain.Move, geo.S))
}

func TestSwapUsesTheNewHandlersForAPendingInterrupt(t *testing.T) {
	h := newHarness(t, "function loop() { wait(); }")
	h.beat()
	h.b.Swap(mustProgram(t, "function loop() { wait(); } function onHit(from) { move(from); }"))
	h.hitFrom(geo.W)
	h.want(h.beat(), act(brain.Move, geo.W))
}

func TestSwapDuringAHandlerResetsTheMode(t *testing.T) {
	h := newHarness(t, nestedProg)
	h.beat()
	h.hitFrom(geo.N)
	h.beat() // inside onHit
	h.b.Swap(mustProgram(t, nestedProg))
	h.bumpFrom(geo.S) // not ignored: the new code is at a fresh loop
	h.want(h.beat(), act(brain.Scan, geo.S))
}

func liveGoroutines() int { return runtime.NumGoroutine() }

func TestCloseStopsTheGoroutine(t *testing.T) {
	before := liveGoroutines()
	var brains []*lang.ScriptBrain
	for i := 0; i < 5; i++ {
		b := lang.NewScriptBrain(mustProgram(t, "function loop() { while (1) { num[0]++; } }"), 1)
		b.Decide(brain.Perception{Beat: 1})
		brains = append(brains, b)
	}
	if liveGoroutines() < before+5 {
		t.Fatalf("expected the goroutines to be running")
	}
	for _, b := range brains {
		b.Close()
		b.Close() // idempotent
	}
	if !settle(10000, func() bool { return liveGoroutines() <= before }) {
		t.Errorf("goroutines leaked: %d before, %d after", before, liveGoroutines())
	}
}

func TestCloseBeforeStartAndDecideAfterClose(t *testing.T) {
	b := lang.NewScriptBrain(mustProgram(t, "function loop() { move(E); }"), 1)
	b.Close()
	if a := b.Decide(brain.Perception{Beat: 1}); a != wait {
		t.Errorf("closed brain acted: %v", a)
	}
	b2 := lang.NewScriptBrain(mustProgram(t, "function loop() { move(E); }"), 1)
	b2.Decide(brain.Perception{Beat: 1})
	b2.Close()
	if a := b2.Decide(brain.Perception{Beat: 2}); a != wait {
		t.Errorf("closed brain acted: %v", a)
	}
}

func TestInterpreterBugDoesNotHangTheMatch(t *testing.T) {
	b := lang.NewScriptBrain(&lang.Program{}, 1) // no Loop: the interpreter panics
	defer b.Close()
	for i := 1; i <= 3; i++ {
		if a := b.Decide(brain.Perception{Beat: i}); a != wait {
			t.Errorf("beat %d: %v", i, a)
		}
	}
	if b.Err() == nil {
		t.Error("the fault was swallowed")
	}
}

func TestHealthyBrainHasNoError(t *testing.T) {
	h := newHarness(t, "function loop() { wait(); }")
	h.beat()
	if err := h.b.Err(); err != nil {
		t.Error(err)
	}
}

func TestLongThinkingResumesAcrossIdleBeatsInsteadOfRestarting(t *testing.T) {
	src := "function loop() { for (num[0] = 0; num[0] < 9999; num[0]++) { for (num[1] = 0; num[1] < 5; num[1]++) { num[2] = 1; } } move(E); }"
	h := newHarness(t, src)
	idle := 0
	for ; idle < 200; idle++ {
		if a := h.beat(); a != wait {
			h.want(a, act(brain.Move, geo.E))
			break
		}
	}
	if idle < 2 || idle >= 200 {
		t.Fatalf("%d idle beats before move E; want several, and the loop must finish", idle)
	}
}

func (h *harness) respawn() *harness {
	h.p.Life++
	h.p.Last = brain.Outcome{OK: true}
	return h
}

func TestRespawnRestartsLoopKeepingMemory(t *testing.T) {
	h := newHarness(t, "function loop() { num[0]++; move(E); fire(E); }")
	h.want(h.beat(), act(brain.Move, geo.E))
	h.respawn()
	h.want(h.beat(), act(brain.Move, geo.E)) // fresh loop, not fire E
	h.mem(2, 0, 0)
}

func TestRespawnClearsTheHandlerMode(t *testing.T) {
	h := newHarness(t, nestedProg)
	h.beat()
	h.hitFrom(geo.N)
	h.beat() // inside onHit
	h.respawn()
	h.want(h.beat(), act(brain.Move, geo.E)) // loop, not the rest of onHit
	h.bumpFrom(geo.S)
	h.want(h.beat(), act(brain.Scan, geo.S)) // not ignored as "during onHit"
}

func TestRespawnClearsTheTrailEvenWithoutAGapInBeats(t *testing.T) {
	h := newHarness(t, "function loop() { wait(); }")
	h.beats(3)
	h.respawn()
	h.beat()
	if n := len(h.b.Trail()); n != 0 {
		t.Errorf("trail has %d entries after a respawn", n)
	}
	h.beat()
	if n := len(h.b.Trail()); n != 1 {
		t.Errorf("trail has %d entries one beat into the new life, want 1", n)
	}
}

func TestRespawnStartsALoopInsteadOfAStaleInterrupt(t *testing.T) {
	h := newHarness(t, hitProg)
	h.beat()
	h.p.Life++
	h.p.Last = brain.Outcome{OK: true}
	h.want(h.beat(), act(brain.Move, geo.E))
}

func TestClosedBrainStopsUpdatingItsTrail(t *testing.T) {
	h := newHarness(t, "function loop() { wait(); }")
	h.beats(3)
	h.b.Close()
	h.beats(3)
	if n := len(h.b.Trail()); n != 2 {
		t.Errorf("trail has %d entries, want the 2 from before Close", n)
	}
}

// wallProg stores wall(x+1, y) in a, wall(x-1, y) in b, wall(x, y-1) in c.
const wallProg = "function loop() { num[0] = wall(x + 1, y); num[1] = wall(x - 1, y); num[2] = wall(x, y - 1); wait(); }"

// southProg stores wall(x, y+1) in num[0].
const southProg = "function loop() { num[0] = wall(x, y + 1); wait(); }"

func wallMem(t *testing.T, prog string, pos geo.Pos) [3]int {
	t.Helper()
	m, err := geo.ParseMap("...\n.#.\n...\n")
	if err != nil {
		t.Fatal(err)
	}
	h := newHarness(t, prog)
	h.p.Grid, h.p.Pos = m.Grid, pos
	h.beat()
	h.beat() // the memory is written before the first wait() returns
	n := h.b.Num()
	return [3]int(n[:3])
}

func TestWallFloorWallAndEdges(t *testing.T) {
	tests := []struct {
		name string
		pos  geo.Pos
		want [3]int
	}{
		{"floor around", geo.Pos{X: 1, Y: 2}, [3]int{0, 0, 1}},
		{"north-west corner", geo.Pos{X: 0, Y: 0}, [3]int{0, 1, 1}},
		{"north-east corner", geo.Pos{X: 2, Y: 0}, [3]int{1, 0, 1}},
		{"south-east corner", geo.Pos{X: 2, Y: 2}, [3]int{1, 0, 0}},
		{"west edge, wall to the east", geo.Pos{X: 0, Y: 1}, [3]int{1, 1, 0}},
		{"past the corner", geo.Pos{X: -3, Y: 9}, [3]int{1, 1, 1}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := wallMem(t, wallProg, tc.pos); got != tc.want {
				t.Errorf("slots %v, want %v", got, tc.want)
			}
		})
	}
}

func TestWallSouthNeighbour(t *testing.T) {
	tests := []struct {
		name string
		pos  geo.Pos
		want int
	}{
		{"bottom row, south is outside", geo.Pos{X: 1, Y: 2}, 1},
		{"south is a wall", geo.Pos{X: 1, Y: 0}, 1},
		{"south is floor", geo.Pos{X: 0, Y: 0}, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := wallMem(t, southProg, tc.pos)[0]; got != tc.want {
				t.Errorf("a = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestMissedAndFallthroughBeatsAreCounted(t *testing.T) {
	h := newHarness(t, "function loop() { while (1) { num[0]++; } }")
	h.beats(3)
	if got := h.b.Missed(); got != 3 {
		t.Errorf("Missed = %d, want 3", got)
	}
	if got := h.b.Fallthroughs(); got != 0 {
		t.Errorf("Fallthroughs = %d, want 0", got)
	}
	f := newHarness(t, "function loop() { num[0]++; }")
	f.beats(4)
	if f.b.Fallthroughs() != 4 || f.b.Missed() != 0 {
		t.Errorf("Fallthroughs/Missed = %d/%d, want 4/0", f.b.Fallthroughs(), f.b.Missed())
	}
}
