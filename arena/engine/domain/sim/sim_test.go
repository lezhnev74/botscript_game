package sim

import (
	"reflect"
	"testing"

	"github.com/lezhnev/arena/engine/domain/brain"
	"github.com/lezhnev/arena/engine/domain/geo"
)

func mustMap(t *testing.T, s string) geo.Map {
	t.Helper()
	m, err := geo.ParseMap(s)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func TestDefaultRules(t *testing.T) {
	want := Rules{BulletSpeed: 3, Reload: 3, HP: 4, BonusMax: 2, RespawnBeats: 5, ShieldBeats: 2, CoinEvery: 50, CoinMax: 5, CoinHeal: 1, ConeSlope: 100, HearRange: 6}
	if got := DefaultRules(); got != want {
		t.Errorf("defaults %+v, want %+v", got, want)
	}
}

func TestRulesSet(t *testing.T) {
	r := DefaultRules()
	if err := r.Set("reload", 5); err != nil || r.Reload != 5 {
		t.Errorf("Set reload: %v %+v", err, r)
	}
	if err := r.Set("bogus", 1); err == nil {
		t.Error("unknown key accepted")
	}
}

func TestNewWorldKeepsMaxHPAndReloadForViews(t *testing.T) {
	r := DefaultRules()
	r.HP, r.Reload = 7, 5
	if w := NewWorld(mustMap(t, "###\n#A#\n###\n"), r, 1); w.MaxHP != 7 || w.Bots[0].HP != 7 || w.Reload != 5 {
		t.Errorf("maxHp %d, bot hp %d, reload %d", w.MaxHP, w.Bots[0].HP, w.Reload)
	}
}

func TestNewWorldAndRender(t *testing.T) {
	m := mustMap(t, "####\n#A$#\n#B.#\n####\n")
	w := NewWorld(m, DefaultRules(), 7)
	if len(w.Bots) != 2 || w.Bots[0].ID != "A" || w.Bots[1].ID != "B" {
		t.Fatalf("bots: %+v", w.Bots)
	}
	if w.Bots[0].HP != 4 || w.Bots[0].Pos != (geo.Pos{X: 1, Y: 1}) || w.Seed != 7 {
		t.Errorf("bot A: %+v seed %d", w.Bots[0], w.Seed)
	}
}

func TestRender(t *testing.T) {
	m := mustMap(t, "####\n#A$#\n#B.#\n####\n")
	if got := NewWorld(m, DefaultRules(), 7).Render(); got != m.Render() {
		t.Errorf("render %q, want %q", got, m.Render())
	}
}

func TestQuery(t *testing.T) {
	w := NewWorld(mustMap(t, "####\n#A.#\n####\n"), DefaultRules(), 1)
	for spec, want := range map[string]int{"A.x": 1, "A.y": 1, "A.hp": 4} {
		if got, err := w.Query(spec); err != nil || got != want {
			t.Errorf("Query(%q) = %d, %v; want %d", spec, got, err, want)
		}
	}
	for _, bad := range []string{"A", "Z.hp", "A.mana", "A.hp.x"} {
		if _, err := w.Query(bad); err == nil {
			t.Errorf("Query(%q) accepted", bad)
		}
	}
}

func TestQueryHit(t *testing.T) {
	w := NewWorld(mustMap(t, "####\n#A.#\n####\n"), DefaultRules(), 1)
	w.Bots[0].Last = brain.Outcome{Hit: true, HitDir: geo.W}
	if got, err := w.Query("A.hit"); err != nil || got != int(geo.W) {
		t.Errorf("Query(A.hit) = %d, %v; want W", got, err)
	}
	w.Bots[0].Last = brain.Outcome{}
	if got, err := w.Query("A.hit"); err != nil || got != -1 {
		t.Errorf("Query(A.hit) = %d, %v; want -1", got, err)
	}
}

func TestPerception(t *testing.T) {
	w := NewWorld(mustMap(t, "####\n#A.#\n####\n"), DefaultRules(), 1)
	p := w.Perception("A", 4)
	if p.Grid.Width() != 4 || p.Grid.Height() != 3 || !p.Grid.IsWall(geo.Pos{X: 0, Y: 0}) || p.Grid.IsWall(geo.Pos{X: 2, Y: 1}) {
		t.Errorf("grid %+v", p.Grid)
	}
	p.Grid = geo.Grid{}
	if !reflect.DeepEqual(p, brain.Perception{Beat: 4, Pos: geo.Pos{X: 1, Y: 1}, HP: 4, Last: brain.Outcome{OK: true}, Loaded: true,
		Width: 4, Height: 3, Open: [4]bool{false, true, false, false}}) {
		t.Errorf("perception %+v", p)
	}
}

func TestResolveWaitChangesNothing(t *testing.T) {
	w := NewWorld(mustMap(t, "####\n#A.#\n####\n"), DefaultRules(), 1)
	acts := map[string]brain.Action{"A": {Kind: brain.Wait}}
	w2, evs := Resolve(w, acts, DefaultRules())
	if len(evs) != 0 || w2.Render() != w.Render() {
		t.Errorf("stub changed things: %v\n%s", evs, w2.Render())
	}
}

func TestGlyph(t *testing.T) {
	if g := (Bot{ID: "A"}).Glyph(); g != 'A' {
		t.Errorf("single letter glyph %q", g)
	}
	for _, id := range []string{"", "AB"} {
		if g := (Bot{ID: id}).Glyph(); g != '?' {
			t.Errorf("id %q glyph %q, want '?'", id, g)
		}
	}
}

func TestQueryAliveAndShield(t *testing.T) {
	w := NewWorld(mustMap(t, shootoutMap), DefaultRules(), 1)
	_ = w.SetBot("B", "shield", 2)
	for spec, want := range map[string]int{"A.alive": 1, "B.alive": 1, "B.shield": 2, "A.shield": 0} {
		if got, err := w.Query(spec); err != nil || got != want {
			t.Errorf("%s = %d, %v; want %d", spec, got, err, want)
		}
	}
}

func TestQueryDeadBot(t *testing.T) {
	w, _ := resolveA(duelWorld(t, 1), fr(geo.E))
	for spec, want := range map[string]int{"B.alive": 0, "B.hp": 0, "B.bonus": 0, "B.shield": 0} {
		if got, err := w.Query(spec); err != nil || got != want {
			t.Errorf("%s = %d, %v; want %d", spec, got, err, want)
		}
	}
	if _, err := w.Query("Z.alive"); err == nil {
		t.Error("unknown bot accepted")
	}
}

func TestRenderOmitsDeadBots(t *testing.T) {
	w, _ := resolveA(duelWorld(t, 1), fr(geo.E))
	if got, want := w.Render(), "#####\n#A..#\n#####\n"; got != want {
		t.Errorf("render %q, want %q", got, want)
	}
}

func TestRulesSetRejectsOutOfRangeValues(t *testing.T) {
	min := map[string]int{
		"bullet_speed": 1, "reload": 1, "hp": 1, "respawn_beats": 1,
		"bonus_max": 0, "shield_beats": 0, "coin_every": 0, "coin_max": 0, "coin_heal": 0, "cone_slope": 0, "hear_range": 0,
	}
	if len(min) != len(RuleKeys()) {
		t.Fatalf("a minimum is needed for every rule: %v", RuleKeys())
	}
	for key, lo := range min {
		r := DefaultRules()
		if err := r.Set(key, lo); err != nil {
			t.Errorf("%s=%d should be accepted: %v", key, lo, err)
		}
		before := r
		if err := r.Set(key, lo-1); err == nil || r != before {
			t.Errorf("%s=%d should be rejected and leave the rules alone (err %v)", key, lo-1, err)
		}
	}
}
