package brain

import (
	"testing"

	"github.com/lezhnev/arena/engine/domain/geo"
)

func TestParseAction(t *testing.T) {
	cases := []struct {
		in   string
		want Action
	}{
		{"wait", Action{Kind: Wait}},
		{"  move   S ", Action{Kind: Move, Dir: geo.S}},
		{"scan N", Action{Kind: Scan, Dir: geo.N}},
		{"fire E", Action{Kind: Fire, Dir: geo.E}},
	}
	for _, c := range cases {
		got, err := ParseAction(c.in)
		if err != nil || got != c.want {
			t.Errorf("ParseAction(%q) = %v, %v; want %v", c.in, got, err, c.want)
		}
	}
}

func TestParseActionErrors(t *testing.T) {
	for _, in := range []string{"", "jump N", "move", "move X", "wait N", "ping", "ping N", "fire E E"} {
		if _, err := ParseAction(in); err == nil {
			t.Errorf("ParseAction(%q) accepted", in)
		}
	}
}

func TestActionString(t *testing.T) {
	if s := (Action{Kind: Fire, Dir: geo.E}).String(); s != "fire E" {
		t.Errorf("got %q", s)
	}
	if s := (Action{Kind: Wait}).String(); s != "wait" {
		t.Errorf("got %q", s)
	}
}

func TestScriptedPlaysThenWaits(t *testing.T) {
	b := NewScripted([]Action{{Kind: Move, Dir: geo.S}, {Kind: Fire, Dir: geo.E}})
	want := []Action{{Kind: Move, Dir: geo.S}, {Kind: Fire, Dir: geo.E}, {Kind: Wait}, {Kind: Wait}}
	for i, w := range want {
		if got := b.Decide(Perception{}); got != w {
			t.Errorf("decision %d = %v, want %v", i, got, w)
		}
	}
}

func TestParseScript(t *testing.T) {
	got, err := ParseScript("fire E, move S ,wait")
	if err != nil || len(got) != 3 || got[0] != (Action{Fire, geo.E}) || got[2].Kind != Wait {
		t.Errorf("got %v, %v", got, err)
	}
}

func TestParseScriptEdgeCases(t *testing.T) {
	if got, err := ParseScript("  "); err != nil || got != nil {
		t.Errorf("empty script: %v, %v", got, err)
	}
	if _, err := ParseScript("fire E, jump N"); err == nil {
		t.Error("want an error for an unknown action")
	}
}

func TestFormatScriptRoundTrips(t *testing.T) {
	src := "fire E, move S, wait"
	acts, _ := ParseScript(src)
	if got := FormatScript(acts); got != src {
		t.Errorf("got %q", got)
	}
}
