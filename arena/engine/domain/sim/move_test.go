package sim

import (
	"slices"
	"testing"

	"github.com/lezhnev/arena/engine/domain/brain"
	"github.com/lezhnev/arena/engine/domain/event"
	"github.com/lezhnev/arena/engine/domain/geo"
)

func mv(d geo.Dir) brain.Action { return brain.Action{Kind: brain.Move, Dir: d} }

func texts(evs []event.Event) []string {
	out := make([]string, len(evs))
	for i, e := range evs {
		out[i] = event.Text(e)
	}
	return out
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

type moveCase struct {
	name   string
	mapStr string
	acts   map[string]brain.Action
	events []string
	frame  string
	last   map[string]brain.Outcome
}

var moveCases = []moveCase{
	{
		name: "free cell", mapStr: "####\n#A.#\n####\n",
		acts:   map[string]brain.Action{"A": mv(geo.E)},
		events: []string{"Moved A E"},
		frame:  "####\n#.A#\n####\n",
		last:   map[string]brain.Outcome{"A": {Act: mv(geo.E), OK: true}},
	},
	{
		name: "wall is a silent no-op", mapStr: "####\n#A.#\n####\n",
		acts:   map[string]brain.Action{"A": mv(geo.N)},
		events: nil,
		frame:  "####\n#A.#\n####\n",
		last:   map[string]brain.Outcome{"A": {Act: mv(geo.N), OK: false}},
	},
	{
		name: "bump into a stationary bot", mapStr: "####\n#AB#\n####\n",
		acts:   map[string]brain.Action{"A": mv(geo.E)},
		events: []string{"Bumped A E"},
		frame:  "####\n#AB#\n####\n",
		last: map[string]brain.Outcome{
			"A": {Act: mv(geo.E), Bumped: true, BumpDir: geo.E},
			"B": {OK: true},
		},
	},
	{
		name: "same cell conflict", mapStr: "#####\n#A.B#\n#####\n",
		acts:   map[string]brain.Action{"A": mv(geo.E), "B": mv(geo.W)},
		events: []string{"Bumped A E", "Bumped B W"},
		frame:  "#####\n#A.B#\n#####\n",
		last: map[string]brain.Outcome{
			"A": {Act: mv(geo.E), Bumped: true, BumpDir: geo.E},
			"B": {Act: mv(geo.W), Bumped: true, BumpDir: geo.W},
		},
	},
	{
		name: "swap conflict", mapStr: "####\n#AB#\n####\n",
		acts:   map[string]brain.Action{"A": mv(geo.E), "B": mv(geo.W)},
		events: []string{"Bumped A E", "Bumped B W"},
		frame:  "####\n#AB#\n####\n",
		last: map[string]brain.Outcome{
			"A": {Act: mv(geo.E), Bumped: true, BumpDir: geo.E},
			"B": {Act: mv(geo.W), Bumped: true, BumpDir: geo.W},
		},
	},
	{
		name: "chain: occupant steps away, mover still bumps (snapshot rule)", mapStr: "#####\n#AB.#\n#####\n",
		acts:   map[string]brain.Action{"A": mv(geo.E), "B": mv(geo.E)},
		events: []string{"Bumped A E", "Moved B E"},
		frame:  "#####\n#A.B#\n#####\n",
		last: map[string]brain.Outcome{
			"A": {Act: mv(geo.E), Bumped: true, BumpDir: geo.E},
			"B": {Act: mv(geo.E), OK: true},
		},
	},
	{
		name: "perpendicular claim on one cell: both bump", mapStr: "#####\n#.B.#\n#A..#\n#####\n",
		acts:   map[string]brain.Action{"A": mv(geo.E), "B": mv(geo.S)},
		events: []string{"Bumped A E", "Bumped B S"},
		frame:  "#####\n#.B.#\n#A..#\n#####\n",
	},
	{
		name: "three bots claim one cell", mapStr: "#####\n#.A.#\n#B.C#\n#####\n",
		acts:   map[string]brain.Action{"A": mv(geo.S), "B": mv(geo.E), "C": mv(geo.W)},
		events: []string{"Bumped A S", "Bumped B E", "Bumped C W"},
		frame:  "#####\n#.A.#\n#B.C#\n#####\n",
	},
	{
		name: "independent movers both move", mapStr: "#####\n#A.B#\n#####\n",
		acts:   map[string]brain.Action{"A": mv(geo.E), "B": mv(geo.E)},
		events: []string{"Moved A E"},
		frame:  "#####\n#.AB#\n#####\n",
	},
	{
		name: "invalid direction fails quietly", mapStr: "####\n#A.#\n####\n",
		acts:   map[string]brain.Action{"A": {Kind: brain.Move, Dir: geo.Dir(9)}},
		events: nil,
		frame:  "####\n#A.#\n####\n",
	},
	{
		name: "non-move actions do not move", mapStr: "####\n#A.#\n####\n",
		acts:   map[string]brain.Action{"A": {Kind: brain.Wait}},
		events: nil,
		frame:  "####\n#A.#\n####\n",
		last:   map[string]brain.Outcome{"A": {Act: brain.Action{Kind: brain.Wait}, OK: true}},
	},
}

func TestMovePhase(t *testing.T) {
	for _, c := range moveCases {
		t.Run(c.name, func(t *testing.T) { checkMoveCase(t, c) })
	}
}

func checkMoveCase(t *testing.T, c moveCase) {
	t.Helper()
	w := NewWorld(mustMap(t, c.mapStr), DefaultRules(), 1)
	next, evs := Resolve(w, c.acts, DefaultRules())
	if got := texts(evs); !equal(got, c.events) {
		t.Errorf("events %v, want %v", got, c.events)
	}
	if got := next.Render(); got != c.frame {
		t.Errorf("frame\n%s\nwant\n%s", got, c.frame)
	}
	checkLast(t, next, c.last)
}

func checkLast(t *testing.T, w World, want map[string]brain.Outcome) {
	t.Helper()
	for id, o := range want {
		if got := lastOf(w, id); got != o {
			t.Errorf("%s last %+v, want %+v", id, got, o)
		}
	}
}

func lastOf(w World, id string) brain.Outcome {
	b, _ := w.bot(id)
	return b.Last
}

func TestResolveDoesNotMutateInput(t *testing.T) {
	w := NewWorld(mustMap(t, "#####\n#A$.#\n#B..#\n#####\n"), DefaultRules(), 1)
	w.Bullets = []Bullet{{Pos: geo.Pos{X: 3, Y: 2}, Dir: geo.W}}
	bots, coins, bullets := slices.Clone(w.Bots), slices.Clone(w.Coins), slices.Clone(w.Bullets)
	acts := map[string]brain.Action{"A": mv(geo.E), "B": {Kind: brain.Scan, Dir: geo.E}}
	Resolve(w, acts, DefaultRules())
	if !slices.Equal(w.Bots, bots) || !slices.Equal(w.Coins, coins) || !slices.Equal(w.Bullets, bullets) {
		t.Errorf("input world mutated: %+v", w)
	}
}

func TestPerceptionShowsLastOutcome(t *testing.T) {
	w := NewWorld(mustMap(t, "####\n#A.#\n####\n"), DefaultRules(), 1)
	next, _ := Resolve(w, map[string]brain.Action{"A": mv(geo.N)}, DefaultRules())
	p := next.Perception("A", 2)
	if p.Last.OK || p.Last.Act != mv(geo.N) || p.Last.Bumped {
		t.Errorf("wall move perception %+v", p.Last)
	}
}

func TestMoveEventsAreRegistered(t *testing.T) {
	if !event.Known("Moved") || !event.Known("Bumped") {
		t.Error("Moved/Bumped must be registered")
	}
}
