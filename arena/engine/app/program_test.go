package app

import (
	"runtime"
	"strings"
	"testing"

	"github.com/lezhnev/arena/engine/domain/brain"
	"github.com/lezhnev/arena/engine/lang"
)

const sweeper = `
function loop() { scan(num[0]); num[0] = right(num[0]); if (enemy == 3) fire(last); }
function onHit(from) { move(pick(left(from), right(from))); }
`

func programDuel() Record {
	rec := duel()
	rec.Beats = 30
	rec.Bots[0].Brain = BrainSpec{Kind: KindProgram, Src: sweeper}
	rec.Bots[1].Brain = BrainSpec{Kind: KindProgram, Src: "function loop() { move(pick(N, E, S, W)); fire(W); }"}
	return rec
}

func hashOf(t *testing.T, rec Record) string {
	t.Helper()
	log, err := Replay(rec)
	if err != nil {
		t.Fatal(err)
	}
	h, err := HashLog(log)
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func TestProgramBrainsReplayIdentically(t *testing.T) {
	if a, b := hashOf(t, programDuel()), hashOf(t, programDuel()); a != b {
		t.Errorf("replays differ: %s vs %s", a, b)
	}
}

func TestProgramBrainsAreSeededByTheRecordSeed(t *testing.T) {
	other := programDuel()
	other.Seed++
	if hashOf(t, programDuel()) == hashOf(t, other) {
		t.Error("the dice ignore the record seed")
	}
}

func TestProgramBrainUsesSeedForPerBot(t *testing.T) {
	spec := BrainSpec{Kind: KindProgram, Src: "function loop() { move(pick(N, E, S, W)); }"}
	b, err := NewBrain(spec, 9, "A")
	if err != nil {
		t.Fatal(err)
	}
	defer closeBrain(b)
	want := lang.NewScriptBrain(mustProg(t, spec.Src), lang.SeedFor(9, "A"))
	defer want.Close()
	open := [4]bool{true, true, true, true}
	for i := 1; i <= 20; i++ {
		p := brain.Perception{Beat: i, Open: open}
		if got, w := b.Decide(p), want.Decide(p); got != w {
			t.Fatalf("beat %d: %v vs %v", i, got, w)
		}
	}
}

func mustProg(t *testing.T, src string) *lang.Program {
	t.Helper()
	p, errs := lang.Validate(src)
	if len(errs) > 0 {
		t.Fatal(errs)
	}
	return p
}

func TestBadProgramIsRejectedWithItsErrors(t *testing.T) {
	rec := programDuel()
	rec.Bots[0].Brain.Src = "function loop() { let x = 1; }"
	_, err := Replay(rec)
	if err == nil || !strings.Contains(err.Error(), "line 1") {
		t.Errorf("err = %v", err)
	}
}

func TestReplayStopsTheProgramGoroutines(t *testing.T) {
	before := runtime.NumGoroutine()
	for i := 0; i < 3; i++ {
		hashOf(t, programDuel())
	}
	for i := 0; i < 10000 && runtime.NumGoroutine() > before; i++ {
		runtime.Gosched()
	}
	if n := runtime.NumGoroutine(); n > before {
		t.Errorf("goroutines leaked: %d before, %d after", before, n)
	}
}

type closeCounter struct{ closed *int }

func (closeCounter) Decide(brain.Perception) brain.Action { return brain.Action{} }
func (c closeCounter) Close()                             { *c.closed++ }

func TestMatchCloseClosesBrains(t *testing.T) {
	n := 0
	w, r := freshWorld(t)
	m := NewMatch(w, r, map[string]brain.Brain{
		"A": closeCounter{&n}, "B": brain.NewScripted(nil),
	})
	m.Close()
	if n != 1 {
		t.Errorf("closed %d brains, want 1", n)
	}
}
