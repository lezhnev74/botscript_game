package httpapi

import (
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/lezhnev/arena/engine/adapters/protocol"
	"github.com/lezhnev/arena/engine/app"
)

const (
	maxCheckBytes = 16 << 10 // a program sent to /api/check, as body or as the src parameter
	checkLimit    = 30       // requests per remote host per window
	checkWindow   = time.Minute
	maxCheckHosts = 1024
	maxTrials     = 4 // checks running at once, over all hosts
)

// CheckFunc checks a program: validation errors, or a trial of it.
type CheckFunc func(src string) (app.CheckResult, error)

// rateLimiter allows limit requests per host in a sliding window. The table of
// hosts is bounded: when it is full of hosts with fresh requests, a new host
// is refused until the oldest request leaves the window. Time comes from now
// so the tests control it.
type rateLimiter struct {
	limit    int
	window   time.Duration
	maxHosts int
	now      func() time.Time

	mu   sync.Mutex
	hits map[string][]time.Time
}

func newRateLimiter(limit int, window time.Duration, maxHosts int, now func() time.Time) *rateLimiter {
	if now == nil {
		now = time.Now
	}
	return &rateLimiter{limit: limit, window: window, maxHosts: maxHosts, now: now, hits: map[string][]time.Time{}}
}

// allow records a request of host. When it is refused, wait is how long until
// a request would pass.
func (l *rateLimiter) allow(host string) (ok bool, wait time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	recent := l.recent(host)
	if len(recent) >= l.limit {
		return false, l.untilFree(recent[0])
	}
	if len(recent) == 0 && len(l.hits) >= l.maxHosts {
		l.sweep()
		if len(l.hits) >= l.maxHosts {
			return false, l.untilFree(l.oldest())
		}
	}
	l.hits[host] = append(recent, l.now())
	return true, 0
}

func (l *rateLimiter) untilFree(t time.Time) time.Duration {
	return max(t.Add(l.window).Sub(l.now()), time.Second)
}

func (l *rateLimiter) oldest() time.Time {
	oldest := l.now()
	for _, ts := range l.hits {
		if ts[0].Before(oldest) {
			oldest = ts[0]
		}
	}
	return oldest
}

func (l *rateLimiter) sweep() {
	for host := range l.hits {
		l.recent(host)
	}
}

// recent drops the host's requests older than the window and returns the rest.
func (l *rateLimiter) recent(host string) []time.Time {
	cutoff := l.now().Add(-l.window)
	var keep []time.Time
	for _, t := range l.hits[host] {
		if t.After(cutoff) {
			keep = append(keep, t)
		}
	}
	if keep == nil {
		delete(l.hits, host)
	} else {
		l.hits[host] = keep
	}
	return keep
}

type checkHandlers struct {
	check   CheckFunc
	limiter *rateLimiter
	slots   chan struct{} // one token per running check
	proxies proxies
}

// serve answers POST (raw body) and GET (?src=) alike.
func (c *checkHandlers) serve(w http.ResponseWriter, r *http.Request) {
	if ok, wait := c.limiter.allow(c.proxies.clientHost(r)); !ok {
		w.Header().Set("Retry-After", strconv.Itoa(int(wait.Seconds())+1))
		writeJSON(w, http.StatusTooManyRequests, errorBody("too many checks: wait a minute"))
		return
	}
	src, status, msg := programOf(w, r)
	if status != 0 {
		writeJSON(w, status, errorBody(msg))
		return
	}
	select {
	case c.slots <- struct{}{}:
		defer func() { <-c.slots }()
	default:
		w.Header().Set("Retry-After", "1")
		writeJSON(w, http.StatusServiceUnavailable, errorBody("busy: try again in a second"))
		return
	}
	res, err := c.check(src)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorBody("internal error"))
		return
	}
	writeJSON(w, http.StatusOK, protocol.FromCheck(res))
}

// programOf reads the program of a check request; status != 0 is the failure to answer.
func programOf(w http.ResponseWriter, r *http.Request) (src string, status int, msg string) {
	if r.Method == http.MethodGet {
		return programParam(r.URL.Query())
	}
	b, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxCheckBytes))
	var tooBig *http.MaxBytesError
	switch {
	case errors.As(err, &tooBig):
		return "", http.StatusRequestEntityTooLarge, "program is too large (16 KB at most)"
	case err != nil:
		return "", http.StatusBadRequest, "could not read the request body"
	}
	return string(b), 0, ""
}

func programParam(q url.Values) (string, int, string) {
	if !q.Has("src") {
		return "", http.StatusBadRequest, "pass the program as ?src=<url-encoded program> or POST it as the body"
	}
	src := q.Get("src")
	if len(src) > maxCheckBytes {
		return "", http.StatusBadRequest, "program is too large (16 KB at most)"
	}
	return src, 0, ""
}

func configHandler(baseURL string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, protocol.ConfigBody{BaseURL: baseURL})
	}
}
