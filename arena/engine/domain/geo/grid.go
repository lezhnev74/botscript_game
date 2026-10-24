package geo

import "strings"

// Grid is an immutable rectangular field of floor and wall cells.
type Grid struct {
	w, h  int
	walls []bool
}

// Width is the number of columns.
func (g Grid) Width() int { return g.w }

// Height is the number of rows.
func (g Grid) Height() int { return g.h }

// InBounds reports whether p lies inside the grid.
func (g Grid) InBounds(p Pos) bool {
	return p.X >= 0 && p.X < g.w && p.Y >= 0 && p.Y < g.h
}

// IsWall reports whether p is an in-bounds wall cell.
func (g Grid) IsWall(p Pos) bool {
	return g.InBounds(p) && g.walls[p.Y*g.w+p.X]
}

// Blocked reports whether p cannot be entered: a wall or outside the grid.
func (g Grid) Blocked(p Pos) bool {
	return !g.InBounds(p) || g.IsWall(p)
}

// Sees reports whether the straight line between the centres of a and b
// passes through no wall cell. Only a wall's interior blocks: a line that
// grazes a wall's edge or corner sees past it. The endpoints themselves are
// not checked, and Sees(a, b) == Sees(b, a).
func (g Grid) Sees(a, b Pos) bool {
	for y := min(a.Y, b.Y); y <= max(a.Y, b.Y); y++ {
		for x := min(a.X, b.X); x <= max(a.X, b.X); x++ {
			w := Pos{X: x, Y: y}
			if w != a && w != b && g.IsWall(w) && crosses(a, b, w) {
				return false
			}
		}
	}
	return true
}

// crosses reports whether the segment between the centres of a and b enters
// the open square of cell w, for a cell w inside their bounding box (where the
// projections on both axes already overlap). Coordinates are doubled so that
// centres are odd and cell edges even: integer math, no rounding. The segment
// enters the square iff the square's corners lie strictly on both sides of it.
func crosses(a, b, w Pos) bool {
	ax, ay := 2*a.X+1, 2*a.Y+1
	dx, dy := 2*(b.X-a.X), 2*(b.Y-a.Y)
	pos, neg := false, false
	for _, c := range [4][2]int{{0, 0}, {2, 0}, {0, 2}, {2, 2}} {
		side := dx*(2*w.Y+c[1]-ay) - dy*(2*w.X+c[0]-ax)
		pos = pos || side > 0
		neg = neg || side < 0
	}
	return pos && neg
}

// Render draws the grid as ASCII: '#' walls, '.' floor, with marks overlaid by
// position. Every row ends with a newline.
func (g Grid) Render(marks map[Pos]byte) string {
	var sb strings.Builder
	for y := 0; y < g.h; y++ {
		for x := 0; x < g.w; x++ {
			sb.WriteByte(g.cell(Pos{x, y}, marks))
		}
		sb.WriteByte('\n')
	}
	return sb.String()
}

func (g Grid) cell(p Pos, marks map[Pos]byte) byte {
	if c, ok := marks[p]; ok {
		return c
	}
	if g.IsWall(p) {
		return '#'
	}
	return '.'
}
