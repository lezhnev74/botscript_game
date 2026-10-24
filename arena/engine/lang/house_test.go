package lang_test

import (
	"reflect"
	"testing"

	"github.com/lezhnev/arena/engine/lang"
)

func TestHouseBotNamesAreSorted(t *testing.T) {
	want := []string{"forager", "hound", "lighthouse", "rookie", "tracker"}
	if got := lang.HouseBotNames(); !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestEveryHouseBotValidates(t *testing.T) {
	for _, name := range lang.HouseBotNames() {
		src, ok := lang.HouseBot(name)
		if !ok {
			t.Fatalf("%s: listed but not found", name)
		}
		if _, errs := lang.Validate(src); len(errs) > 0 {
			t.Errorf("%s: %v", name, errs)
		}
	}
}

func TestHouseBotUnknownName(t *testing.T) {
	if src, ok := lang.HouseBot("nobody"); ok || src != "" {
		t.Errorf("got %q, %v", src, ok)
	}
}

func TestProbeReportsMemoryAndTheLatestAct(t *testing.T) {
	h := newHarness(t, "function loop() { num[0] = 5; num[2] = 7; move(E); }")
	if got := h.b.Probe(); got["act"] != -1 || got["num_0"] != 0 {
		t.Errorf("before the first beat: %v", got)
	}
	h.beat()
	want := map[string]int{"num_0": 5, "num_1": 0, "num_2": 7, "num_3": 0, "num_4": 0, "num_5": 0, "num_6": 0, "num_7": 0,
		"pin_0_set": 0, "pin_0_x": -1, "pin_0_y": -1, "pin_1_set": 0, "pin_1_x": -1, "pin_1_y": -1, "pin_2_set": 0, "pin_2_x": -1, "pin_2_y": -1, "pin_3_set": 0, "pin_3_x": -1, "pin_3_y": -1,
		"pin_4_set": 0, "pin_4_x": -1, "pin_4_y": -1, "pin_5_set": 0, "pin_5_x": -1, "pin_5_y": -1, "pin_6_set": 0, "pin_6_x": -1, "pin_6_y": -1, "pin_7_set": 0, "pin_7_x": -1, "pin_7_y": -1,
		"act": int(lang.ActMove), "line": 1}
	if got := h.b.Probe(); !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestProbeActOfAMissedBeatIsIdle(t *testing.T) {
	h := newHarness(t, "function loop() { while (1) { } }")
	h.beat()
	if got := h.b.Probe()["act"]; got != int(lang.ActIdle) {
		t.Errorf("act %d, want IDLE (%d)", got, int(lang.ActIdle))
	}
}

func TestProbeLineIsTheLineOfTheLatestActionCall(t *testing.T) {
	h := newHarness(t, "function loop() {\n  if (num[0] == 0) {\n    num[0] = 1;\n    move(E);\n  }\n  fire(S);\n}")
	if got := h.b.Probe()["line"]; got != 0 {
		t.Errorf("before the first beat: line %d", got)
	}
	h.beat()
	if got := h.b.Probe()["line"]; got != 4 {
		t.Errorf("after move: line %d, want 4", got)
	}
	h.beat()
	if got := h.b.Probe()["line"]; got != 6 {
		t.Errorf("after fire: line %d, want 6", got)
	}
}

func TestHotSwapClearsTheProbeLine(t *testing.T) {
	h := newHarness(t, "function loop() {\n  wait();\n}")
	h.beat()
	h.b.Swap(mustProgram(t, "function loop() {\n\n\n  move(N);\n}"))
	h.p.Beat++
	h.b.Decide(h.p)
	if got := h.b.Probe()["line"]; got != 4 {
		t.Errorf("line %d, want 4", got)
	}
}
