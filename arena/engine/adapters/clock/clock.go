// Package clock is the real-time app.Clock: one tick per beat length.
package clock

import (
	"sync"
	"time"
)

// Ticker ticks every beat length. A reader that is busy misses ticks (at most
// one is kept), so a slow beat never builds up a backlog.
type Ticker struct {
	out  chan struct{}
	done chan struct{}
	once sync.Once
}

// New starts a ticker; call Stop to release it. beat must be positive.
func New(beat time.Duration) *Ticker {
	c := &Ticker{out: make(chan struct{}, 1), done: make(chan struct{})}
	go c.run(time.NewTicker(beat))
	return c
}

func (c *Ticker) run(t *time.Ticker) {
	defer close(c.out)
	defer t.Stop()
	for {
		select {
		case <-t.C:
			select {
			case c.out <- struct{}{}:
			default:
			}
		case <-c.done:
			return
		}
	}
}

// Ticks implements app.Clock. The channel closes after Stop.
func (c *Ticker) Ticks() <-chan struct{} { return c.out }

// Stop ends the ticks and closes the channel. It is safe to call twice.
func (c *Ticker) Stop() { c.once.Do(func() { close(c.done) }) }
