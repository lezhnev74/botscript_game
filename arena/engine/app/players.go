package app

import (
	"errors"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/lezhnev/arena/engine/lang"
)

const maxNameRunes = 24

// player is a bot's owner-side state. inMatch turns true when the join is
// applied at a beat start.
type player struct {
	id, name, token, source string
	look                    Look
	house                   bool
	seq                     int // creation order, oldest house bot leaves first
	inMatch                 bool
	leaving                 bool // a leave or kick is queued
	kicked                  bool
	swappedAt               int    // beat the latest swap lands on (or the rebooted bot is back); 0 = never swapped
	badgePending            bool   // a reboot is under way: swappedAt is set when the bot is back
	detached                bool   // out of the arena (or leaving it) by choice or by time; token and bot stay
	leftBy                  string // LeftByPlayer or LeftByTime while detached
	movedAt                 int    // beat the latest leave or return lands on; 0 = never
	stay                    int    // beats spent in the match in the current stay
}

// Who sent a bot out of the arena, as BotView.LeftBy reports it.
const (
	LeftByPlayer = "player"
	LeftByTime   = "time"
)

type pendingEdit struct {
	id   string
	prog *lang.Program
	src  string
}

// queue holds the commands waiting for the next beat boundary.
type queue struct {
	joins, leaves []string // bot ids, in arrival order
	edits         []pendingEdit
}

// BotView is what a player sees of their own bot.
type BotView struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Source string `json:"source"`
	Look   Look   `json:"look"`
	// InArena is false while the bot is detached (it left, or its stay ran out).
	InArena bool `json:"inArena"`
	// LeftBy says who detached the bot: "", LeftByPlayer or LeftByTime.
	LeftBy string `json:"leftBy"`
	// StayLeft is the beats until the automatic leave; -1 when detached or without a limit.
	StayLeft int `json:"stayLeft"`
	// RebootLeft is the beats until the bot is back from a reboot (a saved
	// program waiting for the next beat counts the whole reboot); -1 when not rebooting.
	RebootLeft int `json:"rebootLeft"`
	// RebootTotal is the length of a full reboot (RebootBeats), 0 when saves do not reboot.
	RebootTotal int `json:"rebootTotal"`
}

// Join creates a bot with the Rookie template for a new player. The bot
// enters the arena at the next beat; the token is the only credential.
func (a *Arena) Join(name string) (token, botID string, err error) {
	name, err = cleanName(name)
	if err != nil {
		return "", "", err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closed {
		return "", "", ErrClosed
	}
	if a.humans() >= a.playerCap() {
		return "", "", ErrFull
	}
	id, token, err := a.drawCredentials()
	if err != nil {
		return "", "", err
	}
	rookie, _ := lang.HouseBot("rookie") // embedded
	p := &player{id: id, token: token, name: name, source: rookie}
	a.register(p)
	a.queue.joins = append(a.queue.joins, p.id)
	return p.token, p.id, nil
}

// playerCap is how many human players may be registered: the floor of the
// room, lowered by Config.MaxPlayers when that is set.
func (a *Arena) playerCap() int {
	floor := a.currentFloor()
	if a.cfg.MaxPlayers > 0 && a.cfg.MaxPlayers < floor {
		return a.cfg.MaxPlayers
	}
	return floor
}

// cleanName trims a display name and checks its length.
func cleanName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if n := utf8.RuneCountInString(name); n < 1 || n > maxNameRunes {
		return "", ErrBadName
	}
	return name, nil
}

// humans counts the registered human players, waiting ones included and
// detached ones not.
func (a *Arena) humans() int {
	n := 0
	for _, p := range a.players {
		if !p.house && !p.detached {
			n++
		}
	}
	return n
}

const maxDraws = 16

// drawCredentials asks the IDGen for an id and token nobody uses: not a
// player, not a token and not a bot with a leaderboard line (ids key the
// persisted board, so a restarted generator must not reuse them).
func (a *Arena) drawCredentials() (id, token string, err error) {
	for range maxDraws {
		id, token = a.deps.IDs.BotID(), a.deps.IDs.Token()
		_, usedToken := a.tokens[token]
		_, usedID := a.players[id]
		if !usedToken && !usedID && !a.board.Known(id) {
			return id, token, nil
		}
	}
	return "", "", ErrIDCollision
}

// register adds a new player (its join is queued by the caller).
func (a *Arena) register(p *player) {
	if p.look == (Look{}) {
		p.look = DefaultLook(p.id)
	}
	a.seq++
	p.seq = a.seq
	a.players[p.id] = p
	if !p.house {
		a.tokens[p.token] = p.id
		a.botsDirty = true
	}
}

// auth finds the player a token belongs to. Callers hold the lock.
func (a *Arena) auth(token string) (*player, error) {
	if id, ok := a.tokens[token]; ok {
		return a.players[id], nil
	}
	return nil, ErrUnauthorized
}

// GetBot returns the caller's own bot, including a swap not yet applied.
func (a *Arena) GetBot(token string) (BotView, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	p, err := a.auth(token)
	if err != nil {
		return BotView{}, err
	}
	return BotView{
		ID: p.id, Name: p.name, Source: p.source, Look: p.look,
		InArena: !p.detached, LeftBy: p.leftBy, StayLeft: a.stayLeft(p), RebootLeft: a.rebootLeft(p), RebootTotal: a.cfg.RebootBeats,
	}, nil
}

// stayLeft is the beats until p's automatic leave, -1 when detached or when
// stays are not limited.
func (a *Arena) stayLeft(p *player) int {
	if p.detached || a.cfg.MaxStay <= 0 {
		return -1
	}
	return max(0, a.cfg.MaxStay-p.stay)
}

// Inspect returns the live memory, last action and trail of the caller's bot.
// Before the bot enters the arena (its join lands at the next beat) InArena is
// false.
func (a *Arena) Inspect(token string) (BotState, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	p, err := a.auth(token)
	if err != nil {
		return BotState{}, err
	}
	if !p.inMatch || a.stopped {
		return BotState{Act: -1}, nil
	}
	return a.match.Inspect(p.id), nil
}

// PutBot validates a new program for the caller's bot and saves it. In a
// running game with RebootBeats set the bot reboots at the next beat (see
// Match.Reboot): the latest save wins and does not restart the countdown, and
// no rate limit applies. Without RebootBeats the program is hot-swapped at the
// next beat, at most once per SwapEvery beats (*RateLimitError). Invalid
// source gives a *ValidationError.
func (a *Arena) PutBot(token, source string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	p, err := a.auth(token)
	if err != nil {
		return err
	}
	prog, errs := lang.Validate(source)
	if len(errs) > 0 {
		return &ValidationError{Errs: errs}
	}
	if a.cfg.RebootBeats > 0 {
		a.saveForReboot(p, prog, source)
		return nil
	}
	return a.hotSwap(p, prog, source)
}

// hotSwap saves a program that lands at the next beat, rate-limited.
func (a *Arena) hotSwap(p *player, prog *lang.Program, source string) error {
	landing := a.match.World().Beat + 1
	// No beats play while stopped, so the limit would never lapse: the latest
	// edit wins and lands at the start.
	if wait := p.swappedAt + a.swapEvery() - landing; !a.stopped && p.swappedAt > 0 && wait > 0 {
		return &RateLimitError{RetryBeats: wait}
	}
	p.swappedAt, p.source = landing, source
	a.botsDirty = true
	if p.inMatch {
		a.queue.edits = append(a.queue.edits, pendingEdit{id: p.id, prog: prog, src: source})
	}
	return nil
}

// saveForReboot stores the program and, for a bot in a running game, queues
// the reboot. A stopped game or a bot outside the match just keeps the
// program: it runs from the next join or game start, without a reboot.
func (a *Arena) saveForReboot(p *player, prog *lang.Program, source string) {
	p.source = source
	a.botsDirty = true
	if p.inMatch && !a.stopped {
		a.queue.edits = append(a.queue.edits, pendingEdit{id: p.id, prog: prog, src: source})
	}
}

// rebootLeft is the beats until p is back from a reboot, -1 when it is not
// rebooting.
func (a *Arena) rebootLeft(p *player) int {
	for _, r := range a.match.World().Rebooting {
		if r.Bot.ID == p.id {
			return r.Left
		}
	}
	if slices.ContainsFunc(a.queue.edits, func(e pendingEdit) bool { return e.id == p.id }) && a.cfg.RebootBeats > 0 {
		return a.cfg.RebootBeats
	}
	return -1
}

// SetProfile renames the caller's bot (same rules as Join) and sets its look.
// It takes effect in the next published Update, the leaderboard line follows.
// It is not limited like a hot-swap; two calls in one beat leave the latest.
// House bots have no token, so they cannot be changed.
func (a *Arena) SetProfile(token, name string, look Look) error {
	name, err := cleanName(name)
	if err != nil {
		return err
	}
	if err := look.Validate(); err != nil {
		return err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	p, err := a.auth(token)
	if err != nil {
		return err
	}
	p.name, p.look = name, look
	a.botsDirty = true
	if p.inMatch && !a.stopped {
		a.boardDirty = a.board.SetName(p.id, name) || a.boardDirty
	}
	return nil
}

// dismiss revokes the player's token and queues its leave (or cancels its
// join). The leave lands at the next beat and is recorded like any other.
func (a *Arena) dismiss(p *player) {
	delete(a.tokens, p.token)
	a.botsDirty = true
	if !p.inMatch {
		a.queue.joins = slices.DeleteFunc(a.queue.joins, func(id string) bool { return id == p.id })
		delete(a.players, p.id)
		return
	}
	a.queueLeave(p)
}

// queueLeave queues the match leave of a bot in the match, once.
func (a *Arena) queueLeave(p *player) {
	if !p.leaving {
		p.leaving = true
		a.queue.leaves = append(a.queue.leaves, p.id)
	}
}

// BotInfo is one line of the admin listing.
type BotInfo struct {
	ID, Name string
	House    bool
	Detached bool // a human out of the arena, token and bot kept
	Score    int
}

// Bots lists every bot that is in the arena, waiting to join or detached, oldest first.
func (a *Arena) Bots() []BotInfo {
	a.mu.Lock()
	defer a.mu.Unlock()
	score := map[string]int{}
	for _, e := range a.board.Top() {
		score[e.Bot] = e.Score
	}
	var ps []*player
	for _, p := range a.players {
		if !p.kicked {
			ps = append(ps, p)
		}
	}
	slices.SortFunc(ps, func(x, y *player) int { return x.seq - y.seq })
	out := make([]BotInfo, len(ps))
	for i, p := range ps {
		out[i] = BotInfo{ID: p.id, Name: p.name, House: p.house, Detached: p.detached, Score: score[p.id]}
	}
	return out
}

// Kick removes a human bot by id, detached or not (at the next beat, recorded as
// a leave), revokes its token and deletes its leaderboard line. House bots
// cannot be kicked.
func (a *Arena) Kick(id string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	p, ok := a.players[id]
	if !ok || p.kicked {
		return ErrUnknownBot
	}
	if p.house {
		return ErrHouseBot
	}
	p.kicked = true
	a.dismiss(p)
	a.board.Remove(id)
	a.boardDirty = true
	return nil
}

// load restores the leaderboard and the players' bots from the Store.
func (a *Arena) load() error {
	entries, err := a.deps.Store.LoadBoard()
	if err != nil {
		return err
	}
	a.board = NewBoard(entries).WithPoints(a.points())
	if a.last, err = a.deps.Store.LoadLast(); err != nil {
		return err
	}
	bots, err := a.deps.Store.LoadBots()
	if err != nil {
		return err
	}
	slices.SortFunc(bots, func(x, y SavedBot) int { return strings.Compare(x.ID, y.ID) })
	for _, b := range bots {
		a.restore(b)
	}
	return nil
}

func (a *Arena) restore(b SavedBot) {
	if _, errs := lang.Validate(b.Source); len(errs) > 0 {
		a.botsDirty = true // drop the unusable bot from the store
		return
	}
	look := b.Look
	if look.Validate() != nil {
		look = Look{} // missing or unusable: the default look of the id
	}
	p := &player{id: b.ID, token: b.Token, name: b.Name, source: b.Source, look: look, stay: b.StayBeats}
	if b.Detached {
		p.detached, p.leftBy = true, b.LeftBy
		a.register(p)
		return
	}
	a.register(p)
	a.queue.joins = append(a.queue.joins, b.ID)
}

// saveJob is a copy of what changed, to be saved without the state lock.
type saveJob struct {
	board     []Entry
	bots      []SavedBot
	saveBoard bool // board is to be saved
	saveBots  bool // bots are to be saved
	saveMap   bool // text is to be saved
	text      string
	saveLast  bool // last is to be saved
	last      []Entry
}

// takeSave copies the dirty state and clears the dirty flags. Call it under
// the state lock. A failed save is marked dirty again by flush.
func (a *Arena) takeSave() saveJob {
	var j saveJob
	if j.saveBoard = a.boardDirty; j.saveBoard {
		j.board = a.board.Top()
	}
	if j.saveBots = a.botsDirty; j.saveBots {
		j.bots = a.savedBots()
	}
	if j.saveMap = a.mapDirty; j.saveMap {
		j.text = a.cfg.Map
	}
	if j.saveLast = a.lastDirty; j.saveLast {
		j.last = append([]Entry(nil), a.last...)
	}
	a.boardDirty, a.botsDirty, a.mapDirty, a.lastDirty = false, false, false, false
	return j
}

// flush saves a job to the Store without holding the state lock. What fails
// stays dirty and is retried by the next beat.
func (a *Arena) flush(j saveJob) error {
	var errs []error
	if j.saveBoard {
		errs = append(errs, a.retryLater(a.deps.Store.SaveBoard(j.board), &a.boardDirty))
	}
	if j.saveBots {
		errs = append(errs, a.retryLater(a.deps.Store.SaveBots(j.bots), &a.botsDirty))
	}
	if j.saveMap {
		errs = append(errs, a.retryLater(a.deps.Store.SaveMap(j.text), &a.mapDirty))
	}
	if j.saveLast {
		errs = append(errs, a.retryLater(a.deps.Store.SaveLast(j.last), &a.lastDirty))
	}
	return errors.Join(errs...)
}

// retryLater marks a flag dirty again after a failed save.
func (a *Arena) retryLater(err error, dirty *bool) error {
	if err != nil {
		a.mu.Lock()
		*dirty = true
		a.mu.Unlock()
	}
	return err
}

func (a *Arena) savedBots() []SavedBot {
	out := make([]SavedBot, 0, len(a.tokens))
	for token, id := range a.tokens {
		p := a.players[id]
		out = append(out, SavedBot{
			Token: token, ID: id, Name: p.name, Source: p.source, Look: p.look,
			Detached: p.detached, LeftBy: p.leftBy, StayBeats: p.stay,
		})
	}
	slices.SortFunc(out, func(x, y SavedBot) int { return strings.Compare(x.ID, y.ID) })
	return out
}

// roster lists the bots in the arena by id.
func (a *Arena) roster() []Player {
	out := make([]Player, 0, len(a.players))
	for _, p := range a.players {
		if p.inMatch {
			out = append(out, Player{ID: p.id, Name: p.name, House: p.house, Look: p.look})
		}
	}
	SortPlayers(out)
	return out
}

// edited maps each bot in the match to the beat its latest hot-swap landed
// on (bots never swapped are absent).
func (a *Arena) edited() map[string]int {
	out := map[string]int{}
	for id, p := range a.players {
		if p.inMatch && p.swappedAt > 0 {
			out[id] = p.swappedAt
		}
	}
	return out
}

func (a *Arena) points() Points {
	if a.cfg.Points != nil {
		return *a.cfg.Points
	}
	return DefaultPoints()
}
