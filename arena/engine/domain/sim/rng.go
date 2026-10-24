package sim

// rng is a PCG-XSH-RR 32-bit generator held as a plain value: drawing returns
// the advanced copy, so a World that carries one stays a pure value.
type rng struct{ state uint64 }

const (
	pcgMul = 6364136223846793005
	pcgInc = 1442695040888963407 // odd stream constant
)

func (r rng) step() rng { return rng{r.state*pcgMul + pcgInc} }

// newRNG seeds the generator the way the PCG reference does.
func newRNG(seed uint64) rng {
	return rng{}.step().advanceBy(seed).step()
}

func (r rng) advanceBy(v uint64) rng { return rng{r.state + v} }

// next returns 32 random bits and the advanced generator.
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
