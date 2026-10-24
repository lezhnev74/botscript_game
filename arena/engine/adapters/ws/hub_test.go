package ws_test

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/lezhnev/arena/engine/adapters/protocol"
	"github.com/lezhnev/arena/engine/adapters/ws"
	"github.com/lezhnev/arena/engine/app"
	"github.com/lezhnev/arena/engine/app/apptest"
	"github.com/lezhnev/arena/engine/domain/sim"
)

const room = "#########\n#.......#\n#.......#\n#.......#\n#.......#\n#.......#\n#########\n"

// live is an arena that broadcasts to a hub served over httptest.
type live struct {
	a     *app.Arena
	hub   *ws.Hub
	srv   *httptest.Server
	clock *apptest.ManualClock
	beats chan struct{} // one signal per beat, sent after the hub has the update
}

// tee hands every update to the hub and then signals that the beat is done.
type tee struct {
	hub   *ws.Hub
	beats chan<- struct{}
}

func (t tee) Publish(u app.Update) {
	t.hub.Publish(u)
	t.beats <- struct{}{}
}

// advance plays n beats and returns once the hub has published all of them.
func (l *live) advance(t *testing.T, n int) {
	t.Helper()
	l.clock.Advance(n)
	for range n {
		select {
		case <-l.beats:
		case <-time.After(5 * time.Second):
			t.Fatal("beat was not published")
		}
	}
}

func newLive(t *testing.T, opts ws.Options) *live {
	t.Helper()
	l := &live{clock: apptest.NewManualClock(), beats: make(chan struct{}, 64)}
	cfg := app.Config{Rules: sim.DefaultRules(), Map: room, Seed: 3, MinBots: 2}
	// the hub is created before the arena, so it starts empty (beat 0)
	l.hub = ws.New(opts, app.Update{})
	a, err := app.NewArena(cfg, app.Deps{Store: &apptest.MemStore{}, Broadcaster: tee{hub: l.hub, beats: l.beats}, IDs: &apptest.Seq{}})
	if err != nil {
		t.Fatal(err)
	}
	l.a = a
	l.hub.Reset(app.Update{World: a.World(), Board: a.Leaderboard()})
	l.srv = httptest.NewServer(l.hub)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { _ = a.Run(ctx, l.clock); close(done) }()
	t.Cleanup(func() {
		cancel()
		<-done
		l.srv.Close()
		l.hub.Close()
		a.Close()
	})
	return l
}

func (l *live) dial(t *testing.T) *websocket.Conn {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(l.srv.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.CloseNow() }) //nolint:errcheck
	return c
}

func read(t *testing.T, c *websocket.Conn) protocol.Message {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, data, err := c.Read(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var m protocol.Message
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("%s: %v", data, err)
	}
	return m
}

func TestClientGetsASnapshotThenFrames(t *testing.T) {
	l := newLive(t, ws.Options{})
	c := l.dial(t)
	snap := read(t, c)
	if got := []int{snap.Frame.Beat, snap.Frame.Width, len(snap.Events)}; snap.Type != protocol.TypeSnapshot || !slices.Equal(got, []int{0, 9, 0}) {
		t.Fatalf("snapshot %+v", snap)
	}
	l.advance(t, 1)
	beat := read(t, c)
	if got := []int{beat.Frame.Beat, len(beat.Frame.Bots)}; beat.Type != protocol.TypeBeat || !slices.Equal(got, []int{1, 2}) || len(beat.Events) == 0 {
		t.Fatalf("beat %+v", beat)
	}
	l.advance(t, 1)
	if m := read(t, c); m.Frame.Beat != 2 {
		t.Errorf("second beat %+v", m.Frame)
	}
}

func TestLateClientSnapshotIsTheLatestBeat(t *testing.T) {
	l := newLive(t, ws.Options{})
	l.advance(t, 2)
	c := l.dial(t)
	snap := read(t, c)
	if snap.Type != protocol.TypeSnapshot || snap.Frame.Beat != 2 || len(snap.Events) != 0 || len(snap.Frame.Bots) != 2 {
		t.Errorf("snapshot %+v", snap)
	}
}

func TestEveryClientGetsEveryBeat(t *testing.T) {
	l := newLive(t, ws.Options{})
	a, b := l.dial(t), l.dial(t)
	read(t, a)
	read(t, b)
	l.advance(t, 1)
	if read(t, a).Frame.Beat != 1 || read(t, b).Frame.Beat != 1 {
		t.Error("both clients should see beat 1")
	}
}

func TestClientsAreForgottenWhenTheyLeave(t *testing.T) {
	l := newLive(t, ws.Options{})
	c := l.dial(t)
	read(t, c)
	if l.hub.Clients() != 1 {
		t.Fatalf("clients %d", l.hub.Clients())
	}
	_ = c.Close(websocket.StatusNormalClosure, "bye")
	waitFor(t, func() bool { return l.hub.Clients() == 0 })
}

func TestCloseDisconnectsClients(t *testing.T) {
	l := newLive(t, ws.Options{})
	c := l.dial(t)
	read(t, c)
	l.hub.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, _, err := c.Read(ctx); err == nil {
		t.Error("read should fail after Close")
	}
}

func TestForeignOriginIsRefusedUnlessAllowed(t *testing.T) {
	l := newLive(t, ws.Options{})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	url := "ws" + strings.TrimPrefix(l.srv.URL, "http")
	hdr := map[string][]string{"Origin": {"https://evil.example"}}
	if c, _, err := websocket.Dial(ctx, url, &websocket.DialOptions{HTTPHeader: hdr}); err == nil {
		c.CloseNow() //nolint:errcheck
		t.Fatal("a foreign origin must be refused")
	}
	l2 := newLive(t, ws.Options{OriginPatterns: []string{"evil.example"}})
	url = "ws" + strings.TrimPrefix(l2.srv.URL, "http")
	c, _, err := websocket.Dial(ctx, url, &websocket.DialOptions{HTTPHeader: hdr})
	if err != nil {
		t.Fatal(err)
	}
	c.CloseNow() //nolint:errcheck
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatal("condition not met")
		}
		time.Sleep(time.Millisecond)
	}
}

func TestNewClientSeesGameOverAndTheFinalBoard(t *testing.T) {
	l := newLive(t, ws.Options{})
	if err := l.a.SetTimer(2); err != nil {
		t.Fatal(err)
	}
	l.advance(t, 2)
	l.clock.Advance(1) // a tick while stopped plays nothing
	snap := read(t, l.dial(t))
	if snap.Type != protocol.TypeSnapshot || !snap.GameOver || snap.Remaining != 0 || snap.Frame.Beat != 2 {
		t.Fatalf("snapshot %+v", snap.Frame.Beat)
	}
	if len(snap.Board) != 2 {
		t.Errorf("final board %+v", snap.Board)
	}
}

func TestMessagesCarryTheBeatLength(t *testing.T) {
	l := newLive(t, ws.Options{Beat: 250 * time.Millisecond})
	c := l.dial(t)
	if snap := read(t, c); snap.BeatMs != 250 {
		t.Errorf("snapshot beatMs %d", snap.BeatMs)
	}
	l.advance(t, 1)
	if beat := read(t, c); beat.BeatMs != 250 {
		t.Errorf("beat beatMs %d", beat.BeatMs)
	}
}
