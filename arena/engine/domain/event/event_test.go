package event

import (
	"strings"
	"testing"

	"github.com/lezhnev/arena/engine/domain/geo"
)

func TestText(t *testing.T) {
	cases := []struct {
		e    Event
		want string
	}{
		{Moved{Bot: "B", Dir: geo.S}, "Moved B S"},
		{Fired{Bot: "A", Dir: geo.E}, "Fired A E"},
		{Clicked{Bot: "A", Dir: geo.E}, "Clicked A E"},
		{Hit{Bot: "B", From: geo.W}, "Hit B W"},
		{Killed{Killer: "A", Victim: "B"}, "Killed A B"},
		{Respawned{Bot: "B", X: 3, Y: 1}, "Respawned B 3 1"},
		{CoinTaken{Bot: "A", X: 2, Y: 4}, "CoinTaken A 2 4"},
		{CoinSpawned{X: 5, Y: 6}, "CoinSpawned 5 6"},
		{Joined{Bot: "p1", X: 2, Y: 3}, "Joined p1 2 3"},
		{Left{Bot: "p1"}, "Left p1"},
		{Kicked{Bot: "p1"}, "Kicked p1"},
	}
	for _, c := range cases {
		if got := Text(c.e); got != c.want {
			t.Errorf("Text = %q, want %q", got, c.want)
		}
	}
}

func TestJSONLine(t *testing.T) {
	got, err := JSONLine(Logged{Beat: 2, Event: Moved{Bot: "B", Dir: geo.S}})
	if err != nil {
		t.Fatal(err)
	}
	want := `{"beat":2,"type":"Moved","args":["B","S"]}`
	if string(got) != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

func TestKnown(t *testing.T) {
	if !Known("Hit") || Known("Moovd") {
		t.Error("Known mismatch")
	}
}

func TestValidatePattern(t *testing.T) {
	isBot := func(id string) bool { return id == "A" || id == "B" }
	ok := [][]string{{"Moved"}, {"Moved", "B"}, {"Moved", "B", "S"}, {"Hit", "A", "W"}}
	for _, w := range ok {
		if err := ValidatePattern(w, isBot); err != nil {
			t.Errorf("%v: %v", w, err)
		}
	}
	bad := []struct {
		words []string
		msg   string
	}{
		{nil, "empty"},
		{[]string{"Moovd"}, "unknown event"},
		{[]string{"Moved", "Z"}, "no bot"},
		{[]string{"Hit", "B", "X"}, "direction"},
		{[]string{"Moved", "B", "S", "N"}, "too many"},
	}
	for _, c := range bad {
		err := ValidatePattern(c.words, isBot)
		if err == nil || !strings.Contains(err.Error(), c.msg) {
			t.Errorf("%v: got %v, want %q", c.words, err, c.msg)
		}
	}
}

func TestValidateIntArg(t *testing.T) {
	specs["Test"] = []ArgKind{ArgInt}
	defer delete(specs, "Test")
	if err := ValidatePattern([]string{"Test", "5"}, nil); err != nil {
		t.Error(err)
	}
	if err := ValidatePattern([]string{"Test", "x"}, nil); err == nil {
		t.Error("non-integer accepted")
	}
}

// sampleEvents has one fully-populated event per registered type.
var sampleEvents = []Event{
	Moved{Bot: "B", Dir: geo.S},
	Bumped{Bot: "A", Dir: geo.E},
	Fired{Bot: "A", Dir: geo.E},
	Clicked{Bot: "A", Dir: geo.E},
	Hit{Bot: "B", From: geo.W},
	Scanned{Bot: "A", Dir: geo.E, Enemy: 3, Coin: 2, Bullet: 1},
	Killed{Killer: "A", Victim: "B"},
	Respawned{Bot: "B", X: 3, Y: 1},
	CoinTaken{Bot: "A", X: 2, Y: 4},
	CoinSpawned{X: 5, Y: 6},
	Joined{Bot: "A", X: 1, Y: 2},
	Left{Bot: "B"},
	Kicked{Bot: "B"},
}

func TestEveryRegisteredEventHasValidSample(t *testing.T) {
	isBot := func(id string) bool { return id == "A" || id == "B" }
	covered := map[string]bool{}
	for _, e := range sampleEvents {
		covered[e.Name()] = true
		words := strings.Fields(Text(e))
		if err := ValidatePattern(words, isBot); err != nil {
			t.Errorf("%s: full text does not validate: %v", Text(e), err)
		}
		if len(e.Args()) != len(specs[e.Name()]) {
			t.Errorf("%s: %d args, registry says %d", e.Name(), len(e.Args()), len(specs[e.Name()]))
		}
	}
	for name := range specs {
		if !covered[name] {
			t.Errorf("registered event %s has no sample in sampleEvents", name)
		}
	}
}

func TestScannedTextCarriesLevels(t *testing.T) {
	e := Scanned{Bot: "A", Dir: geo.E, Enemy: 3, Coin: 0, Bullet: 1}
	if got := Text(e); got != "Scanned A E 3 0 1" {
		t.Errorf("Text = %q", got)
	}
}

func TestScannedJSONHasRecord(t *testing.T) {
	e := Scanned{
		Bot: "A", Dir: geo.E, Enemy: 3,
		Cells: []Cell{{X: 2, Y: 1}},
		Seen:  []Sighting{{Kind: KindEnemy, X: 2, Y: 1, Level: 3}},
	}
	got, err := JSONLine(Logged{Beat: 1, Event: e})
	if err != nil {
		t.Fatal(err)
	}
	want := `{"beat":1,"type":"Scanned","args":["A","E","3","0","0"],` +
		`"data":{"cells":[{"x":2,"y":1}],"seen":[{"kind":"enemy","x":2,"y":1,"level":3}]}}`
	if string(got) != want {
		t.Errorf("got %s\nwant %s", got, want)
	}
}

func TestPlainEventsHaveNoData(t *testing.T) {
	got, _ := JSONLine(Logged{Beat: 1, Event: Moved{Bot: "B", Dir: geo.S}})
	if strings.Contains(string(got), "data") {
		t.Errorf("plain event carries data: %s", got)
	}
}

func TestScannedJSONUsesEmptyArraysNotNull(t *testing.T) {
	got, _ := JSONLine(Logged{Beat: 1, Event: Scanned{Bot: "A", Dir: geo.N}})
	want := `{"beat":1,"type":"Scanned","args":["A","N","0","0","0"],"data":{"cells":[],"seen":[]}}`
	if string(got) != want {
		t.Errorf("got %s\nwant %s", got, want)
	}
}

func TestDeathAndCoinPatternsValidate(t *testing.T) {
	isBot := func(id string) bool { return id == "A" || id == "B" }
	ok := [][]string{{"Killed", "A", "B"}, {"Respawned", "B", "3", "1"}, {"CoinTaken", "A"}, {"CoinSpawned", "5", "6"}}
	for _, w := range ok {
		if err := ValidatePattern(w, isBot); err != nil {
			t.Errorf("%v: %v", w, err)
		}
	}
	if err := ValidatePattern([]string{"Killed", "A", "Z"}, isBot); err == nil {
		t.Error("unknown victim accepted")
	}
}
