// Package ws broadcasts the arena to browsers over WebSocket: one JSON
// protocol.Message per beat, read-only (client messages are ignored).
package ws

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"

	"github.com/lezhnev/arena/engine/adapters/protocol"
	"github.com/lezhnev/arena/engine/app"
)

const (
	defaultQueue = 16
	writeTimeout = 10 * time.Second
)

// Options tune a Hub; zero values are fine.
type Options struct {
	Queue          int           // beats buffered per client before it is dropped; 0 = 16
	OriginPatterns []string      // extra allowed browser origins (host patterns); same-origin is always allowed
	Beat           time.Duration // length of a beat, sent to the UI to show the game timer as a clock; 0 = unknown
}

// client is one connected browser: a bounded queue of encoded messages.
type client struct {
	ch     chan []byte
	cancel context.CancelFunc
}

// Hub implements app.Broadcaster and serves GET /ws.
//
// Slow clients: Publish runs inside the beat and must never block, so every
// client has a bounded queue and a client whose queue is full is dropped
// (its connection is closed). Dropping a whole client beats dropping single
// frames: a UI that missed a beat has a wrong picture, while a reconnecting
// one gets a fresh snapshot.
type Hub struct {
	opts    Options
	mu      sync.Mutex
	last    app.Update
	clients map[*client]struct{}
}

// New returns a Hub whose snapshot is initial until the first beat.
func New(opts Options, initial app.Update) *Hub {
	if opts.Queue <= 0 {
		opts.Queue = defaultQueue
	}
	return &Hub{opts: opts, last: initial, clients: map[*client]struct{}{}}
}

// Reset replaces the snapshot sent to new clients (the state before beat 1).
func (h *Hub) Reset(u app.Update) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.last = u
}

// Publish implements app.Broadcaster: it encodes the beat once and queues it
// for every client, dropping those that cannot keep up.
func (h *Hub) Publish(u app.Update) {
	data, err := h.encode(u, protocol.TypeBeat)
	h.mu.Lock()
	defer h.mu.Unlock()
	h.last = u
	if err != nil {
		return // plain structs always encode; keep the snapshot fresh anyway
	}
	for c := range h.clients {
		select {
		case c.ch <- data:
		default:
			h.dropLocked(c)
		}
	}
}

// encode renders an update as a JSON message of the given type.
func (h *Hub) encode(u app.Update, typ string) ([]byte, error) {
	m := protocol.FromUpdate(u, typ)
	m.BeatMs = int(h.opts.Beat / time.Millisecond)
	return json.Marshal(m)
}

// Clients is the number of connected clients.
func (h *Hub) Clients() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.clients)
}

// Close disconnects every client.
func (h *Hub) Close() {
	h.mu.Lock()
	defer h.mu.Unlock()
	for c := range h.clients {
		h.dropLocked(c)
	}
}

func (h *Hub) dropLocked(c *client) {
	if _, ok := h.clients[c]; !ok {
		return
	}
	delete(h.clients, c)
	close(c.ch)
	c.cancel()
}

func (h *Hub) remove(c *client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.dropLocked(c)
}

// snapshotAndAdd registers a client and encodes the snapshot under one lock,
// so no beat falls between the snapshot and the queue.
func (h *Hub) snapshotAndAdd(cancel context.CancelFunc) (*client, []byte, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	data, err := h.encode(h.last, protocol.TypeSnapshot)
	if err != nil {
		return nil, nil, err
	}
	c := &client{ch: make(chan []byte, h.opts.Queue), cancel: cancel}
	h.clients[c] = struct{}{}
	return c, data, nil
}

// ServeHTTP upgrades the request, sends the snapshot and then every beat.
func (h *Hub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{OriginPatterns: h.opts.OriginPatterns})
	if err != nil {
		return // Accept has answered the client
	}
	ctx, cancel := context.WithCancel(conn.CloseRead(r.Context()))
	defer cancel()
	c, snap, err := h.snapshotAndAdd(cancel)
	if err != nil {
		_ = conn.Close(websocket.StatusInternalError, "snapshot")
		return
	}
	defer h.remove(c)
	h.pump(ctx, conn, c, snap)
}

// pump writes the snapshot, then queued beats until the client leaves or is dropped.
func (h *Hub) pump(ctx context.Context, conn *websocket.Conn, c *client, snap []byte) {
	if !write(ctx, conn, snap) {
		_ = conn.CloseNow()
		return
	}
	for {
		select {
		case data, ok := <-c.ch:
			if !ok || !write(ctx, conn, data) {
				_ = conn.Close(websocket.StatusPolicyViolation, "too slow or closing")
				return
			}
		case <-ctx.Done():
			_ = conn.CloseNow()
			return
		}
	}
}

func write(ctx context.Context, conn *websocket.Conn, data []byte) bool {
	ctx, cancel := context.WithTimeout(ctx, writeTimeout)
	defer cancel()
	return conn.Write(ctx, websocket.MessageText, data) == nil
}
