package app

import "slices"

// stayFlushBeats is how often the stay counts reach the Store, so a restart
// loses at most this many beats of a stay.
const stayFlushBeats = 60

// Leave takes the caller's bot out of the arena at the next beat. The bot is
// detached, not deleted: the token stays valid and the name, look, program and
// leaderboard line are kept until Return. The leave is recorded like any other.
// Leaving a detached bot does nothing; a change too soon after the previous
// leave or return gives a *RateLimitError (not while the game is stopped).
func (a *Arena) Leave(token string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	p, err := a.auth(token)
	if err != nil || p.detached {
		return err
	}
	if p.movedAt, err = a.moveLanding(p, a.swapEvery()); err != nil {
		return err
	}
	a.detach(p, LeftByPlayer)
	return nil
}

// Return brings a detached bot back: it joins at the next beat like a fresh
// join (new spawn, fresh HP and memory) with its saved program and look, and a
// new stay begins. A bot already in the arena does nothing. A full arena gives
// ErrFull, a change too soon after the previous one a *RateLimitError. A bot
// that left by its own Leave waits RebootBeats beats after the leave landed.
func (a *Arena) Return(token string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	p, err := a.auth(token)
	if err != nil || !p.detached {
		return err
	}
	landing, err := a.moveLanding(p, a.returnGap(p))
	if err != nil {
		return err
	}
	if a.humans() >= a.playerCap() {
		return ErrFull
	}
	p.movedAt, p.detached, p.leftBy, p.stay = landing, false, "", 0
	a.botsDirty = true
	if p.leaving { // the leave has not landed yet: the bot never goes
		p.leaving = false
		a.queue.leaves = slices.DeleteFunc(a.queue.leaves, func(id string) bool { return id == p.id })
		return nil
	}
	a.queue.joins = append(a.queue.joins, p.id)
	return nil
}

// moveLanding is the beat a leave or return called now lands on, or the
// rate-limit error when the previous one landed less than gap beats ago. No
// beats play while stopped, so the limit never lapses there: it does not apply.
func (a *Arena) moveLanding(p *player, gap int) (int, error) {
	landing := a.match.World().Beat + 1
	if wait := p.movedAt + gap - landing; !a.stopped && p.movedAt > 0 && wait > 0 {
		return 0, &RateLimitError{RetryBeats: wait}
	}
	return landing, nil
}

// returnGap is the beats a return must trail the leave. A program saved while
// detached must not skip the reboot, so a bot that left by its own Leave waits
// RebootBeats at least. Automatic leaves and a leave that has not landed yet
// (the return just cancels it) only face the usual limit.
func (a *Arena) returnGap(p *player) int {
	if p.leftBy == LeftByPlayer && !p.leaving {
		return max(a.swapEvery(), a.cfg.RebootBeats)
	}
	return a.swapEvery()
}

// detach marks the player as out of the arena. A bot in the match gets a
// queued leave, a bot still waiting to join loses its queued join.
func (a *Arena) detach(p *player, by string) {
	p.detached, p.leftBy = true, by
	a.botsDirty = true
	if p.inMatch {
		a.queueLeave(p)
		return
	}
	a.queue.joins = slices.DeleteFunc(a.queue.joins, func(id string) bool { return id == p.id })
}

// countStays adds the beat just played to the stay of every human in the
// match and detaches those whose stay is over. They leave at the next beat,
// oldest first. Callers hold the lock.
func (a *Arena) countStays(beat int) {
	if a.cfg.MaxStay <= 0 {
		return
	}
	counted := 0
	for _, p := range a.bySeq() {
		if p.countsStay() {
			counted++
			a.addStay(p)
		}
	}
	if counted > 0 && beat%stayFlushBeats == 0 {
		a.botsDirty = true
	}
}

// countsStay reports whether p is a human in the match that is not on its way out.
func (p *player) countsStay() bool { return !p.house && p.inMatch && !p.detached }

// addStay counts one beat of p's stay and detaches it when the stay is over.
func (a *Arena) addStay(p *player) {
	if p.stay++; p.stay >= a.cfg.MaxStay {
		a.detach(p, LeftByTime)
	}
}
