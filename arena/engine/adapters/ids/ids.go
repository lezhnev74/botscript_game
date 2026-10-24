// Package ids is the crypto/rand app.IDGen. Nothing is remembered between
// runs, so uniqueness across restarts rests on randomness (32 bits for bot
// ids; the arena redraws on the rare clash with a known id) and tokens carry
// 256 bits.
package ids

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
)

// Gen is an app.IDGen.
type Gen struct{}

// New returns a generator.
func New() Gen { return Gen{} }

// BotID is "b-" and 8 hex digits.
func (Gen) BotID() string { return "b-" + hex.EncodeToString(random(4)) }

// Token is 32 random bytes, base64url without padding (43 characters).
func (Gen) Token() string { return base64.RawURLEncoding.EncodeToString(random(32)) }

// Seed is a random map seed below 2^48, so the browser reads it back exactly
// as a JSON number.
func (Gen) Seed() uint64 {
	var n uint64
	for _, b := range random(6) {
		n = n<<8 | uint64(b)
	}
	return n
}

func random(n int) []byte {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand: " + err.Error()) // the OS entropy source failed; nothing sane to do
	}
	return b
}
