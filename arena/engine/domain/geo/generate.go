package geo

import (
	"fmt"
	"math/rand/v2"
	"strings"
)

// Size limits of a generated room, in cells.
const (
	MinGenWidth, MaxGenWidth   = 9, 41
	MinGenHeight, MaxGenHeight = 7, 31
)

// Share of interior cells that become walls, in percent: the default and the
// most a generated room may have.
const (
	DefaultWallPercent = 3
	MaxWallPercent     = 15
)

// maxDraws caps the retries of Generate. With walls at most 15% of the
// interior in segments of at most 3 cells and a free ring, a connected draw
// comes within a few tries, so the cap is a safety net, not a normal exit.
var maxDraws = 1000

// maxSegment is the longest interior wall segment, in cells.
const maxSegment = 3

// Generate draws a w x h room as ASCII (the format of ParseMap, no bots): a
// wall border and about walls% interior walls (0..MaxWallPercent), laid as short straight segments and
// single pillars. A one-cell floor ring is always left along the border, and
// every floor cell can reach every other. The same seed gives the same room;
// a draw that cuts the floor in two is dropped and the next one is used.
func Generate(w, h, walls int, seed uint64) (string, error) {
	if err := checkGen(w, h, walls); err != nil {
		return "", err
	}
	rng := rand.New(rand.NewPCG(seed, 0x9e3779b97f4a7c15))
	for range maxDraws {
		rows := draw(w, h, walls, rng)
		if m, err := ParseMap(rows); err == nil && Connected(m.Grid) {
			return rows, nil
		}
	}
	return "", fmt.Errorf("no connected %dx%d room in %d draws", w, h, maxDraws)
}

func checkGen(w, h, walls int) error {
	if walls < 0 || walls > MaxWallPercent {
		return fmt.Errorf("walls %d%%: must be 0..%d", walls, MaxWallPercent)
	}
	if w >= MinGenWidth && w <= MaxGenWidth && h >= MinGenHeight && h <= MaxGenHeight {
		return nil
	}
	return fmt.Errorf("map size %dx%d: width must be %d..%d, height %d..%d",
		w, h, MinGenWidth, MaxGenWidth, MinGenHeight, MaxGenHeight)
}

// draw lays one random room.
func draw(w, h, walls int, rng *rand.Rand) string {
	cells := make([][]byte, h)
	for y := range cells {
		cells[y] = []byte(strings.Repeat(".", w))
		if y == 0 || y == h-1 {
			cells[y] = []byte(strings.Repeat("#", w))
		}
		cells[y][0], cells[y][w-1] = '#', '#'
	}
	target := (w - 2) * (h - 2) * walls / 100
	for placed := 0; placed < target; {
		placed += laySegment(cells, rng, target-placed)
	}
	rows := make([]string, h)
	for y := range cells {
		rows[y] = string(cells[y])
	}
	return strings.Join(rows, "\n") + "\n"
}

// laySegment walls up to limit new cells of one random segment inside the
// ring (cells 2..w-3, 2..h-3) and returns how many it walled.
func laySegment(cells [][]byte, rng *rand.Rand, limit int) int {
	h, w := len(cells), len(cells[0])
	x, y := 2+rng.IntN(w-4), 2+rng.IntN(h-4)
	dx, dy := 1, 0
	if rng.IntN(2) == 0 {
		dx, dy = 0, 1
	}
	length, n := 1+rng.IntN(maxSegment), 0
	for i := 0; i < length && n < limit; i++ {
		cx, cy := x+i*dx, y+i*dy
		if cx > w-3 || cy > h-3 {
			break
		}
		if cells[cy][cx] == '.' {
			cells[cy][cx] = '#'
			n++
		}
	}
	return n
}

// Connected reports whether every floor cell of g can reach every other. A
// grid without floor is connected.
func Connected(g Grid) bool {
	start, ok := firstFloor(g)
	if !ok {
		return true
	}
	for i, d := range g.Distances(start) {
		if d < 0 && !g.walls[i] {
			return false
		}
	}
	return true
}

func firstFloor(g Grid) (Pos, bool) {
	for i, wall := range g.walls {
		if !wall {
			return Pos{X: i % g.w, Y: i / g.w}, true
		}
	}
	return Pos{}, false
}
