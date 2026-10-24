package sim

import (
	"slices"
	"strings"
	"testing"
)

func TestAdmitPlacesTheNewBotFarthestFromOthers(t *testing.T) {
	w := NewWorld(mustMap(t, "#######\n#A....#\n#######\n"), DefaultRules(), 1)
	next, ok := w.Admit("p1", DefaultRules())
	b, found := next.bot("p1")
	if !ok || !found || b.Pos.X != 5 || b.HP != DefaultRules().HP || b.Shield != 2 || !b.Last.OK {
		t.Errorf("admitted %v: bot %+v", ok, b)
	}
}

func TestAdmitKeepsBotsSortedAndTheInputUntouched(t *testing.T) {
	w := NewWorld(mustMap(t, "#######\n#A....#\n#######\n"), DefaultRules(), 1)
	next, _ := w.Admit("0", DefaultRules())
	if len(w.Bots) != 1 || next.RNG == w.RNG {
		t.Error("Admit must not touch the input world and must draw from the RNG")
	}
	if !slices.IsSortedFunc(next.Bots, func(a, b Bot) int { return strings.Compare(a.ID, b.ID) }) {
		t.Errorf("bots not sorted: %+v", next.Bots)
	}
}

func TestAdmitIntoAnEmptyArenaWorks(t *testing.T) {
	w := NewWorld(mustMap(t, "####\n#..#\n####\n"), DefaultRules(), 3)
	if next, ok := w.Admit("p1", DefaultRules()); !ok || len(next.Bots) != 1 {
		t.Errorf("ok %v bots %v", ok, next.Bots)
	}
}

func TestAdmitFailsWithoutAFreeCell(t *testing.T) {
	w := NewWorld(mustMap(t, "###\n#A#\n###\n"), DefaultRules(), 1)
	if _, ok := w.Admit("p1", DefaultRules()); ok {
		t.Error("admitted into a full arena")
	}
}

func TestRemoveDropsLiveAndFallenBots(t *testing.T) {
	w := NewWorld(mustMap(t, "#####\n#AB.#\n#####\n"), DefaultRules(), 1)
	w.Fallen = []Fallen{{Bot: Bot{ID: "C"}}}
	next := w.Remove("A").Remove("C")
	if len(next.Bots) != 1 || next.Bots[0].ID != "B" || len(next.Fallen) != 0 {
		t.Errorf("after remove: %+v / %+v", next.Bots, next.Fallen)
	}
	if len(w.Bots) != 2 || len(w.Fallen) != 1 {
		t.Error("Remove changed its input")
	}
}
