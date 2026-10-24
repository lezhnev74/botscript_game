package app_test

import (
	"errors"
	"reflect"
	"slices"
	"testing"

	"github.com/lezhnev/arena/engine/app"
	"github.com/lezhnev/arena/engine/domain/event"
	"github.com/lezhnev/arena/engine/domain/geo"
)

const bigRoom = "###########\n#.........#\n#.........#\n#....#....#\n#.........#\n#.........#\n#.........#\n#.........#\n###########\n"

func TestApplyMapWaitsForTheNextBeat(t *testing.T) {
	r := newRig(t, 0)
	r.join(t, "Ann")
	r.tick(t, 3)
	if err := r.a.ApplyMap(bigRoom); err != nil {
		t.Fatal(err)
	}
	if w := r.a.World(); w.Beat != 3 || w.Grid.Width() != 9 {
		t.Fatalf("map changed before the beat: beat %d width %d", w.Beat, w.Grid.Width())
	}
	r.tick(t, 1)
	u := r.feed.Last()
	if u.Beat != 1 || u.World.Grid.Width() != 11 || u.World.Grid.Height() != 9 {
		t.Errorf("new game: beat %d, %dx%d", u.Beat, u.World.Grid.Width(), u.World.Grid.Height())
	}
}

func TestApplyMapKeepsPlayersTokensAndPrograms(t *testing.T) {
	r := newRig(t, 0)
	token, id := r.join(t, "Ann")
	if err := r.a.PutBot(token, walker); err != nil {
		t.Fatal(err)
	}
	r.tick(t, 5)
	applyAndTick(t, r, bigRoom)
	u := r.feed.Last()
	if r.roster() != id || len(u.World.Bots) != 1 {
		t.Fatalf("roster %q, bots %d", r.roster(), len(u.World.Bots))
	}
	if v, err := r.a.GetBot(token); err != nil || v.Source != walker || v.ID != id {
		t.Errorf("bot view %+v err %v", v, err)
	}
}

func TestApplyMapGivesFreshBotsAndKeepsTheBoard(t *testing.T) {
	r := newRig(t, 0)
	r.join(t, "Ann")
	r.tick(t, 200)
	before := r.a.Leaderboard()
	applyAndTick(t, r, bigRoom)
	u := r.feed.Last()
	if b := u.World.Bots[0]; b.HP != u.World.MaxHP {
		t.Errorf("bot not fresh: %+v", b)
	}
	if got := r.a.Leaderboard(); !reflect.DeepEqual(got, before) {
		t.Errorf("board changed: %+v -> %+v", before, got)
	}
}

func applyAndTick(t *testing.T, r *rig, text string) {
	t.Helper()
	if err := r.a.ApplyMap(text); err != nil {
		t.Fatal(err)
	}
	r.tick(t, 1)
}

func TestApplyMapGivesFreshMemoryAndNoSwapLimit(t *testing.T) {
	r := newRig(t, 0)
	token, _ := r.join(t, "Ann")
	if err := r.a.PutBot(token, "function loop() { num[0] = 5; wait(); }"); err != nil {
		t.Fatal(err)
	}
	r.tick(t, 4)
	if err := r.a.ApplyMap(bigRoom); err != nil {
		t.Fatal(err)
	}
	r.tick(t, 1)
	if st, _ := r.a.Inspect(token); !st.InArena {
		t.Fatalf("not in the arena after the map change: %+v", st)
	}
	if err := r.a.PutBot(token, waiter); err != nil {
		t.Errorf("swap right after a new game: %v", err)
	}
	if got := r.feed.Last().Probes["p1.edited"]; got != 0 {
		t.Errorf("stale edited probe %d", got)
	}
}

func TestApplyMapKeepsQueuedJoins(t *testing.T) {
	r := newRig(t, 0)
	r.join(t, "Ann")
	r.tick(t, 1)
	cy, _ := r.join(t, "Cy") // still waiting for its join
	applyAndTick(t, r, bigRoom)
	if got := r.roster(); got != "p1,p2" {
		t.Errorf("roster %q", got)
	}
	if _, err := r.a.GetBot(cy); err != nil {
		t.Errorf("Cy: %v", err)
	}
}

func TestApplyMapDropsQueuedLeavesAndEdits(t *testing.T) {
	r := newRig(t, 0)
	ann, _ := r.join(t, "Ann")
	bob, _ := r.join(t, "Bob")
	r.tick(t, 1)
	if err := r.a.Leave(bob); err != nil {
		t.Fatal(err)
	}
	if err := r.a.PutBot(ann, walker); err != nil {
		t.Fatal(err)
	}
	applyAndTick(t, r, bigRoom)
	rec := r.a.Record()
	if got := [3]int{len(rec.Joins), len(rec.Leaves), len(rec.EditLog)}; got != [3]int{1, 0, 0} || r.roster() != "p1" {
		t.Errorf("roster %q, joins/leaves/edits %v", r.roster(), got)
	}
	if v, err := r.a.GetBot(bob); err != nil || v.InArena {
		t.Errorf("Bob: %+v %v", v, err)
	}
}

func TestApplyMapKeepsTheSourceOfAQueuedEdit(t *testing.T) {
	r := newRig(t, 0)
	ann, _ := r.join(t, "Ann")
	r.tick(t, 1)
	if err := r.a.PutBot(ann, walker); err != nil {
		t.Fatal(err)
	}
	applyAndTick(t, r, bigRoom)
	if v, err := r.a.GetBot(ann); err != nil || v.Source != walker {
		t.Errorf("Ann: %+v %v", v, err)
	}
}

func TestApplyMapRefillsHouseBots(t *testing.T) {
	r := newRig(t, 3)
	r.tick(t, 2)
	if err := r.a.ApplyMap(bigRoom); err != nil {
		t.Fatal(err)
	}
	r.tick(t, 1)
	if n := len(r.feed.Last().World.Bots); n != 3 {
		t.Errorf("%d bots after the change", n)
	}
	for _, b := range r.feed.Last().Roster {
		if !b.House {
			t.Errorf("unexpected %+v", b)
		}
	}
}

func TestApplyMapRefusesAFloorSmallerThanTheBots(t *testing.T) {
	r := newRig(t, 0)
	for i := range 4 {
		r.join(t, string(rune('A'+i)))
	}
	r.tick(t, 1)
	if err := r.a.ApplyMap("####\n#.##\n####\n"); !errors.Is(err, app.ErrMapTooSmall) {
		t.Errorf("1 floor cell for 4 bots: %v", err)
	}
	if err := r.a.ApplyMap("####\n#..#\n#..#\n####\n"); err != nil {
		t.Errorf("4 floor cells for 4 bots: %v", err)
	}
}

func TestApplyMapRefusesBadMaps(t *testing.T) {
	r := newRig(t, 0)
	var pe *geo.ParseError
	if err := r.a.ApplyMap("#.#\n#?#\n"); !errors.Is(err, app.ErrBadMap) || !errors.As(err, &pe) {
		t.Errorf("bad text: %v", err)
	}
	if err := r.a.ApplyMap("####\n#A.#\n####\n"); !errors.Is(err, app.ErrBadMap) {
		t.Errorf("bots on the map: %v", err)
	}
	r.tick(t, 1)
	if w := r.a.World(); w.Beat != 1 || w.Grid.Width() != 9 {
		t.Errorf("a refused map changed the game: beat %d", w.Beat)
	}
}

func TestApplyMapOnAClosedArena(t *testing.T) {
	r := newRig(t, 0)
	r.a.Close()
	if err := r.a.ApplyMap(bigRoom); !errors.Is(err, app.ErrClosed) {
		t.Errorf("err %v", err)
	}
}

func TestApplyMapRefusesAFloorBelowMinBots(t *testing.T) {
	r := newRig(t, 6)
	if err := r.a.ApplyMap("####\n#..#\n####\n"); err == nil {
		t.Error("2 floor cells accepted with min bots 6")
	}
}

func TestApplyMapLastCallBeforeTheBeatWinsAndSeedGoesUpOnce(t *testing.T) {
	r := newRig(t, 0)
	if err := r.a.ApplyMap(bigRoom); err != nil {
		t.Fatal(err)
	}
	if err := r.a.ApplyMap(room); err != nil {
		t.Fatal(err)
	}
	r.tick(t, 1)
	rec := r.a.Record()
	if rec.Map != room || rec.Seed != 12 {
		t.Errorf("map %q seed %d", rec.Map, rec.Seed)
	}
}

// games records the log and Record of every game the arena plays.
type games struct {
	r    *rig
	logs [][]event.Logged
	recs []app.Record
}

func (g *games) play(t *testing.T, n int) {
	t.Helper()
	for range n {
		g.r.tick(t, 1)
		g.logs[len(g.logs)-1] = append(g.logs[len(g.logs)-1], g.r.feed.Last().Events...)
	}
	g.recs[len(g.recs)-1] = g.r.a.Record()
}

func (g *games) next(t *testing.T, text string) {
	t.Helper()
	if err := g.r.a.ApplyMap(text); err != nil {
		t.Fatal(err)
	}
	g.logs, g.recs = append(g.logs, nil), append(g.recs, app.Record{})
}

func TestEachGamesRecordReplaysItsOwnLog(t *testing.T) {
	r := newRig(t, 3)
	g := &games{r: r, logs: [][]event.Logged{nil}, recs: []app.Record{{}}}
	ann, _ := r.join(t, "Ann")
	_ = r.a.PutBot(ann, firer)
	g.play(t, 60)
	g.next(t, bigRoom)
	g.play(t, 1) // the new game starts at its beat 1, with the players rejoining
	r.join(t, "Bob")
	g.play(t, 80)
	g.next(t, room)
	g.play(t, 70)
	if len(g.recs) != 3 {
		t.Fatalf("%d games", len(g.recs))
	}
	wantMaps, wantSeeds := []string{room, bigRoom, room}, []uint64{11, 12, 13}
	for i, rec := range g.recs {
		if rec.Map != wantMaps[i] || rec.Seed != wantSeeds[i] || rec.Beats == 0 {
			t.Errorf("game %d: seed %d beats %d map %q", i, rec.Seed, rec.Beats, rec.Map)
		}
		g.checkReplay(t, i)
	}
}

func (g *games) checkReplay(t *testing.T, i int) {
	t.Helper()
	want, _ := app.HashLog(g.logs[i])
	replayed, err := app.Replay(g.recs[i])
	if err != nil {
		t.Fatalf("game %d: %v", i, err)
	}
	if got, _ := app.HashLog(replayed); got != want {
		t.Errorf("game %d: replay hash %s, live hash %s", i, got, want)
	}
}

func TestApplyMapSavesTheMapAndRetriesAFailedSave(t *testing.T) {
	r := newRig(t, 0)
	r.store.Err = errors.New("disk full")
	if err := r.a.ApplyMap(bigRoom); err != nil {
		t.Fatal(err)
	}
	if err := r.a.Tick(); err == nil {
		t.Fatal("save error not reported")
	}
	if r.store.Map != "" {
		t.Fatalf("map saved despite the error: %q", r.store.Map)
	}
	r.store.Err = nil
	r.tick(t, 1)
	if r.store.Map != bigRoom {
		t.Errorf("stored map %q", r.store.Map)
	}
}

func TestAMapSavedInTheStoreIsNotSavedAgainWithoutChange(t *testing.T) {
	r := newRig(t, 0)
	r.tick(t, 2)
	if r.store.Map != "" {
		t.Errorf("map written without ApplyMap: %q", r.store.Map)
	}
}

func houseLines(r *rig) (ids []string) {
	for _, e := range r.a.Leaderboard() {
		ids = append(ids, e.Bot)
	}
	slices.Sort(ids)
	return ids
}

func TestApplyMapKeepsTheHouseBotsAndTheirBoardLines(t *testing.T) {
	r := newRig(t, 3)
	r.tick(t, 120)
	before, bots := houseLines(r), r.a.Bots()
	for range 3 {
		applyAndTick(t, r, bigRoom)
	}
	if got := houseLines(r); !slices.Equal(got, before) {
		t.Errorf("board lines %v -> %v", before, got)
	}
	if got := r.a.Bots(); !reflect.DeepEqual(got, bots) {
		t.Errorf("bots %+v -> %+v", bots, got)
	}
}

func TestApplyMapDropsExtraHouseBotsWhenHumansFillTheirSeats(t *testing.T) {
	r := newRig(t, 3)
	r.tick(t, 2)
	r.join(t, "Ann")
	r.tick(t, 1)
	applyAndTick(t, r, bigRoom)
	if n := len(r.feed.Last().World.Bots); n != 3 {
		t.Errorf("%d bots, want 3", n)
	}
}

func TestJoinAndSetMinBotsUseThePendingMapsFloor(t *testing.T) {
	r := newRig(t, 0)
	if err := r.a.ApplyMap("####\n#..#\n####\n"); err != nil { // 2 floor cells
		t.Fatal(err)
	}
	r.join(t, "Ann")
	r.join(t, "Bob")
	if _, _, err := r.a.Join("Cy"); !errors.Is(err, app.ErrFull) {
		t.Errorf("third join on 2 cells: %v", err)
	}
	if err := r.a.SetMinBots(3); !errors.Is(err, app.ErrMinBots) {
		t.Errorf("min bots 3 on 2 cells: %v", err)
	}
	if err := r.a.SetMinBots(2); err != nil {
		t.Errorf("min bots 2: %v", err)
	}
}
