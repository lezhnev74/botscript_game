package app

// ResetBoard wipes the live leaderboard. The bots in the arena get a fresh
// zero line (their names stay), everything else is gone. The wipe is saved on
// the next beat and shows in that beat's broadcast. A stopped game has no live
// board to wipe (the next start empties it): ErrNotRunning.
func (a *Arena) ResetBoard() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.stopped {
		return ErrNotRunning
	}
	a.board.Reset()
	for _, p := range a.players {
		if p.inMatch {
			a.board.SetName(p.id, p.name)
		}
	}
	a.boardDirty = true
	return nil
}

// SetMinBots sets how many bots the arena is filled up to with house bots. It
// takes effect at the next beat through the usual balance. n is 0 to the
// number of floor cells.
func (a *Arena) SetMinBots(n int) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if n < 0 || n > a.currentFloor() {
		return ErrMinBots
	}
	a.cfg.MinBots = n
	return nil
}

// MinBots is the current house-bot fill level.
func (a *Arena) MinBots() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.cfg.MinBots
}

// MapSize is the width and height of the map in play (a pending ApplyMap
// shows from the beat it starts on).
func (a *Arena) MapSize() (width, height int) {
	a.mu.Lock()
	defer a.mu.Unlock()
	g := a.match.World().Grid
	return g.Width(), g.Height()
}

// Beat is the number of the last played beat.
func (a *Arena) Beat() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.match.World().Beat
}
