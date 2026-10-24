// Package httpapi is the REST API of the arena plus the static web app.
//
// Routes:
//
//	POST /api/join             {name}            -> {token, botId}
//	GET  /api/bot              Bearer            -> {id, name, source, look, arena{in, leftBy, stayLeftBeats}, beatMs}
//	POST /api/bot/leave        Bearer            -> 204 (detach: the bot leaves the arena, token and board line stay; 429 + retryBeats)
//	POST /api/bot/return       Bearer            -> 204 (back into the arena; 409 when full; 429 + retryBeats)
//	PUT  /api/bot              Bearer, body=src  -> 204
//	PUT  /api/bot/profile      Bearer {name, look} -> 204 (400 with the bad field; 401/403 as the bot routes)
//	GET  /api/bot/{id}         as /api/bot, 403 unless {id} is the caller's bot
//	PUT  /api/bot/{id}         as /api/bot, 403 unless {id} is the caller's bot
//	GET  /api/state            Bearer            -> live memory, last act and trail
//	GET  /api/leaderboard                        -> [{bot, name, score, ...}]
//	GET  /api/leaderboard/last                   -> the final board of the latest finished game
//	POST /api/check            body=src          -> {ok, errors | trial} (public, per-IP rate limit; Options.Check)
//	GET  /api/check?src=...                      -> the same, for tools that can only fetch URLs
//	GET  /api/config                             -> {baseUrl} (Options.BaseURL, "" when unset)
//	POST /api/admin/login      {password}        -> 204 + session cookie (Options.Admin)
//	POST /api/admin/logout                       -> 204
//	GET  /api/admin/status     cookie            -> {bots, minBots, beat, running, remaining, ...}
//	POST /api/admin/timer      cookie {minutes}  -> 204 (0 cancels; 409 when the game is over)
//	POST /api/admin/start      cookie            -> 204 (409 while a game runs)
//	POST /api/admin/kick       cookie {id}       -> 204
//	POST /api/admin/reset-board cookie           -> 204
//	POST /api/admin/min-bots   cookie {n}        -> 204
//	GET  /ws                                     -> WebSocket (Options.WS)
//	GET  /, /bot, /leaderboard, /admin, assets           -> the web app
package httpapi

import (
	"net/http"
	"net/netip"
	"time"

	"github.com/lezhnev/arena/engine/app"
)

// Service is the part of app.Arena the API uses.
type Service interface {
	Join(name string) (token, botID string, err error)
	GetBot(token string) (app.BotView, error)
	PutBot(token, source string) error
	SetProfile(token, name string, look app.Look) error
	Leave(token string) error
	Return(token string) error
	Inspect(token string) (app.BotState, error)
	Leaderboard() []app.Entry
	LastBoard() []app.Entry
}

// Options configure the handler.
type Options struct {
	WS     http.Handler // serves GET /ws; nil = no such route
	WebDir string       // the built web app (web/dist); "" or missing = a placeholder page
	Admin  AdminOptions // the /api/admin/* routes; disabled (404) without a password
	// Check serves /api/check; nil = no such route.
	Check CheckFunc
	// BaseURL is the public address of the server for GET /api/config ("" = not configured).
	BaseURL string
	// Beat is the length of a beat, reported by GET /api/bot (beatMs).
	Beat time.Duration
	// Now is the clock of the /api/check rate limit; nil = time.Now.
	Now func() time.Time
	// TrustedProxies are the reverse proxies (e.g. nginx) whose X-Forwarded-For
	// names the client for the per-IP limits and whose X-Forwarded-Proto: https
	// makes the admin cookie Secure; empty = the headers are ignored.
	TrustedProxies []netip.Prefix
}

// New returns the HTTP handler of the arena.
func New(svc Service, opts Options) http.Handler {
	h := &handlers{svc: svc, beat: opts.Beat}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/join", h.join)
	mux.HandleFunc("GET /api/bot", h.getBot)
	mux.HandleFunc("PUT /api/bot", h.putBot)
	mux.HandleFunc("PUT /api/bot/profile", h.putProfile)
	mux.HandleFunc("GET /api/bot/profile", methodNotAllowed("PUT")) // not a bot id
	mux.HandleFunc("POST /api/bot/leave", h.leave)
	mux.HandleFunc("POST /api/bot/return", h.returnBot)
	for _, path := range []string{"/api/bot/leave", "/api/bot/return"} { // not bot ids
		mux.HandleFunc("GET "+path, methodNotAllowed("POST"))
		mux.HandleFunc("PUT "+path, methodNotAllowed("POST"))
	}
	mux.HandleFunc("GET /api/bot/{id}", h.getBot)
	mux.HandleFunc("PUT /api/bot/{id}", h.putBot)
	mux.HandleFunc("GET /api/state", h.state)
	mux.HandleFunc("GET /api/leaderboard", h.leaderboard)
	mux.HandleFunc("GET /api/leaderboard/last", h.lastBoard)
	mux.HandleFunc("GET /api/config", configHandler(opts.BaseURL))
	registerCheck(mux, opts)
	for pattern, allow := range map[string]string{
		"/api/join": "POST", "/api/bot": "GET, PUT", "/api/bot/{id}": "GET, PUT",
		"/api/state": "GET", "/api/leaderboard": "GET", "/api/leaderboard/last": "GET", "/api/config": "GET",
	} {
		mux.HandleFunc(pattern, methodNotAllowed(allow))
	}
	if opts.Admin.enabled() {
		registerAdmin(mux, opts.Admin, opts.TrustedProxies)
	}
	mux.HandleFunc("/api/", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusNotFound, errorBody("no such endpoint"))
	})
	if opts.WS != nil {
		mux.Handle("GET /ws", opts.WS)
	}
	mux.Handle("/", staticHandler(opts.WebDir))
	return mux
}

// methodNotAllowed answers 405 with an Allow header; the method-specific
// routes registered for the same path win for their methods.
func methodNotAllowed(allow string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Allow", allow)
		writeJSON(w, http.StatusMethodNotAllowed, errorBody("method not allowed"))
	}
}

// registerCheck mounts /api/check (GET and POST) when a checker is given.
func registerCheck(mux *http.ServeMux, opts Options) {
	if opts.Check == nil {
		return
	}
	c := &checkHandlers{check: opts.Check, limiter: newRateLimiter(checkLimit, checkWindow, maxCheckHosts, opts.Now), slots: make(chan struct{}, maxTrials), proxies: opts.TrustedProxies}
	mux.HandleFunc("GET /api/check", c.serve)
	mux.HandleFunc("POST /api/check", c.serve)
	mux.HandleFunc("/api/check", methodNotAllowed("GET, POST"))
}
