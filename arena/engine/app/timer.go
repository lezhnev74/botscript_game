package app

import "github.com/lezhnev/arena/engine/domain/geo"

// noTimer is Update.Remaining (and the internal countdown) when no timer runs.
const noTimer = -1

// SetTimer starts a countdown of the given number of beats (0 cancels it).
// When it reaches zero at a beat boundary the game ends: no more beats are
// played, the live board becomes the last game board and the update of that
// beat carries GameOver. A stopped game takes no timer; start a new one first.
func (a *Arena) SetTimer(beats int) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closed {
		return ErrClosed
	}
	if beats < 0 {
		return ErrBadTimer
	}
	if a.stopped {
		return ErrNotRunning
	}
	a.remaining = beats
	if beats == 0 {
		a.remaining = noTimer
	}
	return nil
}

// GameState says whether a game is running and the beats left on its timer
// (-1 = no timer, 0 once the game is over).
func (a *Arena) GameState() (running bool, remaining int) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return !a.stopped, a.remaining
}

// LastBoard is the final leaderboard of the latest finished game, best first
// (empty when no game has ended yet).
func (a *Arena) LastBoard() []Entry {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]Entry{}, a.last...)
}

// StartGame starts a fresh game after one has ended: an empty live board, a new
// match on the current map (or on a map applied meanwhile) with seed + 1, the
// same players and programs, and no timer. Joins, edits, kicks and ApplyMap
// accepted while the game was over apply here. The first beat plays at the
// next Tick.
func (a *Arena) StartGame() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closed {
		return ErrClosed
	}
	if !a.stopped {
		return ErrRunning
	}
	if a.nextMap == nil {
		m, _ := geo.ParseMap(a.cfg.Map) // it parsed when it became the map in play
		a.nextMap = &nextMap{text: a.cfg.Map, m: m}
	}
	a.startGame()
	a.board.Reset()
	a.boardDirty = true
	a.stopped, a.remaining = false, noTimer
	return nil
}

// countDown ends a beat on the timer: the last beat stops the game. It returns
// true when the game is over.
func (a *Arena) countDown() bool {
	if a.remaining == noTimer {
		return false
	}
	a.remaining--
	if a.remaining > 0 {
		return false
	}
	a.stopGame()
	return true
}

// stopGame freezes the game: the bots' goroutines end, the board is archived.
func (a *Arena) stopGame() {
	a.stopped = true
	a.match.Close()
	a.last, a.lastDirty = a.board.Top(), true
}
