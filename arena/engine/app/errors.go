package app

import (
	"errors"
	"fmt"
	"strings"

	"github.com/lezhnev/arena/engine/lang"
)

// Errors of the live arena. The adapters map them to HTTP codes: Unauthorized
// 401/403, RateLimited 429, ValidationError 422, BadName 400.
var (
	ErrUnauthorized = errors.New("unknown token")
	ErrRateLimited  = errors.New("rate limited: one swap per few beats")
	ErrBadName      = errors.New("name must be 1 to 24 characters")
	ErrBadLook      = errors.New("bad look")
	ErrClosed       = errors.New("arena is closed")
	ErrFull         = errors.New("arena is full")
	ErrUnknownBot   = errors.New("no such bot")
	ErrHouseBot     = errors.New("house bots cannot be kicked")
	ErrMinBots      = errors.New("min bots must be between 0 and the floor cells of the arena")
	ErrBadMap       = errors.New("bad arena map")
	ErrMapTooSmall  = errors.New("the map has fewer floor cells than there are bots or house bots to fill")
	ErrRunning      = errors.New("the game is running: start a new game only after it is over")
	ErrNotRunning   = errors.New("the game is over: start a new game first")
	ErrBadTimer     = errors.New("timer must be 0 (off) or a positive number of beats")
	ErrIDCollision  = errors.New("could not draw an unused bot id and token")
)

// RateLimitError says how many beats to wait; it matches ErrRateLimited.
type RateLimitError struct{ RetryBeats int }

func (e *RateLimitError) Error() string {
	return fmt.Sprintf("%v (try again in %d beats)", ErrRateLimited, e.RetryBeats)
}

// Is makes errors.Is(err, ErrRateLimited) true.
func (e *RateLimitError) Is(target error) bool { return target == ErrRateLimited }

// ValidationError carries every problem of a rejected program, each with line,
// column and a hint.
type ValidationError struct{ Errs []lang.Error }

func (e *ValidationError) Error() string {
	parts := make([]string, len(e.Errs))
	for i, x := range e.Errs {
		parts[i] = x.Error()
	}
	return "invalid program: " + strings.Join(parts, "; ")
}
