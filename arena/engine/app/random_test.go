package app

import (
	"reflect"
	"testing"

	"github.com/lezhnev/arena/engine/domain/geo"
	"github.com/lezhnev/arena/engine/domain/sim"
)

func TestRandomRecordIsSeeded(t *testing.T) {
	p := RandomParams{W: 7, H: 6, Bots: 3, Beats: 20}
	a, b := RandomRecord(5, p), RandomRecord(5, p)
	if !reflect.DeepEqual(a, b) {
		t.Error("same seed, different record")
	}
	if reflect.DeepEqual(a, RandomRecord(6, p)) {
		t.Error("different seeds, same record")
	}
}

func TestRandomRecordShape(t *testing.T) {
	rec := RandomRecord(9, RandomParams{W: 8, H: 5, Bots: 4, Beats: 12})
	m, err := geo.ParseMap(rec.Map)
	if err != nil {
		t.Fatal(err)
	}
	if m.Grid.Width() != 8 || m.Grid.Height() != 5 || len(m.Bots) != 4 || len(rec.Bots) != 4 || rec.Beats != 12 {
		t.Errorf("shape %dx%d bots %d beats %d", m.Grid.Width(), m.Grid.Height(), len(m.Bots), rec.Beats)
	}
}

func TestRandomRecordReplays(t *testing.T) {
	if _, err := Replay(RandomRecord(9, RandomParams{W: 8, H: 5, Bots: 4, Beats: 12})); err != nil {
		t.Errorf("record does not replay: %v", err)
	}
}

func TestRandomRecordClampsParams(t *testing.T) {
	rec := RandomRecord(1, RandomParams{W: 1, H: 99, Bots: 0, Beats: -3})
	m, err := geo.ParseMap(rec.Map)
	if err != nil {
		t.Fatal(err)
	}
	if m.Grid.Width() != 5 || m.Grid.Height() != 9 || len(m.Bots) != 2 || rec.Beats != 0 {
		t.Errorf("clamped to %dx%d bots %d beats %d", m.Grid.Width(), m.Grid.Height(), len(m.Bots), rec.Beats)
	}
}

func TestRandomRecordOnScriptsEveryMapBot(t *testing.T) {
	rec, err := RandomRecordOn(3, sim.DefaultRules(), "#####\n#A.B#\n#C..#\n#####\n", 9)
	if err != nil || len(rec.Bots) != 3 || rec.Bots[2].ID != "C" || rec.Beats != 9 {
		t.Fatalf("got %+v, %v", rec, err)
	}
	if rec.Rules != sim.DefaultRules() {
		t.Errorf("rules %+v", rec.Rules)
	}
	if _, err := RandomRecordOn(3, sim.DefaultRules(), "#A\n#", 9); err == nil {
		t.Error("want a map error")
	}
}

func TestRandomMapIsSeededAndClamped(t *testing.T) {
	if RandomMap(4, 7, 7, 3) != RandomMap(4, 7, 7, 3) || RandomMap(4, 7, 7, 3) == RandomMap(5, 7, 7, 3) {
		t.Error("RandomMap is not a pure function of its seed")
	}
	m, err := geo.ParseMap(RandomMap(1, 2, 20, 9))
	if err != nil || m.Grid.Width() != 5 || m.Grid.Height() != 9 || len(m.Bots) != 4 {
		t.Errorf("clamped map: %v %v", m.Grid, err)
	}
}

func TestRandomRulesVaryTheBonusShieldAndCoinRules(t *testing.T) {
	seen := map[string]map[int]bool{"bonus": {}, "shield": {}, "coins": {}}
	for seed := uint64(0); seed < 60; seed++ {
		r := RandomRecord(seed, RandomParams{W: 5, H: 5, Bots: 2}).Rules
		seen["bonus"][r.BonusMax], seen["shield"][r.ShieldBeats], seen["coins"][r.CoinMax] = true, true, true
	}
	for name, vals := range seen {
		if len(vals) < 2 {
			t.Errorf("%s is never varied: %v", name, vals)
		}
	}
}

func TestRandomRecordMixesScriptsHouseProgramsAndEdits(t *testing.T) {
	programs, scripts, edits := 0, 0, 0
	for seed := uint64(0); seed < 40; seed++ {
		rec := RandomRecord(seed, RandomParams{W: 7, H: 7, Bots: 3, Beats: 30})
		p, s := countKinds(rec)
		programs, scripts, edits = programs+p, scripts+s, edits+len(rec.EditLog)
		checkEdits(t, seed, rec)
		if _, err := Replay(rec); err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
	}
	if programs == 0 || scripts == 0 || edits == 0 {
		t.Errorf("programs %d, scripts %d, edits %d: want some of each", programs, scripts, edits)
	}
}

func countKinds(rec Record) (programs, scripts int) {
	for _, b := range rec.Bots {
		if b.Brain.Kind == KindProgram {
			programs++
		} else {
			scripts++
		}
	}
	return programs, scripts
}

func checkEdits(t *testing.T, seed uint64, rec Record) {
	t.Helper()
	for _, e := range rec.EditLog {
		if e.Beat < 1 || e.Beat > rec.Beats || e.Brain.Kind != KindProgram {
			t.Fatalf("seed %d: bad edit %+v", seed, e)
		}
	}
}

func TestRandomRecordWithoutBeatsHasNoEdits(t *testing.T) {
	for seed := uint64(0); seed < 40; seed++ {
		if rec := RandomRecord(seed, RandomParams{Beats: 0}); len(rec.EditLog) != 0 {
			t.Fatalf("seed %d: edits %+v", seed, rec.EditLog)
		}
	}
}

func TestRandomRecordOnStaysScriptOnly(t *testing.T) {
	rec, err := RandomRecordOn(3, sim.DefaultRules(), "#####\n#A.B#\n#####\n", 9)
	if err != nil || rec.Bots[0].Brain.Kind != KindScript || len(rec.EditLog) != 0 {
		t.Errorf("got %+v, %v", rec, err)
	}
}

func TestRandomRecordsHaveJoinsAndLeavesThatReplay(t *testing.T) {
	var joins, leaves int
	for seed := uint64(1); seed <= 60; seed++ {
		rec := RandomRecord(seed, RandomParams{W: 9, H: 9, Bots: 2 + int(seed%3), Beats: 100})
		joins, leaves = joins+len(rec.Joins), leaves+len(rec.Leaves)
		if _, err := Replay(rec); err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
	}
	if joins < 10 || leaves < 10 {
		t.Errorf("only %d joins and %d leaves over 60 seeds", joins, leaves)
	}
}

func TestSmallRoomsGetNoJoins(t *testing.T) {
	for seed := uint64(1); seed <= 20; seed++ {
		if rec := RandomRecord(seed, RandomParams{W: 5, H: 5, Bots: 4, Beats: 50}); len(rec.Joins) != 0 {
			t.Fatalf("seed %d: a 5x5 room with 4 bots got %d joins", seed, len(rec.Joins))
		}
	}
}

func TestPlaceBotsPutsLettersOnFreeFloorReproducibly(t *testing.T) {
	room := "#####\n#...#\n#.#.#\n#..$#\n#####\n"
	a, err := PlaceBots(room, 4, 3)
	if err != nil {
		t.Fatal(err)
	}
	if b, _ := PlaceBots(room, 4, 3); a != b {
		t.Error("same seed, same placement")
	}
	if c, _ := PlaceBots(room, 5, 3); a == c {
		t.Error("another seed should place differently")
	}
	m, err := geo.ParseMap(a)
	if err != nil || len(m.Bots) != 3 || len(m.Coins) != 1 {
		t.Errorf("bots %v coins %v err %v\n%s", m.Bots, m.Coins, err, a)
	}
}

func TestPlaceBotsKeepsAMapThatHasBotsAndRejectsBadRequests(t *testing.T) {
	withBots := "#####\n#A.B#\n#####\n"
	if got, err := PlaceBots(withBots, 1, 5); err != nil || got != withBots {
		t.Errorf("%q %v", got, err)
	}
	for name, c := range map[string]struct {
		room string
		n    int
	}{"too many": {"#####\n#..##\n#####\n", 3}, "none": {"#####\n#..##\n#####\n", 0}, "over 26": {"#####\n#..##\n#####\n", 27}, "bad map": {"#A\n#", 2}} {
		if _, err := PlaceBots(c.room, 1, c.n); err == nil {
			t.Errorf("%s: want an error", name)
		}
	}
}
