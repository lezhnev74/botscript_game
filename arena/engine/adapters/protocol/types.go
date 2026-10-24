// Package protocol holds the wire types of the arena: what the WebSocket
// broadcasts and what the REST API sends and receives. tygo turns this package
// into web/src/protocol (make gen), so it is the only contract with the UI.
// Only plain structs, strings, numbers, bools and slices belong here.
package protocol

// Message types of the WebSocket stream.
const (
	TypeSnapshot = "snapshot" // the state on connect, with no events
	TypeBeat     = "beat"     // one played beat
)

// Message is one WebSocket text frame.
type Message struct {
	Type   string             `json:"type"`
	Frame  Frame              `json:"frame"`
	Events []Event            `json:"events"`
	Board  []LeaderboardEntry `json:"board"`
	// Remaining is the beats left on the game timer, -1 when no timer runs.
	Remaining int `json:"remaining"`
	// GameOver is true once the timer has run out and until the admin starts the next game.
	GameOver bool `json:"gameOver"`
	// BeatMs is the length of a beat in milliseconds (0 = unknown), to turn Remaining into a clock.
	BeatMs int `json:"beatMs"`
}

// Cell is a map cell.
type Cell struct {
	X int `json:"x"`
	Y int `json:"y"`
}

// Frame is the world after a beat. Grid holds one string per row, '#' for a
// wall and '.' for floor; bots, bullets and coins are listed apart.
type Frame struct {
	Beat   int         `json:"beat"`
	Width  int         `json:"width"`
	Height int         `json:"height"`
	Grid   []string    `json:"grid"`
	Bots   []Bot       `json:"bots"`
	Fallen []FallenBot `json:"fallen"`
	// Rebooting lists the bots off the map for a reboot (a saved program), by bot id.
	Rebooting []RebootingBot `json:"rebooting"`
	Bullets   []Bullet       `json:"bullets"`
	Coins     []Cell         `json:"coins"`
	MaxHP     int            `json:"maxHp"`  // HP of a full health bar
	Reload    int            `json:"reload"` // beats of a full reload (a bot's cooldown counts down from reload-1 after a shot)
}

// Look is the cosmetic face of a bot: a #rrggbb color and four named parts
// (body, eyes, mouth, hat). Not part of the simulation.
type Look struct {
	Color string `json:"color"`
	Body  string `json:"body"`
	Eyes  string `json:"eyes"`
	Mouth string `json:"mouth"`
	Hat   string `json:"hat"`
}

// Bot is a live bot. Num and Act are the program's public state (phone live view; the TV draws no memory badge); house bots
// that are not programs have zero memory. Act is the kind of the latest action
// (SCAN 0, MOVE 1, FIRE 2, WAIT 3, IDLE 4; -1 before the first).
type Bot struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	House  bool   `json:"house"`
	Look   Look   `json:"look"`
	X      int    `json:"x"`
	Y      int    `json:"y"`
	HP     int    `json:"hp"`
	Loaded bool   `json:"loaded"`
	Bonus  int    `json:"bonus"`
	Shield int    `json:"shield"` // beats of immunity left
	// Cooldown is the beats until the regular shot is ready again (0 = ready).
	Cooldown int    `json:"cooldown"`
	Num      [8]int `json:"num"`
	Act      int    `json:"act"`
	// Line is the source line of the bot's latest action call (phone live view),
	// 0 when unknown. EditedBeat is the beat the latest hot-swap landed on (the pencil badge), 0 when the bot was never edited.
	Line       int `json:"line"`
	EditedBeat int `json:"editedBeat"`
}

// FallenBot is a dead bot waiting to respawn in Wait beats.
type FallenBot struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Look Look   `json:"look"`
	X    int    `json:"x"`
	Y    int    `json:"y"`
	Wait int    `json:"wait"`
}

// RebootingBot is a bot off the map while its new program restarts. Cell is
// where it left (the matrix-rain effect is drawn there); the bot is back in
// BeatsLeft of Total beats.
type RebootingBot struct {
	Bot  string `json:"bot"`
	Name string `json:"name"`
	Look Look   `json:"look"`
	Cell Cell   `json:"cell"`
	// HP is what the bot comes back with (full for one that was dead).
	HP        int `json:"hp"`
	BeatsLeft int `json:"beatsLeft"`
	Total     int `json:"total"`
}

// Bullet is a flying shot; Dir is N, E, S or W.
type Bullet struct {
	X     int    `json:"x"`
	Y     int    `json:"y"`
	Dir   string `json:"dir"`
	Owner string `json:"owner"`
}

// Sighting is an object inside a scan cone and the level it was given.
type Sighting struct {
	Kind  string `json:"kind"` // enemy, coin or bullet
	X     int    `json:"x"`
	Y     int    `json:"y"`
	Level int    `json:"level"`
}

// Event is one thing that happened in a beat. Type is the event name (Moved,
// Bumped, Fired, Clicked, Hit, Scanned, Killed, Respawned, CoinTaken,
// CoinSpawned, Joined, Left) and Args its text arguments, e.g. ["B", "S"].
// Scanned also carries the cone cells and what was seen in them (the TV draws
// the x-ray from this and never recomputes it); Hit names the shooter in By.
type Event struct {
	Beat  int        `json:"beat"`
	Type  string     `json:"type"`
	Args  []string   `json:"args"`
	Cells []Cell     `json:"cells,omitempty"`
	Seen  []Sighting `json:"seen,omitempty"`
	By    string     `json:"by,omitempty"`
}

// LeaderboardEntry is a bot's all-time line.
type LeaderboardEntry struct {
	Bot    string `json:"bot"`
	Name   string `json:"name"`
	Score  int    `json:"score"`
	Kills  int    `json:"kills"`
	Deaths int    `json:"deaths"`
	Coins  int    `json:"coins"`
}

// JoinRequest is the body of POST /api/join.
type JoinRequest struct {
	Name string `json:"name"`
}

// JoinResponse is the answer to POST /api/join. The token goes into
// "Authorization: Bearer <token>" on later calls.
type JoinResponse struct {
	Token string `json:"token"`
	BotID string `json:"botId"`
}

// BotBody is the player's own bot (GET /api/bot).
type BotBody struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Source string `json:"source"`
	Look   Look   `json:"look"`
	// Arena says whether the bot is in the arena and for how long.
	Arena BotArena `json:"arena"`
	// BeatMs is the beat length, to turn StayLeftBeats and RebootLeftBeats into time.
	BeatMs int `json:"beatMs"`
	// RebootLeftBeats is the beats until the bot is back from a reboot, -1 when it is not rebooting.
	// A bot still off the map after its countdown (no free cell yet) reports 0.
	RebootLeftBeats int `json:"rebootLeftBeats"`
	// RebootTotalBeats is the length of a full reboot (0 when saves do not reboot).
	RebootTotalBeats int `json:"rebootTotalBeats"`
}

// BotArena is the arena state of the player's own bot: In the arena, or
// detached (left by its player or by time: LeftBy "player" / "time", "" while in).
// StayLeftBeats counts down to the automatic leave, -1 when detached or without a limit.
type BotArena struct {
	In            bool   `json:"in"`
	LeftBy        string `json:"leftBy"`
	StayLeftBeats int    `json:"stayLeftBeats"`
}

// ProfileRequest is the body of PUT /api/bot/profile: the bot's name and look.
type ProfileRequest struct {
	Name string `json:"name"`
	Look Look   `json:"look"`
}

// ErrorBody is the JSON of every failure except a validation failure.
// RetryBeats is set on 429 only.
type ErrorBody struct {
	Error      string `json:"error"`
	RetryBeats int    `json:"retryBeats,omitempty"`
}

// SourceError is one problem of a rejected program.
type SourceError struct {
	Line int    `json:"line"`
	Col  int    `json:"col"`
	Msg  string `json:"msg"`
	Hint string `json:"hint"`
}

// ValidationBody is the 422 answer to PUT /api/bot: every problem found.
type ValidationBody struct {
	Errors []SourceError `json:"errors"`
}

// TrailEntry is one beat of a bot's trail (newest first). Dir, Hit and Bump
// are directions 0..3 (N E S W) or -1.
type TrailEntry struct {
	X    int  `json:"x"`
	Y    int  `json:"y"`
	Act  int  `json:"act"`
	Dir  int  `json:"dir"`
	OK   bool `json:"ok"`
	Hit  int  `json:"hit"`
	Bump int  `json:"bump"`
}

// BotState is the live state of the player's bot (GET /api/state) for the
// editor. InArena is false until the join lands at the next beat.
type BotState struct {
	ID      string       `json:"id"`
	InArena bool         `json:"inArena"`
	Num     [8]int       `json:"num"`
	Pin     [8]*Cell     `json:"pin" tstype:"(Cell | null)[]"` // null for an empty pin
	Act     int          `json:"act"`
	Trail   []TrailEntry `json:"trail"`
}

// AdminLoginRequest is the body of POST /api/admin/login.
type AdminLoginRequest struct {
	Password string `json:"password"`
}

// AdminBot is one bot line of the admin panel.
type AdminBot struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	House bool   `json:"house"`
	Score int    `json:"score"`
	// Detached is a human bot out of the arena (its token and board line are kept).
	Detached bool `json:"detached,omitempty"`
}

// AdminStatus is the answer of GET /api/admin/status.
type AdminStatus struct {
	Bots    []AdminBot `json:"bots"`
	MinBots int        `json:"minBots"`
	Beat    int        `json:"beat"`
	// MapWidth and MapHeight are the size of the map in play.
	MapWidth  int `json:"mapWidth"`
	MapHeight int `json:"mapHeight"`
	// Running is false once the game timer has ended the game, until the admin starts the next one.
	Running bool `json:"running"`
	// Remaining is the beats left on the game timer (-1 = no timer, 0 = over); BeatMs is the length of a beat.
	Remaining int `json:"remaining"`
	BeatMs    int `json:"beatMs"`
}

// AdminTimerRequest is the body of POST /api/admin/timer: minutes until the
// game ends, 0 cancels the timer.
type AdminTimerRequest struct {
	Minutes float64 `json:"minutes"`
}

// AdminMapPreviewRequest is the body of POST /api/admin/map/preview. Without
// a seed the server draws one; walls is the percent of interior cells that
// become walls (0..15, default 3).
type AdminMapPreviewRequest struct {
	Width  int     `json:"width"`
	Height int     `json:"height"`
	Walls  *int    `json:"walls,omitempty"`
	Seed   *uint64 `json:"seed,omitempty"`
}

// AdminMapPreview is a generated room and the seed that makes it again.
type AdminMapPreview struct {
	Seed uint64 `json:"seed"`
	Map  string `json:"map"`
}

// AdminMapApplyRequest is the body of POST /api/admin/map/apply.
type AdminMapApplyRequest struct {
	Map string `json:"map"`
}

// AdminKickRequest is the body of POST /api/admin/kick.
type AdminKickRequest struct {
	ID string `json:"id"`
}

// AdminMinBotsRequest is the body of POST /api/admin/min-bots.
type AdminMinBotsRequest struct {
	N int `json:"n"`
}
