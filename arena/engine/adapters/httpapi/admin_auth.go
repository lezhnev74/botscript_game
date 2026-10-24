package httpapi

import (
	"crypto/sha256"
	"crypto/subtle"
	"net/http"
	"sync"
	"time"

	"github.com/lezhnev/arena/engine/adapters/ids"
)

const (
	adminCookie     = "arena_admin"
	adminPath       = "/api/admin"
	sessionTTL      = 12 * time.Hour
	maxFailures     = 5
	failureWindow   = time.Minute
	failDelay       = 500 * time.Millisecond
	maxLoginBytes   = 1 << 10
	maxFailureHosts = 1024
)

// adminAuth holds the admin sessions and the failed-login history, both in
// memory (a restart logs everybody out). Time comes from now and sleep so the
// tests control it.
type adminAuth struct {
	password [sha256.Size]byte
	now      func() time.Time
	sleep    func(time.Duration)
	newID    func() string

	mu       sync.Mutex
	sessions map[string]time.Time   // session id -> expiry
	fails    map[string][]time.Time // remote host -> recent failure times
}

func newAdminAuth(o AdminOptions) *adminAuth {
	a := &adminAuth{
		password: sha256.Sum256([]byte(o.Password)),
		now:      o.Now, sleep: o.Sleep, newID: o.NewSessionID,
		sessions: map[string]time.Time{}, fails: map[string][]time.Time{},
	}
	if a.now == nil {
		a.now = time.Now
	}
	if a.sleep == nil {
		a.sleep = time.Sleep
	}
	if a.newID == nil {
		a.newID = ids.New().Token
	}
	return a
}

// check compares in constant time (hashing first hides the length).
func (a *adminAuth) check(password string) bool {
	sum := sha256.Sum256([]byte(password))
	return subtle.ConstantTimeCompare(sum[:], a.password[:]) == 1
}

// attempt decides a login in one critical section, so parallel requests cannot
// all pass the check before any failure is recorded. wait > 0 means the host is
// refused (nothing is checked or recorded). Otherwise ok tells whether the
// password was right; a wrong one is recorded and then delayed, after unlocking.
// A host with a full table of fresh strangers is refused too: its failures
// could not be recorded.
func (a *adminAuth) attempt(host, password string) (ok bool, wait time.Duration) {
	a.mu.Lock()
	if wait = a.blockedLocked(host); wait > 0 {
		a.mu.Unlock()
		return false, wait
	}
	ok = a.check(password)
	if !ok {
		a.fails[host] = append(a.recentFails(host), a.now())
	}
	a.mu.Unlock()
	if !ok {
		a.sleep(failDelay)
	}
	return ok, 0
}

func (a *adminAuth) blockedLocked(host string) time.Duration {
	recent := a.recentFails(host)
	if len(recent) >= maxFailures {
		return a.untilFree(recent[0])
	}
	if len(recent) == 0 && len(a.fails) >= maxFailureHosts {
		a.sweep()
		if len(a.fails) >= maxFailureHosts {
			return a.untilFree(a.oldestFail())
		}
	}
	return 0
}

// untilFree is the time left until a failure at t leaves the window.
func (a *adminAuth) untilFree(t time.Time) time.Duration {
	return max(t.Add(failureWindow).Sub(a.now()), time.Second)
}

// oldestFail is the earliest failure of any host (the table is not empty).
func (a *adminAuth) oldestFail() time.Time {
	oldest := a.now()
	for _, ts := range a.fails {
		oldest = minTime(oldest, ts[0])
	}
	return oldest
}

func minTime(x, y time.Time) time.Time {
	if y.Before(x) {
		return y
	}
	return x
}

// sweep drops every host whose failures are all out of the window.
func (a *adminAuth) sweep() {
	for host := range a.fails {
		a.recentFails(host)
	}
}

// recentFails drops the host's failures older than the window and returns the rest.
func (a *adminAuth) recentFails(host string) []time.Time {
	cutoff := a.now().Add(-failureWindow)
	var keep []time.Time
	for _, t := range a.fails[host] {
		if t.After(cutoff) {
			keep = append(keep, t)
		}
	}
	if keep == nil {
		delete(a.fails, host)
	} else {
		a.fails[host] = keep
	}
	return keep
}

// open starts a session and returns its id; expired sessions are dropped.
func (a *adminAuth) open() (string, time.Time) {
	a.mu.Lock()
	defer a.mu.Unlock()
	now := a.now()
	for id, exp := range a.sessions {
		if !exp.After(now) {
			delete(a.sessions, id)
		}
	}
	id, exp := a.newID(), now.Add(sessionTTL)
	a.sessions[id] = exp
	return id, exp
}

// valid reports whether the request carries a live session cookie.
func (a *adminAuth) valid(r *http.Request) bool {
	c, err := r.Cookie(adminCookie)
	if err != nil {
		return false
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	exp, ok := a.sessions[c.Value]
	return ok && exp.After(a.now())
}

func (a *adminAuth) close(r *http.Request) {
	if c, err := r.Cookie(adminCookie); err == nil {
		a.mu.Lock()
		delete(a.sessions, c.Value)
		a.mu.Unlock()
	}
}

// sessionCookie is the admin cookie. It is Secure only when the client came
// over HTTPS: on a LAN the arena runs over plain http, and a Secure-only
// cookie would never be sent back. HttpOnly and SameSite=Strict (the CSRF
// guard of the POST routes) always apply.
func sessionCookie(value string, expires time.Time, maxAge int, secure bool) *http.Cookie {
	return &http.Cookie{
		Name: adminCookie, Value: value, Path: adminPath, Expires: expires,
		MaxAge: maxAge, HttpOnly: true, SameSite: http.SameSiteStrictMode, Secure: secure,
	}
}
