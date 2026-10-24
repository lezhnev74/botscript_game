package app

import (
	"github.com/lezhnev/arena/engine/domain/event"
	"github.com/lezhnev/arena/engine/domain/sim"
)

// Clock drives the live arena: one value on Ticks is one beat. The arena never
// reads wall time. A real adapter wraps a time.Ticker (and drops ticks when
// the arena is slow); apptest.ManualClock sends ticks by hand. A closed
// channel stops Arena.Run.
type Clock interface {
	Ticks() <-chan struct{}
}

// Player is a bot in the arena roster.
type Player struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	House bool   `json:"house"` // a house bot filling an empty seat
	Look  Look   `json:"look"`
}

// Update is what the arena publishes after every beat: the world after the
// beat, the beat's events and who is in the arena. World is a value the
// arena never changes again, so receivers may keep it.
type Update struct {
	Beat   int
	World  sim.World
	Events []event.Logged
	Roster []Player
	Board  []Entry // the all-time leaderboard after the beat, best first
	// Remaining is the beats left on the game timer after the beat, -1 when no timer runs.
	Remaining int
	// GameOver is true on the update that ends a game and in every snapshot taken after it.
	GameOver bool
	Probes   map[string]int // per bot, keyed "<bot>.num_0", "<bot>.pin_0_x" etc.: memory, act, line, edited (beat of the last hot-swap)
}

// Broadcaster carries beats to the UI (the WebSocket adapter). Publish runs
// inside the beat: it must not block and must not call back into the Arena.
type Broadcaster interface {
	Publish(Update)
}

// SavedBot is a player's bot as persisted, so a token survives a restart.
type SavedBot struct {
	Token  string `json:"token"`
	ID     string `json:"id"`
	Name   string `json:"name"`
	Source string `json:"source"`
	Look   Look   `json:"look"` // zero = the default look of the id
	// Detached bots are out of the arena (they left or their stay ran out):
	// restored without joining. LeftBy says who sent them out.
	Detached  bool   `json:"detached,omitempty"`
	LeftBy    string `json:"leftBy,omitempty"`
	StayBeats int    `json:"stayBeats,omitempty"` // beats spent in the current stay
}

// Store persists the all-time leaderboard, the players' bots and the current map. Loads return
// empty slices for a store that was never written.
type Store interface {
	LoadBoard() ([]Entry, error)
	SaveBoard([]Entry) error
	LoadBots() ([]SavedBot, error)
	SaveBots([]SavedBot) error
	// LoadMap returns the arena map applied by the admin, "" when there is none.
	LoadMap() (string, error)
	SaveMap(text string) error
	// LoadLast and SaveLast keep the final leaderboard of the latest finished game.
	LoadLast() ([]Entry, error)
	SaveLast([]Entry) error
}

// IDGen makes the bot ids and secret tokens of joining players. Ids must be
// unique across restarts (they key the persisted leaderboard) and tokens
// unguessable. It is the only randomness of the app layer and never feeds the
// simulation.
type IDGen interface {
	BotID() string
	Token() string
}
