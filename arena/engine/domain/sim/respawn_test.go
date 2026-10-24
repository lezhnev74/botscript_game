package sim

import (
	"slices"
	"testing"

	"github.com/lezhnev/arena/engine/domain/brain"
	"github.com/lezhnev/arena/engine/domain/geo"
)

func idle(w World) (World, []string) {
	next, evs := resolveAll(w, nil)
	return next, texts(evs)
}

// withFallen returns the world with a dead bot A ready to respawn at once.
func withFallen(t *testing.T, m string) World {
	t.Helper()
	w := NewWorld(mustMap(t, m), DefaultRules(), 1)
	w.Fallen = []Fallen{{Bot: Bot{ID: "A"}, Wait: 0}}
	return w
}

func TestRespawnComesAfterRespawnBeats(t *testing.T) {
	w, _ := resolveA(duelWorld(t, 1), fr(geo.E)) // beat 1: B dies
	for beat := 2; beat <= 5; beat++ {
		var evs []string
		if w, evs = idle(w); len(evs) != 0 || alive(w, "B") {
			t.Fatalf("beat %d: events %v, B alive %v; want B still dead", beat, evs, alive(w, "B"))
		}
	}
	w, evs := idle(w) // beat 6 = 1 + RespawnBeats
	if len(evs) != 1 || !alive(w, "B") || len(w.Fallen) != 0 {
		t.Errorf("beat 6: events %v alive %v", evs, alive(w, "B"))
	}
}

func TestRespawnRestoresFullHPShieldAndLoadedGun(t *testing.T) {
	w := withFallen(t, "########\n#.....B#\n########\n")
	w.Fallen[0].Bot = Bot{ID: "A", Cooldown: 2, Bonus: 1, Read: brain.Readings{Enemy: 3}}
	w, _ = idle(w)
	a := bot(w, "A")
	if a.HP != 4 || a.Shield != 2 || a.Cooldown != 0 || a.Bonus != 0 || a.Read.Enemy != 0 || !a.Last.OK {
		t.Errorf("respawned bot %+v", a)
	}
}

func TestRespawnEventHasThePosition(t *testing.T) {
	w := withFallen(t, "########\n#.....B#\n########\n")
	_, evs := idle(w)
	if !equal(evs, []string{"Respawned A 1 1"}) {
		t.Errorf("events %v", evs)
	}
}

func TestRespawnSpotIsFarthestFromEnemies(t *testing.T) {
	w := withFallen(t, "########\n#.....B#\n########\n")
	w, _ = idle(w)
	if got := bot(w, "A").Pos; got != pos(1, 1) {
		t.Errorf("spawned at %v, want (1,1)", got)
	}
}

func TestRespawnSpotUsesTheNearestEnemy(t *testing.T) {
	w := withFallen(t, "#########\n#B.....C#\n#########\n")
	w.Fallen[0].Bot.ID = "A"
	w, _ = idle(w)
	if got := bot(w, "A").Pos; got != pos(4, 1) {
		t.Errorf("spawned at %v, want the middle (4,1)", got)
	}
}

func spawnX(t *testing.T, seed uint64) int {
	t.Helper()
	w := withFallen(t, "#####\n#.B.#\n#####\n")
	w.RNG = newRNG(seed).state
	w, _ = idle(w)
	return bot(w, "A").Pos.X
}

func TestRespawnTieIsBrokenBySeed(t *testing.T) {
	seen := map[int]bool{}
	for seed := uint64(1); seed <= 30; seed++ {
		x := spawnX(t, seed)
		if x != 1 && x != 3 {
			t.Fatalf("seed %d: spawned at x=%d, want a tied cell", seed, x)
		}
		if again := spawnX(t, seed); again != x {
			t.Fatalf("seed %d is not deterministic: %d vs %d", seed, x, again)
		}
		seen[x] = true
	}
	if len(seen) != 2 {
		t.Errorf("30 seeds only ever chose %v", seen)
	}
}

func TestRespawnAvoidsCoinsAndBullets(t *testing.T) {
	w := withFallen(t, "########\n#$....B#\n########\n")
	w.Bullets = []Bullet{{Pos: pos(5, 1), Dir: geo.W}} // ends the beat on (2,1)
	w, _ = idle(w)
	if got := bot(w, "A").Pos; got != pos(3, 1) {
		t.Errorf("spawned at %v, want (3,1)", got)
	}
}

func TestRespawnWaitsForAFreeCell(t *testing.T) {
	w := withFallen(t, "###\n#B#\n###\n")
	w, evs := idle(w)
	if len(evs) != 0 || len(w.Fallen) != 1 || alive(w, "A") {
		t.Errorf("events %v fallen %v", evs, w.Fallen)
	}
}

func TestRespawnedBotsKeepBotsSorted(t *testing.T) {
	w := withFallen(t, "#######\n#B...C#\n#######\n")
	w, _ = idle(w)
	ids := []string{w.Bots[0].ID, w.Bots[1].ID, w.Bots[2].ID}
	if !slices.IsSorted(ids) {
		t.Errorf("bots %v not sorted", ids)
	}
}

func TestShieldTicksDownAfterRespawn(t *testing.T) {
	w := withFallen(t, "########\n#.....B#\n########\n")
	var got []int
	for range 3 {
		w, _ = idle(w)
		got = append(got, bot(w, "A").Shield)
	}
	if !slices.Equal(got, []int{2, 1, 0}) {
		t.Errorf("shield after each beat %v, want [2 1 0]", got)
	}
}

func TestShieldAbsorbsBullets(t *testing.T) {
	w := NewWorld(mustMap(t, shootoutMap), DefaultRules(), 1)
	_ = w.SetBot("B", "shield", 1)
	next, evs := resolveA(w, fr(geo.E))
	if !equal(evs, []string{"Fired A E"}) || bot(next, "B").HP != 4 || len(next.Bullets) != 0 {
		t.Errorf("events %v hp %d bullets %v", evs, bot(next, "B").HP, next.Bullets)
	}
	if next.Bots[1].Last.Hit {
		t.Error("shielded bot must not see a hit")
	}
}

func TestRespawnDoesNotMutateTheInputWorld(t *testing.T) {
	w := withFallen(t, "########\n#.....B#\n########\n")
	w.Fallen[0].Wait = 3
	_, _ = idle(w)
	if w.Fallen[0].Wait != 3 || len(w.Bots) != 1 {
		t.Errorf("input changed: %+v", w)
	}
}

func TestRespawnCountsLivesInThePerception(t *testing.T) {
	w := withFallen(t, "########\n#.....B#\n########\n")
	w.Fallen[0].Bot.Life = 2
	if p := w.Perception("B", 1); p.Life != 0 {
		t.Errorf("Life before any respawn = %d", p.Life)
	}
	w, _ = idle(w)
	if p := w.Perception("A", 2); p.Life != 3 {
		t.Errorf("Life after the third respawn = %d, want 3", p.Life)
	}
}

func TestLifeSurvivesDeathAndRespawnWithoutWaiting(t *testing.T) {
	rules := DefaultRules()
	rules.RespawnBeats = 0
	w2 := duelWorld(t, 1)
	w2, _ = Resolve(w2, map[string]brain.Action{"A": fr(geo.E)}, rules)
	if p := w2.Perception("B", 2); p.Life != 1 {
		t.Errorf("RespawnBeats=0: Life = %d, want 1 on the first beat after the death", p.Life)
	}
}
