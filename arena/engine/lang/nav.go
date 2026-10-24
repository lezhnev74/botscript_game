package lang

import "github.com/lezhnev/arena/engine/domain/geo"

// Nav answers walking questions about one match terrain: the distance from a
// cell to a target and the first step toward it. A BFS per target is cached,
// so a bot that asks about the same pin every beat pays for it once. The cache
// is private to its owner (one Nav per brain), never shared between bot
// goroutines; it holds at most one slice per floor cell.
type Nav struct {
	grid  geo.Grid
	cache map[geo.Pos][]int
}

// NewNav returns a Nav for the terrain g.
func NewNav(g geo.Grid) *Nav { return &Nav{grid: g, cache: map[geo.Pos][]int{}} }

// dist is the distance map toward to, nil when to is a wall or off the map.
func (n *Nav) dist(to geo.Pos) []int {
	if n.grid.Blocked(to) {
		return nil
	}
	d, ok := n.cache[to]
	if !ok {
		d = n.grid.Distances(to)
		n.cache[to] = d
	}
	return d
}

// Steps is the walking distance from a cell to a target, -1 if unreachable.
func (n *Nav) Steps(from, to geo.Pos) int {
	d := n.dist(to)
	if d == nil {
		return -1
	}
	return n.grid.DistanceAt(d, from)
}

// Path is the first step (a direction 0-3) of a shortest walk from a cell to
// a target, ties N, E, S, W. -1 if the target is unreachable or the cell
// itself.
func (n *Nav) Path(from, to geo.Pos) int {
	here := n.Steps(from, to)
	if here <= 0 {
		return -1
	}
	d := n.dist(to)
	for dir := geo.N; dir <= geo.W; dir++ {
		dx, dy := dir.Delta()
		if n.grid.DistanceAt(d, geo.Pos{X: from.X + dx, Y: from.Y + dy}) == here-1 {
			return int(dir)
		}
	}
	return -1
}
