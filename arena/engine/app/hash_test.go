package app

import (
	"strings"
	"testing"

	"github.com/lezhnev/arena/engine/domain/event"
	"github.com/lezhnev/arena/engine/domain/geo"
)

func sampleLog() []event.Logged {
	return []event.Logged{
		{Beat: 1, Event: event.Moved{Bot: "A", Dir: geo.E}},
		{Beat: 2, Event: event.Scanned{Bot: "B", Dir: geo.W, Enemy: 3, Cells: []event.Cell{{X: 1, Y: 2}}}},
	}
}

func TestHashLogPinsTheCanonicalForm(t *testing.T) {
	// Changing this value means the canonical event encoding changed: every
	// recorded hash in the world is invalidated.
	const want = "0bb07e41230c83449fc4dbb312fa3b86e3c2cf89cd74ffe2b2d9dec093734e3f"
	got, err := HashLog(sampleLog())
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Errorf("hash %s, want %s", got, want)
	}
}

func TestHashLogSeesEveryDetail(t *testing.T) {
	a, _ := HashLog(sampleLog())
	other := sampleLog()
	other[1].Beat = 3
	b, _ := HashLog(other)
	other = sampleLog()
	other[1].Event = event.Scanned{Bot: "B", Dir: geo.W, Enemy: 3, Cells: []event.Cell{{X: 2, Y: 2}}}
	c, _ := HashLog(other)
	if a == b || a == c || len(a) != 64 || strings.ToLower(a) != a {
		t.Errorf("hashes %s %s %s", a, b, c)
	}
}

func TestHashLogOfNothingIsStable(t *testing.T) {
	got, _ := HashLog(nil)
	if got != "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855" {
		t.Errorf("empty hash %s", got)
	}
}
