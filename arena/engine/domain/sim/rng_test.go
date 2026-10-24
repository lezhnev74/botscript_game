package sim

import (
	"testing"

	"github.com/lezhnev/arena/engine/domain/geo"
)

func drawN(r rng, n int) ([]int, rng) {
	out := make([]int, n)
	for i := range out {
		out[i], r = r.intn(10)
	}
	return out, r
}

func TestRNGSameSeedSameSequence(t *testing.T) {
	a, _ := drawN(newRNG(42), 20)
	b, _ := drawN(newRNG(42), 20)
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("draw %d differs: %d vs %d", i, a[i], b[i])
		}
	}
}

func TestRNGDifferentSeedsDiffer(t *testing.T) {
	a, _ := drawN(newRNG(1), 20)
	b, _ := drawN(newRNG(2), 20)
	same := true
	for i := range a {
		same = same && a[i] == b[i]
	}
	if same {
		t.Error("seeds 1 and 2 gave the same sequence")
	}
}

func TestRNGIsAValue(t *testing.T) {
	r := newRNG(7)
	first, _ := r.intn(1000)
	again, _ := r.intn(1000)
	if first != again {
		t.Error("drawing from a copy must not change the original")
	}
}

func TestRNGIntnStaysInRange(t *testing.T) {
	r := newRNG(3)
	seen := map[int]bool{}
	for range 200 {
		var v int
		v, r = r.intn(4)
		if v < 0 || v > 3 {
			t.Fatalf("out of range: %d", v)
		}
		seen[v] = true
	}
	if len(seen) != 4 {
		t.Errorf("only saw %v", seen)
	}
}

// Reference values come from an independent PCG-XSH-RR 64/32 implementation
// (multiplier 6364136223846793005, increment 1442695040888963407, seeded with
// step, add seed, step).
func TestRNGMatchesReferenceOutputs(t *testing.T) {
	want := []uint32{3270867926, 1795671209, 1924641435, 1143034755, 4121910957}
	r := newRNG(42)
	for i, w := range want {
		var got uint32
		got, r = r.next()
		if got != w {
			t.Fatalf("output %d = %d, want %d", i, got, w)
		}
	}
}

func TestRNGIntnMatchesReference(t *testing.T) {
	want := []int{7, 4, 4, 2, 9}
	r := newRNG(42)
	for i, w := range want {
		var got int
		got, r = r.intn(10)
		if got != w {
			t.Fatalf("intn %d = %d, want %d", i, got, w)
		}
	}
}

func TestPickSpotAdvancesTheWorldRNG(t *testing.T) {
	c := newBeatCtx(NewWorld(mustMap(t, "#####\n#A..#\n#####\n"), DefaultRules(), 5), nil, DefaultRules())
	before := c.w.RNG
	c.pickSpot([]geo.Pos{pos(1, 1), pos(2, 1)})
	if c.w.RNG == before {
		t.Error("a draw must advance World.RNG")
	}
}
