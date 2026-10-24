package lang_test

import (
	"testing"

	"github.com/lezhnev/arena/engine/domain/brain"
	"github.com/lezhnev/arena/engine/domain/geo"
	"github.com/lezhnev/arena/engine/lang"
)

func TestHeardAccepted(t *testing.T) {
	rows := []struct{ name, src string }{
		{"heard new sound", inLoop("if (heard != pin[0]) { pin[0] = heard; move(path(pin[0])); }")},
		{"heard fields and steps", inLoop("num[0] = heard.x + heard.y + steps(heard);")},
		{"heard none", inLoop("if (heard == none) wait();")},
	}
	for _, r := range rows {
		t.Run(r.name, func(t *testing.T) {
			if _, errs := lang.Validate(r.src); len(errs) != 0 {
				t.Fatalf("want accepted, got %v", errs)
			}
		})
	}
}

func TestHeardRejected(t *testing.T) {
	runRejected(t, []rejectRow{
		rejectAt("ping is gone", "ping();", 3, "ping", "built-ins"),
		rejectAt("pinged is gone", "pin[0] = pinged;", 12, "pinged", "unknown"),
		rejectAt("PING is gone", "if (trail[0].act == PING) wait();", 23, "PING", "unknown"),
		rejectAt("heard as a number", "num[0] = heard;", 12, "a place is not a number", ".x"),
		rejectAt("heard assigned", "heard = at(1, 1);", 3, "cannot assign to 'heard'", "pin[0]"),
		rejectAt("heardAgo is gone", "num[0] = heardAgo;", 12, "unknown", ""),
		{"param named heard", "function loop() {}\nfunction f(heard) {}", 2, 12, "heard", "place"},
	})
}

func TestHeardReading(t *testing.T) {
	const src = "pin[0] = heard; num[0] = heard.x; num[1] = heard.y; num[2] = heard == none; num[3] = path(heard);"
	rows := []struct {
		name  string
		heard lang.Place
		want  [3]int
	}{
		{"unset reads none", lang.Place{}, [3]int{-1, -1, 1}},
		{"set reads its cell", lang.Place{X: 4, Y: 2, Set: true}, [3]int{4, 2, 0}},
	}
	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			r := newRig(t, inLoop(src))
			r.env.heard = row.heard
			r.in.Run(r.prog.Loop)
			if got := [3]int(r.in.Mem.Num[:3]); got != row.want {
				t.Fatalf("got %v, want %v", got, row.want)
			}
			if p := r.in.Mem.Pin[0]; p != row.heard {
				t.Fatalf("pin[0] = %+v, want %+v", p, row.heard)
			}
		})
	}
}

func TestHeardBrainAction(t *testing.T) {
	h := newHarness(t, "function loop() {\n  if (heard != pin[0]) {\n    pin[0] = heard;\n  }\n  if (pin[0] != none) move(path(pin[0]));\n}\n")
	m, err := geo.ParseMap("....")
	if err != nil {
		t.Fatal(err)
	}
	h.p.Grid = m.Grid
	h.p.Pos = geo.Pos{X: 0, Y: 0}
	h.p.Heard = brain.Found{Pos: geo.Pos{X: 3, Y: 0}, Set: true}
	if got := h.beat(); got != act(lang.ActMove.BrainKind(), geo.E) {
		t.Fatalf("action = %v, want move E", got)
	}
}
