package sim

import (
	"slices"
	"testing"

	"github.com/lezhnev/arena/engine/domain/brain"
	"github.com/lezhnev/arena/engine/domain/geo"
)

func fr(d geo.Dir) brain.Action { return brain.Action{Kind: brain.Fire, Dir: d} }

func fireWorld(t *testing.T) World {
	t.Helper()
	return NewWorld(mustMap(t, "#####\n#.A.#\n#...#\n#####\n"), DefaultRules(), 1)
}

func resolveA(w World, a brain.Action) (World, []string) {
	next, evs := Resolve(w, map[string]brain.Action{"A": a}, DefaultRules())
	return next, texts(evs)
}

func botA(w World) Bot {
	b, _ := w.bot("A")
	return b
}

func TestBotsStartLoaded(t *testing.T) {
	w := fireWorld(t)
	p := w.Perception("A", 1)
	if !p.Loaded || p.Bonus != 0 {
		t.Errorf("start perception loaded=%v bonus=%d", p.Loaded, p.Bonus)
	}
}

func TestFireLaunchesBulletAndStartsReload(t *testing.T) {
	next, evs := resolveA(corridorWorld(t), fr(geo.E))
	if !equal(evs, []string{"Fired A E"}) {
		t.Fatalf("events %v", evs)
	}
	want := []Bullet{{Pos: geo.Pos{X: 4, Y: 1}, Dir: geo.E, Owner: "A"}} // spawned at (1,1), flew 3
	if !slices.Equal(next.Bullets, want) {
		t.Errorf("bullets %+v, want %+v", next.Bullets, want)
	}
	if got := botA(next); got.Cooldown != 2 || !got.Last.OK {
		t.Errorf("after fire cooldown=%d last=%+v (want 2: Reload 3 minus this beat)", got.Cooldown, got.Last)
	}
	if next.Perception("A", 2).Loaded {
		t.Error("gun must be empty right after firing")
	}
}

func TestFireWithNothingLoadedClicks(t *testing.T) {
	w := fireWorld(t)
	w.Bots[0].Cooldown = 2
	next, evs := resolveA(w, fr(geo.W))
	if !equal(evs, []string{"Clicked A W"}) {
		t.Fatalf("events %v", evs)
	}
	if len(next.Bullets) != 0 || botA(next).Last.OK {
		t.Errorf("click: bullets %v last %+v", next.Bullets, botA(next).Last)
	}
	if botA(next).Cooldown != 1 {
		t.Errorf("reload must keep counting on a click, got %d", botA(next).Cooldown)
	}
}

func TestReloadCountsDownEveryBeatAndStopsAtZero(t *testing.T) {
	w := fireWorld(t)
	w, _ = resolveA(w, fr(geo.E))
	var got []int
	for range 4 {
		w, _ = resolveA(w, brain.Action{})
		got = append(got, botA(w).Cooldown)
	}
	if !slices.Equal(got, []int{1, 0, 0, 0}) {
		t.Errorf("cooldowns %v", got)
	}
	if !w.Perception("A", 6).Loaded {
		t.Error("gun must be loaded again")
	}
}

func TestFireEveryReloadBeats(t *testing.T) {
	w := fireWorld(t)
	var log []string
	for range 7 {
		var evs []string
		w, evs = resolveA(w, fr(geo.E))
		log = append(log, evs...)
	}
	want := []string{"Fired A E", "Clicked A E", "Clicked A E", "Fired A E", "Clicked A E", "Clicked A E", "Fired A E"}
	if !equal(log, want) {
		t.Errorf("sequence %v", log)
	}
}

func TestBonusFiresWhileReloadingWithoutTouchingCooldown(t *testing.T) {
	w := fireWorld(t)
	w.Bots[0].Cooldown, w.Bots[0].Bonus = 2, 2
	next, evs := resolveA(w, fr(geo.N))
	if !equal(evs, []string{"Fired A N"}) {
		t.Fatalf("events %v", evs)
	}
	if b := botA(next); b.Bonus != 1 || b.Cooldown != 1 || !b.Last.OK {
		t.Errorf("bonus shot: %+v", b)
	}
}

func TestLoadedGunSpendsTheRegularShotBeforeBonus(t *testing.T) {
	w := fireWorld(t)
	w.Bots[0].Bonus = 1
	next, _ := resolveA(w, fr(geo.N))
	if b := botA(next); b.Bonus != 1 || b.Cooldown != 2 {
		t.Errorf("regular shot first: %+v", b)
	}
}

func TestLoadedMeansReloadedOrBonusStored(t *testing.T) {
	b := Bot{Cooldown: 2}
	if b.Loaded() {
		t.Error("reloading with no bonus is not loaded")
	}
	b.Bonus = 1
	if !b.Loaded() {
		t.Error("a stored bonus shot makes the gun ready")
	}
	if p := (World{Bots: []Bot{b}}).Perception("", 1); !p.Loaded || p.Bonus != 1 {
		t.Errorf("perception %+v", p)
	}
}

func TestAddBonusIsCapped(t *testing.T) {
	b := Bot{}
	for range 5 {
		b.AddBonus(2)
	}
	if b.Bonus != 2 {
		t.Errorf("bonus %d, want cap 2", b.Bonus)
	}
}

func TestFireWithInvalidDirectionIsANoOp(t *testing.T) {
	next, evs := resolveA(fireWorld(t), fr(geo.Dir(9)))
	if len(evs) != 0 || len(next.Bullets) != 0 || botA(next).Last.OK || botA(next).Cooldown != 0 {
		t.Errorf("events %v bullets %v bot %+v", evs, next.Bullets, botA(next))
	}
}

func TestFireDoesNotMutateInput(t *testing.T) {
	w := fireWorld(t)
	w.Bots[0].Bonus = 1
	bots, bullets := slices.Clone(w.Bots), slices.Clone(w.Bullets)
	resolveA(w, fr(geo.E))
	if !slices.Equal(w.Bots, bots) || !slices.Equal(w.Bullets, bullets) {
		t.Errorf("input world mutated: %+v", w)
	}
}

func TestQueryReadsShootingState(t *testing.T) {
	w := fireWorld(t)
	w.Bots[0].Cooldown, w.Bots[0].Bonus = 2, 1
	for spec, want := range map[string]int{"A.cooldown": 2, "A.bonus": 1, "A.loaded": 1} {
		if got, err := w.Query(spec); err != nil || got != want {
			t.Errorf("Query(%q) = %d, %v; want %d", spec, got, err, want)
		}
	}
}

func TestSetBotState(t *testing.T) {
	w := fireWorld(t)
	for _, f := range []struct {
		field string
		v     int
	}{{"hp", 2}, {"bonus", 1}, {"cooldown", 3}} {
		if err := w.SetBot("A", f.field, f.v); err != nil {
			t.Fatal(err)
		}
		if got, _ := w.Query("A." + f.field); got != f.v {
			t.Errorf("%s = %d, want %d", f.field, got, f.v)
		}
	}
	if w.SetBot("A", "x", 1) == nil || w.SetBot("Z", "hp", 1) == nil {
		t.Error("only hp, bonus, cooldown of existing bots are settable")
	}
}
