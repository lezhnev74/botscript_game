package ids_test

import (
	"regexp"
	"testing"

	"github.com/lezhnev/arena/engine/adapters/ids"
)

func TestIDsAndTokensHaveTheirShapeAndAreUnique(t *testing.T) {
	g := ids.New()
	idRe, tokRe := regexp.MustCompile(`^b-[0-9a-f]{8}$`), regexp.MustCompile(`^[A-Za-z0-9_-]{43}$`)
	seenID, seenTok := map[string]bool{}, map[string]bool{}
	for range 2000 {
		id, tok := g.BotID(), g.Token()
		if !idRe.MatchString(id) || !tokRe.MatchString(tok) {
			t.Fatalf("id %q token %q", id, tok)
		}
		if seenID[id] || seenTok[tok] {
			t.Fatalf("repeat: %q %q", id, tok)
		}
		seenID[id], seenTok[tok] = true, true
	}
}

func TestSeedFitsAJavaScriptNumberAndVaries(t *testing.T) {
	g := ids.New()
	seen := map[uint64]bool{}
	for range 200 {
		s := g.Seed()
		if s >= 1<<48 {
			t.Fatalf("seed %d is too big", s)
		}
		seen[s] = true
	}
	if len(seen) < 190 {
		t.Errorf("only %d different seeds in 200 draws", len(seen))
	}
}
