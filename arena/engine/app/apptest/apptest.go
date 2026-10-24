// Package apptest holds test doubles for the app ports: a hand-cranked Clock,
// an in-memory Store, a recording Broadcaster and a sequential IDGen. Adapter
// tests use them too.
package apptest

import (
	"fmt"
	"sync"

	"github.com/lezhnev/arena/engine/app"
)

// ManualClock ticks only when told to. Advance blocks until the arena has
// received every tick.
type ManualClock struct{ ch chan struct{} }

// NewManualClock returns a clock with no ticks yet.
func NewManualClock() *ManualClock { return &ManualClock{ch: make(chan struct{})} }

// Ticks implements app.Clock.
func (c *ManualClock) Ticks() <-chan struct{} { return c.ch }

// Advance sends n ticks.
func (c *ManualClock) Advance(n int) {
	for range n {
		c.ch <- struct{}{}
	}
}

// Stop closes the clock; Arena.Run returns.
func (c *ManualClock) Stop() { close(c.ch) }

// MemStore is an in-memory app.Store. Set Err to make saves fail.
type MemStore struct {
	mu    sync.Mutex
	Board []app.Entry
	Bots  []app.SavedBot
	Map   string
	Last  []app.Entry
	Err   error
	Saves int // number of successful SaveBoard calls
}

// LoadBoard implements app.Store.
func (s *MemStore) LoadBoard() ([]app.Entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]app.Entry(nil), s.Board...), nil
}

// SaveBoard implements app.Store.
func (s *MemStore) SaveBoard(e []app.Entry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Err != nil {
		return s.Err
	}
	s.Board, s.Saves = append([]app.Entry(nil), e...), s.Saves+1
	return nil
}

// LoadBots implements app.Store.
func (s *MemStore) LoadBots() ([]app.SavedBot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]app.SavedBot(nil), s.Bots...), nil
}

// SaveBots implements app.Store.
func (s *MemStore) SaveBots(b []app.SavedBot) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Err != nil {
		return s.Err
	}
	s.Bots = append([]app.SavedBot(nil), b...)
	return nil
}

// LoadMap implements app.Store.
func (s *MemStore) LoadMap() (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.Map, nil
}

// SaveMap implements app.Store.
func (s *MemStore) SaveMap(text string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Err != nil {
		return s.Err
	}
	s.Map = text
	return nil
}

// LoadLast implements app.Store.
func (s *MemStore) LoadLast() ([]app.Entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]app.Entry(nil), s.Last...), nil
}

// SaveLast implements app.Store.
func (s *MemStore) SaveLast(e []app.Entry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Err != nil {
		return s.Err
	}
	s.Last = append([]app.Entry(nil), e...)
	return nil
}

// Feed is a Broadcaster that keeps every update.
type Feed struct {
	mu      sync.Mutex
	Updates []app.Update
}

// Publish implements app.Broadcaster.
func (f *Feed) Publish(u app.Update) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Updates = append(f.Updates, u)
}

// Last is the latest update (zero before the first beat).
func (f *Feed) Last() app.Update {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.Updates) == 0 {
		return app.Update{}
	}
	return f.Updates[len(f.Updates)-1]
}

// Seq is an app.IDGen giving p1, p2, ... and tok-1, tok-2, ...
type Seq struct {
	mu         sync.Mutex
	bots, toks int
}

// BotID implements app.IDGen.
func (s *Seq) BotID() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.bots++
	return fmt.Sprintf("p%d", s.bots)
}

// Token implements app.IDGen.
func (s *Seq) Token() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.toks++
	return fmt.Sprintf("tok-%d", s.toks)
}
