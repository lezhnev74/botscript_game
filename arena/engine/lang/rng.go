package lang

import "hash/fnv"

// rng is a PCG-XSH-RR 32-bit generator held as a plain value (the same
// algorithm as domain/sim, kept separate so each bot owns its stream).
type rng struct{ state uint64 }

const (
	pcgMul = 6364136223846793005
	pcgInc = 1442695040888963407
)

func (r rng) step() rng { return rng{r.state*pcgMul + pcgInc} }

func newRNG(seed uint64) rng { return rng{}.step().add(seed).step() }

func (r rng) add(v uint64) rng { return rng{r.state + v} }

func (r rng) next() (uint32, rng) {
	x := uint32(((r.state >> 18) ^ r.state) >> 27)
	rot := uint32(r.state >> 59)
	return x>>rot | x<<((-rot)&31), r.step()
}

// intn returns a value in [0, n) for n > 0 and the advanced generator.
func (r rng) intn(n int) (int, rng) {
	v, r := r.next()
	return int(uint64(v) * uint64(n) >> 32), r
}

func splitmix(x uint64) uint64 {
	z := x + 0x9e3779b97f4a7c15
	z = (z ^ z>>30) * 0xbf58476d1ce4e5b9
	z = (z ^ z>>27) * 0x94d049bb133111eb
	return z ^ z>>31
}

// SeedFor derives a bot's dice seed from the match seed and its ID:
// splitmix64(matchSeed XOR splitmix64(FNV-1a-64(botID))). Every bot gets its
// own stream, independent of how many other bots exist or in what order they
// act. Changing this changes every recorded replay.
func SeedFor(matchSeed uint64, botID string) uint64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(botID))
	return splitmix(matchSeed ^ splitmix(h.Sum64()))
}
