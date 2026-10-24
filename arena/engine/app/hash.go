package app

import (
	"crypto/sha256"
	"encoding/hex"

	"github.com/lezhnev/arena/engine/domain/event"
)

// HashLog is the SHA-256 (hex) of the event log in canonical form: one
// event.JSONLine per event, each followed by a newline. The encoding has fixed
// field order and no maps, so equal logs always give equal hashes.
func HashLog(log []event.Logged) (string, error) {
	h := sha256.New()
	for _, l := range log {
		b, err := event.JSONLine(l)
		if err != nil {
			return "", err
		}
		h.Write(b)
		h.Write([]byte{'\n'})
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
