package app

import (
	"crypto/sha256"
	"fmt"
	"regexp"
	"slices"
)

// Look is the cosmetic face of a bot: a color and four named parts. It is not
// part of the simulation, the Record or any hash (like names).
type Look struct {
	Color string `json:"color"` // #rrggbb
	Body  string `json:"body"`
	Eyes  string `json:"eyes"`
	Mouth string `json:"mouth"`
	Hat   string `json:"hat"`
}

// The allowed values of each part of a Look.
var (
	Bodies = []string{"circle", "square", "rounded", "triangle", "hexagon", "diamond", "ghost"}
	Eyes   = []string{"dots", "wide", "sleepy", "angry", "x", "hearts", "visor", "cyclops"}
	Mouths = []string{"smile", "grin", "flat", "frown", "o", "teeth", "tongue", "zigzag"}
	Hats   = []string{"none", "antenna", "crown", "horns", "headband", "bow"}
)

// Palette holds the colors default looks pick from (the source of truth; the
// UI gets colors through the protocol).
var Palette = []string{
	"#e6194b", "#3cb44b", "#4363d8", "#f58231", "#911eb4",
	"#42d4f4", "#f032e6", "#bfef45", "#fabed4", "#ffe119",
}

var colorRe = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// LookError names the field of a rejected look; it matches ErrBadLook.
type LookError struct{ Field string }

func (e *LookError) Error() string { return fmt.Sprintf("%v: %s", ErrBadLook, e.Field) }

// Is makes errors.Is(err, ErrBadLook) true.
func (e *LookError) Is(target error) bool { return target == ErrBadLook }

// Validate checks every part; the error names the first bad field.
func (l Look) Validate() error {
	if !colorRe.MatchString(l.Color) {
		return &LookError{Field: "color"}
	}
	for _, p := range []struct {
		field, val string
		allowed    []string
	}{{"body", l.Body, Bodies}, {"eyes", l.Eyes, Eyes}, {"mouth", l.Mouth, Mouths}, {"hat", l.Hat, Hats}} {
		if !slices.Contains(p.allowed, p.val) {
			return &LookError{Field: p.field}
		}
	}
	return nil
}

// DefaultLook derives a bot's look from its id alone: a stable hash picks the
// color and each part, so the same id always looks the same and no randomness
// is involved.
func DefaultLook(id string) Look {
	h := sha256.Sum256([]byte(id))
	return Look{
		Color: Palette[int(h[0])%len(Palette)],
		Body:  Bodies[int(h[1])%len(Bodies)],
		Eyes:  Eyes[int(h[2])%len(Eyes)],
		Mouth: Mouths[int(h[3])%len(Mouths)],
		Hat:   Hats[int(h[4])%len(Hats)],
	}
}
