// Package geo holds the pure geometry of the arena: directions, positions,
// the wall grid and ASCII map parsing/rendering.
package geo

import "strconv"

// Dir is one of the four compass directions. Values are stable: N=0 E=1 S=2 W=3.
type Dir int

// The four directions, clockwise from north.
const (
	N Dir = iota
	E
	S
	W
)

var dirNames = [...]string{"N", "E", "S", "W"}

var dirDeltas = [...][2]int{{0, -1}, {1, 0}, {0, 1}, {-1, 0}}

// Valid reports whether d is one of N, E, S, W.
func (d Dir) Valid() bool { return d >= N && d <= W }

// Left is the direction 90 degrees counter-clockwise.
func (d Dir) Left() Dir { return (d + 3) % 4 }

// Right is the direction 90 degrees clockwise.
func (d Dir) Right() Dir { return (d + 1) % 4 }

// Back is the opposite direction.
func (d Dir) Back() Dir { return (d + 2) % 4 }

// Delta is the unit step of d. Y grows southwards (row 0 is the top row).
// d must be a valid Dir; Delta panics otherwise.
func (d Dir) Delta() (dx, dy int) {
	v := dirDeltas[d]
	return v[0], v[1]
}

func (d Dir) String() string {
	if !d.Valid() {
		return "Dir(" + strconv.Itoa(int(d)) + ")"
	}
	return dirNames[d]
}

// ParseDir parses "N", "E", "S" or "W".
func ParseDir(s string) (Dir, bool) {
	for i, n := range dirNames {
		if s == n {
			return Dir(i), true
		}
	}
	return 0, false
}

// Pos is a cell coordinate; X is the column, Y the row (0,0 = top-left).
type Pos struct{ X, Y int }

// Step is the neighbouring cell in direction d. d must be a valid Dir;
// Step panics otherwise.
func (p Pos) Step(d Dir) Pos {
	dx, dy := d.Delta()
	return Pos{p.X + dx, p.Y + dy}
}
