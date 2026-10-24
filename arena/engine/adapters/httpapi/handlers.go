package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/lezhnev/arena/engine/adapters/protocol"
	"github.com/lezhnev/arena/engine/app"
)

const (
	maxSourceBytes = 64 << 10
	maxJoinBytes   = 1 << 10
)

type handlers struct {
	svc  Service
	beat time.Duration // length of a beat, reported to the bot page
}

func (h *handlers) join(w http.ResponseWriter, r *http.Request) {
	var req protocol.JoinRequest
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxJoinBytes))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorBody("body must be JSON like {\"name\": \"Ann\"}"))
		return
	}
	token, id, err := h.svc.Join(req.Name)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, protocol.JoinResponse{Token: token, BotID: id})
}

func (h *handlers) getBot(w http.ResponseWriter, r *http.Request) {
	bot, ok := h.own(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, protocol.BotBody{
		ID: bot.ID, Name: bot.Name, Source: bot.Source, Look: protocol.FromLook(bot.Look),
		Arena:  protocol.BotArena{In: bot.InArena, LeftBy: bot.LeftBy, StayLeftBeats: bot.StayLeft},
		BeatMs: int(h.beat / time.Millisecond), RebootLeftBeats: bot.RebootLeft, RebootTotalBeats: bot.RebootTotal,
	})
}

// leave takes the caller's bot out of the arena (it stays registered).
func (h *handlers) leave(w http.ResponseWriter, r *http.Request) {
	h.done(w, h.svc.Leave(bearer(r)))
}

// returnBot brings the caller's detached bot back into the arena.
func (h *handlers) returnBot(w http.ResponseWriter, r *http.Request) {
	h.done(w, h.svc.Return(bearer(r)))
}

// done answers 204 for success, else the error's status.
func (h *handlers) done(w http.ResponseWriter, err error) {
	if err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *handlers) putBot(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.own(w, r); !ok {
		return
	}
	src, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxSourceBytes))
	if err != nil {
		writeError(w, err)
		return
	}
	if err := h.svc.PutBot(bearer(r), string(src)); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *handlers) putProfile(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.own(w, r); !ok {
		return
	}
	var req protocol.ProfileRequest
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxJoinBytes))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorBody("body must be JSON like {\"name\": \"Ann\", \"look\": {...}}"))
		return
	}
	if err := h.svc.SetProfile(bearer(r), req.Name, protocol.ToLook(req.Look)); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *handlers) state(w http.ResponseWriter, r *http.Request) {
	bot, ok := h.own(w, r)
	if !ok {
		return
	}
	st, err := h.svc.Inspect(bearer(r))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, protocol.FromBotState(bot.ID, st))
}

func (h *handlers) leaderboard(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, protocol.FromBoard(h.svc.Leaderboard()))
}

func (h *handlers) lastBoard(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, protocol.FromBoard(h.svc.LastBoard()))
}

// own authenticates the caller and, when the route names a bot ({id}),
// checks it is the caller's own: 401 for no or unknown token, 403 for
// someone else's bot. It answers the client itself when ok is false.
func (h *handlers) own(w http.ResponseWriter, r *http.Request) (app.BotView, bool) {
	bot, err := h.svc.GetBot(bearer(r))
	if err != nil {
		writeError(w, err)
		return bot, false
	}
	if id := r.PathValue("id"); id != "" && id != bot.ID {
		writeJSON(w, http.StatusForbidden, errorBody("that is not your bot"))
		return bot, false
	}
	return bot, true
}

// bearer extracts the token of "Authorization: Bearer <token>"; "" if absent.
func bearer(r *http.Request) string {
	token, _ := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	if token == r.Header.Get("Authorization") {
		return ""
	}
	return strings.TrimSpace(token)
}

func errorBody(msg string) protocol.ErrorBody { return protocol.ErrorBody{Error: msg} }

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v) // a failed write means the client is gone
}

// writeError maps an app error to its HTTP answer. Unknown errors become a
// bare 500: their text may hold paths or internals.
func writeError(w http.ResponseWriter, err error) {
	var (
		rate  *app.RateLimitError
		valid *app.ValidationError
		big   *http.MaxBytesError
	)
	switch {
	case errors.Is(err, app.ErrUnauthorized):
		w.Header().Set("WWW-Authenticate", "Bearer")
		writeJSON(w, http.StatusUnauthorized, errorBody("missing or unknown token: join first"))
	case errors.As(err, &valid):
		writeJSON(w, http.StatusUnprocessableEntity, protocol.FromValidation(valid))
	case errors.As(err, &rate):
		writeJSON(w, http.StatusTooManyRequests, protocol.ErrorBody{Error: err.Error(), RetryBeats: rate.RetryBeats})
	case errors.As(err, &big):
		writeJSON(w, http.StatusRequestEntityTooLarge, errorBody("bot source is too large"))
	default:
		writeStatusOf(w, err)
	}
}

func writeStatusOf(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, app.ErrBadName), errors.Is(err, app.ErrBadLook):
		writeJSON(w, http.StatusBadRequest, errorBody(err.Error()))
	case errors.Is(err, app.ErrFull):
		writeJSON(w, http.StatusConflict, errorBody(err.Error()))
	case errors.Is(err, app.ErrClosed):
		writeJSON(w, http.StatusServiceUnavailable, errorBody(err.Error()))
	default:
		writeJSON(w, http.StatusInternalServerError, errorBody("internal error"))
	}
}
