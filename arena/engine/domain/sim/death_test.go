package sim

import (
	"slices"
	"testing"

	"github.com/lezhnev/arena/engine/domain/brain"
	"github.com/lezhnev/arena/engine/domain/geo"
)

const shootoutMap = "#####\n#A.B#\n#####\n"

func duelWorld(t *testing.T, hp int) World {
	t.Helper()
	w := NewWorld(mustMap(t, shootoutMap), DefaultRules(), 1)
	if err := w.SetBot("B", "hp", hp); err != nil {
		t.Fatal(err)
	}
	return w
}

func alive(w World, id string) bool {
	_, ok := w.bot(id)
	return ok
}

func TestLethalHitKillsAndEmitsKilled(t *testing.T) {
	next, evs := resolveA(duelWorld(t, 1), fr(geo.E))
	if !equal(evs, []string{"Fired A E", "Hit B W", "Killed A B"}) {
		t.Errorf("events %v", evs)
	}
	if alive(next, "B") {
		t.Error("B must be off the grid")
	}
	if f, ok := next.fallen("B"); !ok || f.Bot.HP != 0 {
		t.Errorf("fallen %+v", next.Fallen)
	}
}

func TestNonLethalHitDoesNotKill(t *testing.T) {
	next, evs := resolveA(duelWorld(t, 2), fr(geo.E))
	if !equal(evs, []string{"Fired A E", "Hit B W"}) || !alive(next, "B") {
		t.Errorf("events %v alive=%v", evs, alive(next, "B"))
	}
}

func TestDeathLosesBonusShots(t *testing.T) {
	w := duelWorld(t, 1)
	_ = w.SetBot("B", "bonus", 2)
	next, _ := resolveA(w, fr(geo.E))
	if f, _ := next.fallen("B"); f.Bot.Bonus != 0 {
		t.Errorf("bonus kept: %d", f.Bot.Bonus)
	}
}

func TestDeathWaitsRespawnBeats(t *testing.T) {
	next, _ := resolveA(duelWorld(t, 1), fr(geo.E))
	if f, _ := next.fallen("B"); f.Wait != DefaultRules().RespawnBeats-1 {
		t.Errorf("wait %d after the death beat", f.Wait)
	}
}

func TestOnlyFirstLethalHitKills(t *testing.T) {
	w := NewWorld(mustMap(t, "#######\n#A.B.C#\n#######\n"), DefaultRules(), 1)
	_ = w.SetBot("B", "hp", 1)
	_, evs := Resolve(w, map[string]brain.Action{"A": fr(geo.E), "C": fr(geo.W)}, DefaultRules())
	got := slices.DeleteFunc(texts(evs), func(s string) bool { return s != "Killed A B" && s != "Killed C B" })
	if !equal(got, []string{"Killed A B"}) {
		t.Errorf("kills %v, want only A's (first lethal hit)", got)
	}
}

func TestSelfKillCreditsTheVictim(t *testing.T) {
	w := NewWorld(mustMap(t, shootoutMap), DefaultRules(), 1)
	_ = w.SetBot("B", "hp", 1)
	w.Bullets = []Bullet{{Pos: pos(2, 1), Dir: geo.E, Owner: "B"}} // B steps into its own bullet
	_, evs := resolveAll(w, map[string]brain.Action{"B": {Kind: brain.Move, Dir: geo.W}})
	if !slices.Contains(texts(evs), "Killed B B") {
		t.Errorf("events %v, want Killed B B", texts(evs))
	}
}

func TestDeadBotIsNotBlockingOrHittable(t *testing.T) {
	next, _ := resolveA(duelWorld(t, 1), fr(geo.E))
	next, evs := resolveAll(next, map[string]brain.Action{"A": {Kind: brain.Move, Dir: geo.E}})
	if !equal(texts(evs), []string{"Moved A E"}) || botA(next).Pos != pos(2, 1) {
		t.Errorf("events %v", texts(evs))
	}
	_ = next.SetBot("A", "cooldown", 0)
	_, evs = resolveAll(next, map[string]brain.Action{"A": fr(geo.E)})
	if !equal(texts(evs), []string{"Fired A E"}) {
		t.Errorf("bullet hit a dead bot: %v", texts(evs))
	}
}

func TestNoHitOnABotThatAlreadyDiedThisBeat(t *testing.T) {
	w := NewWorld(mustMap(t, "#######\n#A.B.C#\n#######\n"), DefaultRules(), 1)
	_ = w.SetBot("B", "hp", 1)
	_, evs := Resolve(w, map[string]brain.Action{"A": fr(geo.E), "C": fr(geo.W)}, DefaultRules())
	if !equal(texts(evs), []string{"Fired A E", "Fired C W", "Hit B W", "Killed A B"}) {
		t.Errorf("events %v", texts(evs))
	}
}
