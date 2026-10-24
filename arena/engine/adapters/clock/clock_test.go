package clock_test

import (
	"testing"
	"time"

	"github.com/lezhnev/arena/engine/adapters/clock"
)

func TestTickerTicksAndStops(t *testing.T) {
	c := clock.New(2 * time.Millisecond)
	for range 3 {
		select {
		case _, ok := <-c.Ticks():
			if !ok {
				t.Fatal("closed early")
			}
		case <-time.After(5 * time.Second):
			t.Fatal("no tick")
		}
	}
	c.Stop()
	c.Stop() // idempotent
	select {
	case <-drain(c):
	case <-time.After(5 * time.Second):
		t.Fatal("Ticks did not close after Stop")
	}
}

func drain(c *clock.Ticker) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		for range c.Ticks() {
		}
		close(done)
	}()
	return done
}

func TestSlowReaderMissesTicksInsteadOfBlocking(t *testing.T) {
	c := clock.New(time.Millisecond)
	defer c.Stop()
	time.Sleep(30 * time.Millisecond)
	n := 0
	for {
		select {
		case <-c.Ticks():
			n++
			continue
		default:
		}
		break
	}
	if n > 1 {
		t.Errorf("buffered %d ticks, want at most 1", n)
	}
}
