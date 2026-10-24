package app

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/lezhnev/arena/engine/domain/event"
)

const counter = "function loop() { num[0] = num[0] + 1; wait(); }"
const mover = "function loop() { move(S); }"

func editRecord() Record {
	rec := duel()
	rec.Beats = 6
	rec.Bots[0].Brain = BrainSpec{Kind: KindProgram, Src: counter}
	rec.EditLog = []Edit{{Beat: 4, Bot: "A", Brain: BrainSpec{Kind: KindProgram, Src: mover}}}
	return rec
}

func moved(log []event.Logged, bot string) []int {
	var beats []int
	for _, l := range log {
		if m, ok := l.Event.(event.Moved); ok && m.Bot == bot {
			beats = append(beats, l.Beat)
		}
	}
	return beats
}

func stepN(m *Match, n int) []event.Logged {
	var log []event.Logged
	for i := 0; i < n; i++ {
		log = append(log, m.Step()...)
	}
	return log
}

func TestEditSwapsTheProgramAtItsBeatAndKeepsMemory(t *testing.T) {
	m, err := editRecord().NewMatch()
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	before := stepN(m, 3)
	if got := m.Probes()["A.num_0"]; got != 3 || len(moved(before, "A")) != 0 {
		t.Fatalf("before the edit: num[0] %d, moves %v", got, moved(before, "A"))
	}
	if got := moved(stepN(m, 1), "A"); len(got) != 1 || got[0] != 4 {
		t.Errorf("the new program should move on beat 4, moved on %v", got)
	}
	if got := m.Probes()["A.num_0"]; got != 3 {
		t.Errorf("num[0] = %d after the swap, want 3 (kept)", got)
	}
}

func TestProbesCoverProgramBotsOnly(t *testing.T) {
	m, err := editRecord().NewMatch()
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	m.Step()
	got := m.Probes()
	if _, ok := got["B.num_0"]; ok || got["A.act"] != 3 {
		t.Errorf("probes %v", got)
	}
}

func TestReplayWithAnEditLogIsRepeatableAfterJSON(t *testing.T) {
	data, err := json.Marshal(editRecord())
	if err != nil {
		t.Fatal(err)
	}
	back, err := DecodeRecord(data)
	if err != nil {
		t.Fatal(err)
	}
	if a, b := hashOf(t, editRecord()), hashOf(t, back); a != b {
		t.Errorf("round trip changed the replay: %s vs %s", a, b)
	}
	plain := editRecord()
	plain.EditLog = nil
	if hashOf(t, plain) == hashOf(t, editRecord()) {
		t.Error("the edit changed nothing")
	}
}

func TestBadEditsAreRejected(t *testing.T) {
	cases := map[string]func(*Record){
		"bad source":      func(r *Record) { r.EditLog[0].Brain.Src = "function loop() { let x; }" },
		"unknown bot":     func(r *Record) { r.EditLog[0].Bot = "Z" },
		"script edit":     func(r *Record) { r.EditLog[0].Brain = BrainSpec{Kind: KindScript, Src: "move S"} },
		"beat zero":       func(r *Record) { r.EditLog[0].Beat = 0 },
		"scripted target": func(r *Record) { r.EditLog[0].Bot = "B" },
	}
	for name, mutate := range cases {
		rec := editRecord()
		mutate(&rec)
		if _, err := Replay(rec); err == nil || !strings.Contains(err.Error(), "edit_log[0]") {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}

func TestTwoEditsOnOneBeatApplyInLogOrder(t *testing.T) {
	rec := editRecord()
	rec.EditLog = append(rec.EditLog, Edit{Beat: 4, Bot: "A", Brain: BrainSpec{Kind: KindProgram, Src: "function loop() { wait(); }"}})
	log, err := Replay(rec)
	if err != nil {
		t.Fatal(err)
	}
	if got := moved(log, "A"); len(got) != 0 {
		t.Errorf("the later edit should win, A moved on %v", got)
	}
}
