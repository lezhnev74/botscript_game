package sim

import (
	"slices"
	"strconv"
	"testing"

	"github.com/lezhnev/arena/engine/domain/geo"
)

func coinRules(every, max int) Rules {
	r := DefaultRules()
	r.CoinEvery, r.CoinMax = every, max
	return r
}

func coinWorld(t *testing.T) World {
	t.Helper()
	return NewWorld(mustMap(t, "#######\n#A$...#\n#.....#\n#######\n"), DefaultRules(), 1)
}

func TestSteppingOntoACoinTakesIt(t *testing.T) {
	next, evs := resolveA(coinWorld(t), mv(geo.E))
	if !equal(evs, []string{"Moved A E", "CoinTaken A 2 1"}) {
		t.Errorf("events %v", evs)
	}
	if len(next.Coins) != 0 || botA(next).Bonus != 1 {
		t.Errorf("coins %v bonus %d", next.Coins, botA(next).Bonus)
	}
}

func TestCoinAtBonusCapIsStillConsumed(t *testing.T) {
	w := coinWorld(t)
	_ = w.SetBot("A", "bonus", 2)
	next, evs := resolveA(w, mv(geo.E))
	if !slices.Contains(evs, "CoinTaken A 2 1") || len(next.Coins) != 0 || botA(next).Bonus != 2 {
		t.Errorf("events %v coins %v bonus %d", evs, next.Coins, botA(next).Bonus)
	}
}

func TestCoinStaysWhenNobodyStepsOnIt(t *testing.T) {
	next, evs := resolveA(coinWorld(t), mv(geo.S))
	if len(next.Coins) != 1 || !equal(evs, []string{"Moved A S"}) {
		t.Errorf("events %v coins %v", evs, next.Coins)
	}
}

func spawnRun(t *testing.T, r Rules, seed uint64, beats int) (World, []string) {
	t.Helper()
	w := NewWorld(mustMap(t, "#####\n#A..#\n#...#\n#####\n"), r, seed)
	var all []string
	for range beats {
		next, e := Resolve(w, nil, r)
		w = next
		all = append(all, texts(e)...)
	}
	return w, all
}

func TestCoinSpawnsEveryCoinEveryBeats(t *testing.T) {
	r := coinRules(2, 5)
	w, evs := spawnRun(t, r, 1, 1)
	if len(w.Coins) != 0 || len(evs) != 0 {
		t.Fatalf("beat 1: coins %v events %v", w.Coins, evs)
	}
	w, evs = spawnRun(t, r, 1, 2)
	if len(w.Coins) != 1 || len(evs) != 1 {
		t.Fatalf("beat 2: coins %v events %v", w.Coins, evs)
	}
	c := w.Coins[0]
	if want := "CoinSpawned " + itoa(c.X) + " " + itoa(c.Y); evs[0] != want {
		t.Errorf("event %q, want %q", evs[0], want)
	}
}

func itoa(v int) string { return strconv.Itoa(v) }

func TestCoinsStopAtCoinMax(t *testing.T) {
	w, evs := spawnRun(t, coinRules(1, 2), 1, 6)
	if len(w.Coins) != 2 || len(evs) != 2 {
		t.Errorf("coins %v events %v, want exactly 2", w.Coins, evs)
	}
}

func TestMapCoinsCountTowardsCoinMax(t *testing.T) {
	w := coinWorld(t)
	_, evs := Resolve(w, nil, coinRules(1, 1))
	if len(evs) != 0 {
		t.Errorf("spawned over the cap: %v", texts(evs))
	}
}

func TestCoinSpawnCanBeSwitchedOff(t *testing.T) {
	w, evs := spawnRun(t, coinRules(0, 3), 1, 20)
	if len(w.Coins) != 0 || len(evs) != 0 {
		t.Errorf("coins %v events %v", w.Coins, evs)
	}
}

func TestCoinSpawnIsSeeded(t *testing.T) {
	seen := map[geo.Pos]bool{}
	for seed := uint64(1); seed <= 40; seed++ {
		a, _ := spawnRun(t, coinRules(1, 1), seed, 1)
		b, _ := spawnRun(t, coinRules(1, 1), seed, 1)
		if !slices.Equal(a.Coins, b.Coins) {
			t.Fatalf("seed %d not deterministic: %v vs %v", seed, a.Coins, b.Coins)
		}
		seen[a.Coins[0]] = true
	}
	if len(seen) < 3 {
		t.Errorf("40 seeds produced only %d distinct spots", len(seen))
	}
}

func TestCoinNeverSpawnsOnABotOrWall(t *testing.T) {
	for seed := uint64(1); seed <= 40; seed++ {
		w, _ := spawnRun(t, coinRules(1, 1), seed, 1)
		if c := w.Coins[0]; c == pos(1, 1) || w.Grid.Blocked(c) {
			t.Fatalf("seed %d: coin on %v", seed, c)
		}
	}
}

func TestNoCoinWhenThereIsNoFreeCell(t *testing.T) {
	w := NewWorld(mustMap(t, "###\n#A#\n###\n"), coinRules(1, 3), 1)
	_, evs := Resolve(w, nil, coinRules(1, 3))
	if len(evs) != 0 {
		t.Errorf("events %v", texts(evs))
	}
}

func TestBeatCounterAdvances(t *testing.T) {
	w, _ := idle(coinWorld(t))
	if w.Beat != 1 {
		t.Errorf("beat %d", w.Beat)
	}
}
