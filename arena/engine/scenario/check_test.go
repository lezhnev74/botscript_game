package scenario

import (
	"strings"
	"testing"

	"github.com/lezhnev/arena/engine/domain/event"
	"github.com/lezhnev/arena/engine/domain/geo"
)

func fakeLog() []event.Logged {
	return []event.Logged{
		// phase order: moves before shots
		{Beat: 1, Event: event.Moved{Bot: "B", Dir: geo.S}},
		{Beat: 1, Event: event.Fired{Bot: "A", Dir: geo.E}},
		{Beat: 2, Event: event.Moved{Bot: "B", Dir: geo.S}},
		{Beat: 3, Event: event.Hit{Bot: "B", From: geo.W}},
	}
}

func ev(line, beat int, words string) Expectation {
	return Expectation{Line: line, Kind: ExpEvent, Beat: beat, Words: strings.Fields(words)}
}

func no(line, beat int, words string) Expectation {
	return Expectation{Line: line, Kind: ExpNo, Beat: beat, Words: strings.Fields(words)}
}

func TestCheckEventsPass(t *testing.T) {
	exps := []Expectation{
		ev(1, 1, "Fired A E"),
		ev(2, 1, "Moved B S"),
		ev(3, 3, "Hit"), // prefix match, skips unrelated events
		no(4, 2, "Hit"),
		no(5, 1, "Hit"),
	}
	if err := CheckEvents(exps, fakeLog()); err != nil {
		t.Fatal(err)
	}
}

func TestCheckEventsUnorderedWithinBeat(t *testing.T) {
	// file order differs from emission (phase) order
	exps := []Expectation{ev(1, 1, "Fired A E"), ev(2, 1, "Moved B S")}
	if err := CheckEvents(exps, fakeLog()); err != nil {
		t.Fatal(err)
	}
}

func TestCheckEventsDescendingBeatsInFile(t *testing.T) {
	exps := []Expectation{ev(1, 3, "Hit B W"), ev(2, 2, "Moved B S"), ev(3, 1, "Fired A E")}
	if err := CheckEvents(exps, fakeLog()); err != nil {
		t.Fatal(err)
	}
}

func TestCheckEventsRepeatedLinesNeedDistinctEvents(t *testing.T) {
	log := []event.Logged{{Beat: 1, Event: event.Moved{Bot: "B", Dir: geo.S}}}
	exps := []Expectation{ev(1, 1, "Moved B S"), ev(2, 1, "Moved B S")}
	err := CheckEvents(exps, log)
	if err == nil || !strings.Contains(err.Error(), "line 2") || strings.Contains(err.Error(), "line 1:") {
		t.Fatalf("want failure on line 2 only, got %v", err)
	}
	log = append(log, log[0])
	if err := CheckEvents(exps, log); err != nil {
		t.Fatal(err)
	}
}

func TestCheckEventsPrefixDoesNotStealSpecificMatch(t *testing.T) {
	exps := []Expectation{ev(1, 1, "Moved"), ev(2, 1, "Moved B S")}
	log := []event.Logged{
		{Beat: 1, Event: event.Moved{Bot: "B", Dir: geo.S}},
		{Beat: 1, Event: event.Moved{Bot: "A", Dir: geo.E}},
	}
	if err := CheckEvents(exps, log); err != nil {
		t.Fatal(err)
	}
}

func TestCheckEventsWrongBeat(t *testing.T) {
	if err := CheckEvents([]Expectation{ev(7, 2, "Fired A E")}, fakeLog()); err == nil {
		t.Fatal("event from beat 1 matched in beat 2")
	}
}

func TestCheckEventsNoViolated(t *testing.T) {
	err := CheckEvents([]Expectation{no(9, 3, "Hit B")}, fakeLog())
	if err == nil || !strings.Contains(err.Error(), "line 9") {
		t.Fatalf("got %v", err)
	}
}

func TestCheckEventsMessageShowsExpectedAndActual(t *testing.T) {
	err := CheckEvents([]Expectation{ev(3, 2, "Fired A E")}, fakeLog())
	msg := err.Error()
	for _, want := range []string{"beat 2: Fired A E", "actual log", "beat 1: Fired A E", "beat 3: Hit B W"} {
		if !strings.Contains(msg, want) {
			t.Errorf("message lacks %q:\n%s", want, msg)
		}
	}
}

func TestCheckEventsEmptyLog(t *testing.T) {
	err := CheckEvents([]Expectation{ev(1, 1, "Moved")}, nil)
	if err == nil || !strings.Contains(err.Error(), "(no events)") {
		t.Fatalf("got %v", err)
	}
}

func TestCheckEventsReportsAllFailures(t *testing.T) {
	err := CheckEvents([]Expectation{ev(1, 1, "Hit"), no(2, 1, "Fired")}, fakeLog())
	if err == nil || !strings.Contains(err.Error(), "line 1") || !strings.Contains(err.Error(), "line 2") {
		t.Fatalf("got %v", err)
	}
}

func TestGoldenJSONL(t *testing.T) {
	got, err := GoldenJSONL(fakeLog()[:2])
	if err != nil {
		t.Fatal(err)
	}
	want := `{"beat":1,"type":"Moved","args":["B","S"]}` + "\n" + `{"beat":1,"type":"Fired","args":["A","E"]}` + "\n"
	if string(got) != want {
		t.Errorf("got %q", got)
	}
	if empty, _ := GoldenJSONL(nil); len(empty) != 0 {
		t.Errorf("empty log gave %q", empty)
	}
}
