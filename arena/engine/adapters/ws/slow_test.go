package ws

import (
	"context"
	"testing"
	"time"

	"github.com/lezhnev/arena/engine/app"
)

// A client that stops reading must neither block Publish nor keep growing a
// queue: once its queue is full it is dropped (it reconnects and gets a new
// snapshot).
func TestSlowClientIsDroppedAndPublishNeverBlocks(t *testing.T) {
	h := New(Options{Queue: 2}, app.Update{})
	ctx, cancel := context.WithCancel(context.Background())
	slow := join(t, h, cancel) // nobody reads slow.ch
	done := make(chan struct{})
	go func() {
		for i := 1; i <= 5; i++ {
			h.Publish(app.Update{Beat: i})
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Publish blocked on a slow client")
	}
	if h.Clients() != 0 {
		t.Errorf("slow client still registered")
	}
	select {
	case <-ctx.Done():
	default:
		t.Error("the slow client's connection was not cancelled")
	}
	if got := len(slow.ch); got != 2 {
		t.Errorf("queued %d, want the 2 that fit", got)
	}
}

func TestFastClientKeepsItsPlace(t *testing.T) {
	h := New(Options{Queue: 2}, app.Update{})
	fast := join(t, h, func() {})
	for i := 1; i <= 5; i++ {
		h.Publish(app.Update{Beat: i})
		<-fast.ch
	}
	if h.Clients() != 1 {
		t.Error("a client that keeps up must stay")
	}
}

func join(t *testing.T, h *Hub, cancel context.CancelFunc) *client {
	t.Helper()
	c, _, err := h.snapshotAndAdd(cancel)
	if err != nil {
		t.Fatal(err)
	}
	return c
}
