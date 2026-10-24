package httpapi

import (
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/lezhnev/arena/engine/adapters/ids"
	"github.com/lezhnev/arena/engine/adapters/protocol"
	"github.com/lezhnev/arena/engine/app"
	"github.com/lezhnev/arena/engine/domain/geo"
)

// AdminService is the part of app.Arena the admin routes use.
type AdminService interface {
	Bots() []app.BotInfo
	Kick(id string) error
	ResetBoard() error
	SetMinBots(n int) error
	MinBots() int
	Beat() int
	MapSize() (width, height int)
	ApplyMap(text string) error
	SetTimer(beats int) error
	StartGame() error
	GameState() (running bool, remaining int)
}

// AdminOptions turn the admin API on. With an empty Password or a nil Service
// the admin is disabled: /api/admin/* answers 404. Now, Sleep and
// NewSessionID default to the real clock, time.Sleep and random tokens; the
// tests replace them.
type AdminOptions struct {
	Service      AdminService
	Password     string
	Now          func() time.Time
	Sleep        func(time.Duration)
	NewSessionID func() string
	NewSeed      func() uint64 // seed of a map preview that names none; default: random
	Beat         time.Duration // length of a beat: turns the timer's minutes into beats
}

func (o AdminOptions) enabled() bool { return o.Password != "" && o.Service != nil }

type adminHandlers struct {
	beat    time.Duration
	svc     AdminService
	auth    *adminAuth
	newSeed func() uint64
	proxies proxies
}

// registerAdmin adds the admin routes; the mutating ones are POST only.
func registerAdmin(mux *http.ServeMux, o AdminOptions, p proxies) {
	h := &adminHandlers{beat: o.Beat, svc: o.Service, auth: newAdminAuth(o), newSeed: o.NewSeed, proxies: p}
	if h.newSeed == nil {
		h.newSeed = ids.New().Seed
	}
	mux.HandleFunc("POST /api/admin/login", h.login)
	mux.HandleFunc("POST /api/admin/logout", h.logout)
	mux.HandleFunc("GET /api/admin/status", h.guard(h.status))
	mux.HandleFunc("POST /api/admin/kick", h.guard(h.kick))
	mux.HandleFunc("POST /api/admin/reset-board", h.guard(h.resetBoard))
	mux.HandleFunc("POST /api/admin/min-bots", h.guard(h.minBots))
	mux.HandleFunc("POST /api/admin/map/preview", h.guard(h.mapPreview))
	mux.HandleFunc("POST /api/admin/map/apply", h.guard(h.mapApply))
	mux.HandleFunc("POST /api/admin/timer", h.guard(h.timer))
	mux.HandleFunc("POST /api/admin/start", h.guard(h.start))
	for pattern, allow := range map[string]string{
		"/api/admin/login": "POST", "/api/admin/logout": "POST", "/api/admin/status": "GET",
		"/api/admin/kick": "POST", "/api/admin/reset-board": "POST", "/api/admin/min-bots": "POST",
		"/api/admin/map/preview": "POST", "/api/admin/map/apply": "POST",
		"/api/admin/timer": "POST", "/api/admin/start": "POST",
	} {
		mux.HandleFunc(pattern, methodNotAllowed(allow))
	}
}

// guard lets a request through only with a live admin session.
func (h *adminHandlers) guard(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !h.auth.valid(r) {
			writeJSON(w, http.StatusUnauthorized, errorBody("admin login required"))
			return
		}
		next(w, r)
	}
}

func (h *adminHandlers) login(w http.ResponseWriter, r *http.Request) {
	var req protocol.AdminLoginRequest
	if !decodeBody(w, r, &req) {
		return
	}
	ok, wait := h.auth.attempt(h.proxies.clientHost(r), req.Password)
	if wait > 0 {
		w.Header().Set("Retry-After", strconv.Itoa(int(wait.Round(time.Second).Seconds())))
		writeJSON(w, http.StatusTooManyRequests, errorBody("too many failed logins: try again in a minute"))
		return
	}
	if !ok {
		writeJSON(w, http.StatusUnauthorized, errorBody("wrong password"))
		return
	}
	id, exp := h.auth.open()
	http.SetCookie(w, sessionCookie(id, exp, int(sessionTTL.Seconds()), h.proxies.https(r)))
	w.WriteHeader(http.StatusNoContent)
}

func (h *adminHandlers) logout(w http.ResponseWriter, r *http.Request) {
	h.auth.close(r)
	http.SetCookie(w, sessionCookie("", time.Unix(0, 0), -1, h.proxies.https(r)))
	w.WriteHeader(http.StatusNoContent)
}

func (h *adminHandlers) status(w http.ResponseWriter, _ *http.Request) {
	bots := h.svc.Bots()
	out := protocol.AdminStatus{Bots: make([]protocol.AdminBot, len(bots)), MinBots: h.svc.MinBots(), Beat: h.svc.Beat()}
	out.MapWidth, out.MapHeight = h.svc.MapSize()
	out.Running, out.Remaining = h.svc.GameState()
	out.BeatMs = int(h.beat / time.Millisecond)
	for i, b := range bots {
		out.Bots[i] = protocol.AdminBot{ID: b.ID, Name: b.Name, House: b.House, Score: b.Score, Detached: b.Detached}
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *adminHandlers) kick(w http.ResponseWriter, r *http.Request) {
	var req protocol.AdminKickRequest
	if !decodeBody(w, r, &req) {
		return
	}
	h.done(w, h.svc.Kick(req.ID))
}

func (h *adminHandlers) resetBoard(w http.ResponseWriter, _ *http.Request) {
	h.done(w, h.svc.ResetBoard())
}

func (h *adminHandlers) minBots(w http.ResponseWriter, r *http.Request) {
	var req protocol.AdminMinBotsRequest
	if !decodeBody(w, r, &req) {
		return
	}
	h.done(w, h.svc.SetMinBots(req.N))
}

func (h *adminHandlers) mapPreview(w http.ResponseWriter, r *http.Request) {
	var req protocol.AdminMapPreviewRequest
	if !decodeBody(w, r, &req) {
		return
	}
	seed := h.newSeed()
	if req.Seed != nil {
		seed = *req.Seed
	}
	walls := geo.DefaultWallPercent
	if req.Walls != nil {
		walls = *req.Walls
	}
	text, err := geo.Generate(req.Width, req.Height, walls, seed)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorBody(err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, protocol.AdminMapPreview{Seed: seed, Map: text})
}

func (h *adminHandlers) mapApply(w http.ResponseWriter, r *http.Request) {
	var req protocol.AdminMapApplyRequest
	if !decodeBody(w, r, &req) {
		return
	}
	h.done(w, h.svc.ApplyMap(req.Map))
}

// maxTimerMinutes caps the game timer at one day.
const maxTimerMinutes = 24 * 60

func (h *adminHandlers) timer(w http.ResponseWriter, r *http.Request) {
	var req protocol.AdminTimerRequest
	if !decodeBody(w, r, &req) {
		return
	}
	if !(req.Minutes >= 0 && req.Minutes <= maxTimerMinutes) || h.beat <= 0 {
		writeJSON(w, http.StatusBadRequest, errorBody("minutes must be between 0 and 1440"))
		return
	}
	h.done(w, h.svc.SetTimer(MinutesToBeats(req.Minutes, h.beat)))
}

// MinutesToBeats rounds a duration up to whole beats (at least one unless it is 0).
func MinutesToBeats(minutes float64, beat time.Duration) int {
	return int(math.Ceil(minutes*float64(time.Minute)/float64(beat) - 1e-9))
}

func (h *adminHandlers) start(w http.ResponseWriter, _ *http.Request) {
	h.done(w, h.svc.StartGame())
}

// done answers 204, or the HTTP status of an admin command's error.
func (h *adminHandlers) done(w http.ResponseWriter, err error) {
	switch {
	case err == nil:
		w.WriteHeader(http.StatusNoContent)
	case errors.Is(err, app.ErrUnknownBot):
		writeJSON(w, http.StatusNotFound, errorBody(err.Error()))
	case errors.Is(err, app.ErrHouseBot):
		writeJSON(w, http.StatusConflict, errorBody(err.Error()))
	case errors.Is(err, app.ErrMinBots), errors.Is(err, app.ErrBadMap), errors.Is(err, app.ErrBadTimer):
		writeJSON(w, http.StatusBadRequest, errorBody(err.Error()))
	case errors.Is(err, app.ErrMapTooSmall), errors.Is(err, app.ErrRunning), errors.Is(err, app.ErrNotRunning):
		writeJSON(w, http.StatusConflict, errorBody(err.Error()))
	default:
		writeError(w, err)
	}
}

// decodeBody reads a small JSON body into v; false means it answered 400.
func decodeBody(w http.ResponseWriter, r *http.Request, v any) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxLoginBytes))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		writeJSON(w, http.StatusBadRequest, errorBody("body must be JSON of the documented shape"))
		return false
	}
	return true
}
