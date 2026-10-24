package geo

// Distances is the walking distance from every cell to the target over floor
// cells (walls only; bots and coins are ignored), 4-neighbour steps, in
// row-major order: index y*Width()+x. Cells that cannot reach the target are
// -1, and so is every cell when the target is a wall or off the grid. Read it
// with DistanceAt. Pure and deterministic.
func (g Grid) Distances(to Pos) []int {
	d := make([]int, g.w*g.h)
	for i := range d {
		d[i] = -1
	}
	if g.Blocked(to) {
		return d
	}
	d[to.Y*g.w+to.X] = 0
	queue := []Pos{to}
	for len(queue) > 0 {
		p := queue[0]
		queue = queue[1:]
		for dir := N; dir <= W; dir++ {
			dx, dy := dir.Delta()
			q := Pos{p.X + dx, p.Y + dy}
			if g.Blocked(q) || d[q.Y*g.w+q.X] >= 0 {
				continue
			}
			d[q.Y*g.w+q.X] = d[p.Y*g.w+p.X] + 1
			queue = append(queue, q)
		}
	}
	return d
}

// DistanceAt reads p from a Distances result of this grid: -1 for a cell
// outside the grid.
func (g Grid) DistanceAt(d []int, p Pos) int {
	if !g.InBounds(p) {
		return -1
	}
	return d[p.Y*g.w+p.X]
}
