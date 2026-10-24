package scenario

import (
	"errors"
	"strings"
	"testing"

	"github.com/lezhnev/arena/engine/domain/brain"
	"github.com/lezhnev/arena/engine/domain/geo"
	"github.com/lezhnev/arena/engine/domain/sim"
)

func twoBotScenario(t *testing.T) Scenario {
	t.Helper()
	m, err := geo.ParseMap("####\n#AB#\n####\n")
	if err != nil {
		t.Fatal(err)
	}
	return Scenario{
		Rules: sim.DefaultRules(), Seed: 1, Map: m, Beats: 3,
		Bots: []BotSpec{{ID: "A", Script: []brain.Action{{Kind: brain.Fire, Dir: geo.E}}}},
	}
}

func TestRunKeepsWorldPerBeat(t *testing.T) {
	res := Run(twoBotScenario(t))
	if len(res.Worlds) != 4 { // initial + 3 beats
		t.Fatalf("worlds = %d", len(res.Worlds))
	}
	if len(res.Log) != 2 { // A fires on beat 1 and hits the adjacent B, then waits
		t.Errorf("log = %v", res.Log)
	}
}

func TestVerifyStateAndFrames(t *testing.T) {
	s := twoBotScenario(t)
	s.Expect = []Expectation{
		{Line: 1, Kind: ExpState, Final: true, Words: []string{"A.x", "1"}},
		{Line: 2, Kind: ExpState, Beat: 1, Words: []string{"B.hp", "3"}},
	}
	s.Frames = []Frame{{Line: 3, Beat: 1, Text: "####\n#AB#  \n####\n\n"}}
	if err := Verify(s, Run(s)); err != nil {
		t.Fatal(err)
	}
}

func TestVerifyStateFailures(t *testing.T) {
	s := twoBotScenario(t)
	s.Expect = []Expectation{
		{Line: 5, Kind: ExpState, Final: true, Words: []string{"A.x", "2"}},
		{Line: 6, Kind: ExpState, Final: true, Words: []string{"Z.x", "2"}},
		{Line: 7, Kind: ExpState, Beat: 1, Words: []string{"A.x", "q"}},
	}
	err := Verify(s, Run(s))
	if err == nil {
		t.Fatal("expected failures")
	}
	for _, want := range []string{"line 5", "final: A.x 2", "got 1", "line 6", "no bot", "line 7"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("message lacks %q:\n%v", want, err)
		}
	}
}

func TestVerifyFrameMismatch(t *testing.T) {
	s := twoBotScenario(t)
	s.Frames = []Frame{{Line: 9, Beat: 2, Text: "####\n#A.B\n####\n"}}
	err := Verify(s, Run(s))
	if err == nil || !strings.Contains(err.Error(), "line 9") || !strings.Contains(err.Error(), "#AB#") {
		t.Fatalf("got %v", err)
	}
}

func TestRunAppliesStartState(t *testing.T) {
	s := twoBotScenario(t)
	s.Start = []StartSpec{{ID: "A", Fields: []StartField{{"bonus", 2}, {"cooldown", 3}}}}
	m, err := newMatch(s)
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := m.World().Query("A.bonus"); got != 2 {
		t.Errorf("bonus %d", got)
	}
	res := Run(s)
	if got, _ := res.Worlds[0].Query("A.cooldown"); got != 3 {
		t.Errorf("initial world ignores start: cooldown %d", got)
	}
}

func TestStartWorldErrorsCarryTheLine(t *testing.T) {
	s := twoBotScenario(t)
	s.Start = []StartSpec{{Line: 9, ID: "A", Fields: []StartField{{"bonus", 3}}}}
	_, err := newMatch(s)
	var pe *ParseError
	if !errors.As(err, &pe) || pe.Line != 9 {
		t.Errorf("err = %v", err)
	}
}
