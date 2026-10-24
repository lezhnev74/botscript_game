package app_test

import (
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/lezhnev/arena/engine/app"
	"github.com/lezhnev/arena/engine/app/apptest"
	"github.com/lezhnev/arena/engine/lang"
)

const memWriter = "function loop() { num[0] = 7; pin[2] = at(4, 5); fire(E); }"

func TestUpdateCarriesBoardAndProbes(t *testing.T) {
	r := newRig(t, 0)
	token, id := r.join(t, "Ann")
	if err := r.a.PutBot(token, memWriter); err != nil {
		t.Fatal(err)
	}
	r.tick(t, 2)
	u := r.feed.Last()
	if len(u.Board) != 1 || u.Board[0].Bot != id || u.Board[0].Name != "Ann" {
		t.Errorf("board %+v", u.Board)
	}
	if u.Probes[id+".num_0"] != 7 {
		t.Errorf("probes %v", u.Probes)
	}
}

func TestUpdateCarriesPinProbes(t *testing.T) {
	r := newRig(t, 0)
	token, id := r.join(t, "Ann")
	if err := r.a.PutBot(token, memWriter); err != nil {
		t.Fatal(err)
	}
	r.tick(t, 2)
	p := r.feed.Last().Probes
	if p[id+".pin_2_set"] != 1 || p[id+".pin_2_x"] != 4 || p[id+".pin_2_y"] != 5 || p[id+".pin_0_set"] != 0 {
		t.Errorf("pin probes %v", p)
	}
}

func TestInspectShowsMemoryActAndTrail(t *testing.T) {
	r := newRig(t, 0)
	token, _ := r.join(t, "Ann")
	if err := r.a.PutBot(token, memWriter); err != nil {
		t.Fatal(err)
	}
	st, err := r.a.Inspect(token)
	if err != nil || st.InArena {
		t.Fatalf("before the first beat: %+v %v", st, err)
	}
	r.tick(t, 3)
	st, err = r.a.Inspect(token)
	if got := []bool{err == nil, st.InArena, st.Num[0] == 7, len(st.Trail) > 0, st.Act >= 0}; slices.Contains(got, false) {
		t.Errorf("after beats: %+v %v", st, err)
	}
	if st.Pin[2] != (lang.Place{X: 4, Y: 5, Set: true}) || st.Pin[0].Set {
		t.Errorf("pins %v", st.Pin)
	}
}

func TestInspectRejectsUnknownTokens(t *testing.T) {
	r := newRig(t, 0)
	if _, err := r.a.Inspect("nope"); !errors.Is(err, app.ErrUnauthorized) {
		t.Errorf("err %v", err)
	}
}

// gateStore blocks SaveBots until released.
type gateStore struct {
	*apptest.MemStore
	entered chan struct{}
	release chan struct{}
}

func (g *gateStore) SaveBots(b []app.SavedBot) error {
	select {
	case g.entered <- struct{}{}:
	default:
	}
	<-g.release
	return g.MemStore.SaveBots(b)
}

func newGateStore(r *rig) *gateStore {
	g := &gateStore{MemStore: r.store, entered: make(chan struct{}, 1), release: make(chan struct{})}
	r.useStore(g)
	return g
}

func TestSlowStoreDoesNotBlockCommands(t *testing.T) {
	r := newUnstartedRig(0, room)
	gs := newGateStore(r)
	r.start(t)
	r.join(t, "Ann")
	done := make(chan error, 1)
	go func() { done <- r.a.Tick() }()
	<-gs.entered
	joined := make(chan error, 1)
	go func() { _, _, err := r.a.Join("Bob"); joined <- err }()
	select {
	case err := <-joined:
		if err != nil {
			t.Error(err)
		}
	case <-time.After(5 * time.Second):
		t.Error("Join blocked while the store was saving")
	}
	close(gs.release)
	if err := <-done; err != nil {
		t.Error(err)
	}
}

func TestFlushSavesPendingJoinsAndEditsWithoutABeat(t *testing.T) {
	r := newRig(t, 0)
	token, id := r.join(t, "Ann")
	if err := r.a.Flush(); err != nil {
		t.Fatal(err)
	}
	if len(r.store.Bots) != 1 || r.store.Bots[0].ID != id {
		t.Fatalf("join not saved: %+v", r.store.Bots)
	}
	if err := r.a.PutBot(token, memWriter); err != nil {
		t.Fatal(err)
	}
	if err := r.a.Flush(); err != nil || r.store.Bots[0].Source != memWriter {
		t.Errorf("edit not saved: %+v %v", r.store.Bots, err)
	}
}

func TestFlushReportsAndRetriesAFailedSave(t *testing.T) {
	r := newRig(t, 0)
	r.join(t, "Ann")
	r.store.Err = errors.New("disk full")
	if err := r.a.Flush(); err == nil {
		t.Fatal("want the save error")
	}
	r.store.Err = nil
	if err := r.a.Flush(); err != nil || len(r.store.Bots) != 1 {
		t.Errorf("retry: %v %+v", err, r.store.Bots)
	}
}

func TestProbesShowTheBeatOfTheLatestEditAndLine(t *testing.T) {
	r := newRig(t, 0)
	token, id := r.join(t, "Ann")
	r.tick(t, 1)
	if _, ok := r.feed.Last().Probes[id+".edited"]; ok {
		t.Error("a bot that was never edited has no edit beat")
	}
	if err := r.a.PutBot(token, memWriter); err != nil {
		t.Fatal(err)
	}
	r.tick(t, 2)
	p := r.feed.Last().Probes
	if p[id+".edited"] != 2 || p[id+".line"] != 1 {
		t.Errorf("probes %v", p)
	}
}
