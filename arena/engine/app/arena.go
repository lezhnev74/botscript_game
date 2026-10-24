package app

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/lezhnev/arena/engine/domain/brain"
	"github.com/lezhnev/arena/engine/domain/geo"
	"github.com/lezhnev/arena/engine/domain/sim"
	"github.com/lezhnev/arena/engine/lang"
)

// DefaultSwapEvery is the hot-swap rate limit: one swap per this many beats.
const DefaultSwapEvery = 3

// Config describes the one continuous arena.
type Config struct {
	Rules       sim.Rules
	Map         string   // ASCII room without bots (players and house bots join it)
	Seed        uint64   // seeds the sim and the bots' dice
	MinBots     int      // house bots fill the arena up to this many bots
	MaxPlayers  int      // cap on human players (house bots not counted); 0 = only the floor limits them
	Houses      []string // house bots to cycle through; empty = all of lang.HouseBotNames
	SwapEvery   int      // beats between hot-swaps (and leave/return changes) of one bot; 0 = DefaultSwapEvery
	MaxStay     int      // beats a human bot may stay in the arena before it detaches; 0 = no limit
	RebootBeats int      // beats a saved program keeps its bot off the map (the reboot); 0 = a plain hot-swap limited by SwapEvery
	Points      *Points  // scoring table; nil = DefaultPoints (not hashed, not recorded)
}

// Deps are the ports the arena needs.
type Deps struct {
	Store       Store
	Broadcaster Broadcaster
	IDs         IDGen
	OnError     func(error) // reports a failed Tick inside Run (a Store error); nil = ignore
}

// Arena is the live service: one continuous arena that plays one beat per
// Tick. All methods are safe for concurrent use; commands (join, leave, put)
// are queued under a mutex and take effect at the start of the next beat,
// in a fixed order (leaves, joins, edits), which is also the order the live
// Record keeps, so Replay(Record()) reproduces the session exactly.
type Arena struct {
	mu    sync.Mutex // the state lock: commands and beats take it briefly
	tick  sync.Mutex // serialises whole Ticks: beat, broadcast and saves, in beat order
	floor int        // floor cells of the room: at most this many players fit
	cfg   Config
	deps  Deps
	match *Match
	board *Board
	rec   Record

	players map[string]*player // by bot id
	tokens  map[string]string  // token -> bot id
	queue   queue
	houseN  int // house bots made so far; ids are never reused in a session
	seq     int // players registered so far

	nextMap *nextMap // set by ApplyMap, taken at the next beat boundary

	remaining int     // beats left on the game timer; noTimer = none
	stopped   bool    // the game is over: no beats until StartGame
	last      []Entry // final board of the latest finished game

	boardDirty, botsDirty, mapDirty, lastDirty bool
	closed                                     bool
}

// NewArena builds the arena, loading the leaderboard and the players' bots
// from the Store. Stored players rejoin at the first beat, keeping their tokens.
func NewArena(cfg Config, d Deps) (*Arena, error) {
	m, err := geo.ParseMap(cfg.Map)
	if err != nil {
		return nil, fmt.Errorf("arena map: %w", err)
	}
	if err := checkRoom(m, cfg); err != nil {
		return nil, err
	}
	a := &Arena{
		cfg: cfg, deps: d, floor: floorCells(m.Grid),
		match:   NewMatch(sim.NewWorld(m, cfg.Rules, cfg.Seed), cfg.Rules, map[string]brain.Brain{}),
		rec:     Record{Version: RecordVersion, Rules: cfg.Rules, Map: cfg.Map, Seed: cfg.Seed},
		players: map[string]*player{}, tokens: map[string]string{}, remaining: noTimer,
	}
	if err := a.load(); err != nil {
		return nil, err
	}
	return a, nil
}

func checkRoom(m geo.Map, cfg Config) error {
	if len(m.Bots) > 0 {
		return fmt.Errorf("arena map must have no bots, found %d", len(m.Bots))
	}
	if floor := floorCells(m.Grid); cfg.MinBots > floor {
		return fmt.Errorf("MinBots %d exceeds the %d floor cells of the arena", cfg.MinBots, floor)
	}
	for _, name := range cfg.Houses {
		if _, ok := lang.HouseBot(name); !ok {
			return fmt.Errorf("unknown house bot %q", name)
		}
	}
	return nil
}

func (a *Arena) swapEvery() int {
	if a.cfg.SwapEvery > 0 {
		return a.cfg.SwapEvery
	}
	return DefaultSwapEvery
}

// Tick plays one beat: queued leaves, joins and edits first, then the sim,
// then the leaderboard, the broadcast and the saves. A Store failure is
// returned after the beat has been played (the save is retried on the next
// beat).
//
// Only the beat itself runs under the state lock. The broadcast and the Store
// saves happen after it is released, so a slow Broadcaster or Store never
// blocks commands (Join, PutBot, ...); it only delays the next beat, because
// Ticks are serialised and keep their order.
func (a *Arena) Tick() error {
	a.tick.Lock()
	defer a.tick.Unlock()
	u, job, played, err := a.play()
	if err != nil {
		return err
	}
	if played {
		a.deps.Broadcaster.Publish(u)
	}
	return a.flush(job)
}

// play plays one beat under the state lock and collects what must be saved.
// While the game is over it plays nothing (played is false) but still
// collects the saves of commands accepted meanwhile.
func (a *Arena) play() (u Update, job saveJob, played bool, err error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closed {
		return Update{}, saveJob{}, false, ErrClosed
	}
	if !a.stopped {
		u, played = a.beat(), true
	}
	return u, a.takeSave(), played, nil
}

// beat plays one beat under the state lock and describes it.
func (a *Arena) beat() Update {
	if a.nextMap != nil {
		a.startGame()
	}
	beat := a.match.World().Beat + 1
	a.beginBeat(beat)
	evs := a.match.Step()
	a.countStays(beat)
	a.landBadges(beat)
	a.boardDirty = a.board.Apply(evs) || a.boardDirty
	u := NewUpdate(a.match, evs, a.roster(), a.edited(), a.board.Top())
	u.GameOver = a.countDown()
	u.Remaining = a.remaining
	return u
}

// Run plays one beat per clock tick until ctx ends, the clock closes or the
// arena is closed. A failed beat (a Store error) goes to Deps.OnError and the
// arena plays on.
func (a *Arena) Run(ctx context.Context, c Clock) error {
	ticks := c.Ticks()
	for {
		select {
		case <-ctx.Done():
			return nil
		case _, ok := <-ticks:
			if !ok {
				return nil
			}
			if err := a.tickReported(); err != nil {
				return err
			}
		}
	}
}

// tickReported plays a beat; a Store error is reported and swallowed, only
// ErrClosed is returned.
func (a *Arena) tickReported() error {
	err := a.Tick()
	if errors.Is(err, ErrClosed) {
		return err
	}
	if err != nil {
		a.report(err)
	}
	return nil
}

func (a *Arena) report(err error) {
	if a.deps.OnError != nil {
		a.deps.OnError(err)
	}
}

// Flush saves what changed since the last save (a join or an edit that no beat
// has carried to the Store yet). Call it after Run has stopped, before the
// process exits, so no accepted command is lost.
func (a *Arena) Flush() error {
	a.tick.Lock()
	defer a.tick.Unlock()
	a.mu.Lock()
	a.botsDirty = a.botsDirty || a.cfg.MaxStay > 0 // the stay counts
	job := a.takeSave()
	a.mu.Unlock()
	return a.flush(job)
}

// Close stops the bots' goroutines. The arena plays no more beats.
func (a *Arena) Close() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.closed {
		a.closed = true
		a.match.Close()
	}
}

// World is the state after the last beat.
func (a *Arena) World() sim.World {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.match.World()
}

// Record is the live session so far as a replayable Record: Replay of it
// gives the same event log as the arena has played.
func (a *Arena) Record() Record {
	a.mu.Lock()
	defer a.mu.Unlock()
	r := a.rec
	r.Beats = a.match.World().Beat
	r.Joins = append([]Join(nil), r.Joins...)
	r.Leaves = append([]Leave(nil), r.Leaves...)
	r.EditLog = append([]Edit(nil), r.EditLog...)
	return r
}

// Leaderboard lists the all-time entries, best first.
func (a *Arena) Leaderboard() []Entry {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.board.Top()
}

func floorCells(g geo.Grid) int {
	n := 0
	for y := range g.Height() {
		for x := range g.Width() {
			if !g.Blocked(geo.Pos{X: x, Y: y}) {
				n++
			}
		}
	}
	return n
}
