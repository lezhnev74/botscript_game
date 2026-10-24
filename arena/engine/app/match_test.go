package app

import (
	"strings"
	"testing"

	"github.com/lezhnev/arena/engine/domain/brain"
	"github.com/lezhnev/arena/engine/domain/geo"
	"github.com/lezhnev/arena/engine/domain/sim"
)

func mustMap(t *testing.T, text string) geo.Map {
	t.Helper()
	m, err := geo.ParseMap(text)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

const twoBots = "#####\n#A.B#\n#...#\n#####\n"

type recorder struct {
	id   string
	seen *[]string
}

func (r recorder) Decide(brain.Perception) brain.Action {
	*r.seen = append(*r.seen, r.id)
	return brain.Action{}
}

func TestStepAsksBrainsSortedByID(t *testing.T) {
	m := mustMap(t, twoBots)
	var seen []string
	brains := map[string]brain.Brain{"B": recorder{"B", &seen}, "A": recorder{"A", &seen}}
	NewMatch(sim.NewWorld(m, sim.DefaultRules(), 1), sim.DefaultRules(), brains).Step()
	if strings.Join(seen, "") != "AB" {
		t.Errorf("order %v", seen)
	}
}

type peeker struct{ last *brain.Outcome }

func (p peeker) Decide(pc brain.Perception) brain.Action {
	*p.last = pc.Last
	return brain.Action{Kind: brain.Move, Dir: geo.N}
}

func TestStepFeedsPreviousOutcomeToDecide(t *testing.T) {
	m := mustMap(t, twoBots)
	var last brain.Outcome
	brains := map[string]brain.Brain{"A": peeker{&last}, "B": peeker{new(brain.Outcome)}}
	mt := NewMatch(sim.NewWorld(m, sim.DefaultRules(), 1), sim.DefaultRules(), brains)
	mt.Step() // moves N into a wall
	mt.Step()
	if last.OK || last.Act.Kind != brain.Move {
		t.Errorf("beat 2 saw %+v, want a failed move", last)
	}
}

func TestStepTagsEventsWithTheBeat(t *testing.T) {
	m := mustMap(t, twoBots)
	script := brain.NewScripted([]brain.Action{{}, {Kind: brain.Move, Dir: geo.S}})
	brains := map[string]brain.Brain{"A": script, "B": brain.NewScripted(nil)}
	mt := NewMatch(sim.NewWorld(m, sim.DefaultRules(), 1), sim.DefaultRules(), brains)
	if got := mt.Step(); len(got) != 0 {
		t.Fatalf("beat 1 events %v", got)
	}
	got := mt.Step()
	if len(got) != 1 || got[0].Beat != 2 || mt.World().Beat != 2 {
		t.Errorf("beat 2 events %v, world beat %d", got, mt.World().Beat)
	}
}
