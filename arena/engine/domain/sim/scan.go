package sim

import (
	"github.com/lezhnev/arena/engine/domain/brain"
	"github.com/lezhnev/arena/engine/domain/event"
	"github.com/lezhnev/arena/engine/domain/geo"
)

// coneCell is a scanned cell and its lane distance from the scan line.
type coneCell struct {
	Pos geo.Pos
	Off int // |perpendicular offset| from the scanner's line
}

// cone lists, deterministically (nearest column first, left to right), the
// floor cells a scan in direction d covers: every cell whose perpendicular
// offset is <= its distance along d times slope/100 (slope 100 is a 90 degree
// wedge), up to the arena
// edge. Wall cells are never listed, and walls block the view: a cell
// behind a wall (no line of sight between cell centres, geo.Grid.Sees) is not
// in the cone.
func cone(g geo.Grid, from geo.Pos, d geo.Dir, slope int) []coneCell {
	var out []coneCell
	side := d.Right()
	for k := 1; g.InBounds(stepN(from, d, k)); k++ {
		axis := stepN(from, d, k)
		reach := k * slope / 100
		for o := -reach; o <= reach; o++ {
			p := stepN(axis, side, o)
			if g.InBounds(p) && !g.IsWall(p) && g.Sees(from, p) {
				out = append(out, coneCell{Pos: p, Off: abs(o)})
			}
		}
	}
	return out
}

// stepN moves n cells in direction d; n may be negative.
func stepN(p geo.Pos, d geo.Dir, n int) geo.Pos {
	dx, dy := d.Delta()
	return geo.Pos{X: p.X + dx*n, Y: p.Y + dy*n}
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// level is how lined up a cell is: 3 on my line, 2 one lane off, 1 elsewhere
// in the cone.
func level(off int) int {
	switch off {
	case 0:
		return 3
	case 1:
		return 2
	}
	return 1
}

// scanPhase runs after moves and after bullet flight, so every scan sees the
// post-move positions and only bullets still in the air: one that struck a wall
// or a bot this beat is already gone (its contact waits in c.hits for hitPhase).
func scanPhase(c *beatCtx) {
	for i, b := range c.w.Bots {
		a := c.acts[b.ID]
		switch {
		case a.Kind == brain.Scan && a.Dir.Valid():
			c.scan(i, a.Dir)
		}
	}
}

func (c *beatCtx) scan(i int, d geo.Dir) {
	self := c.w.Bots[i]
	cells := cone(c.w.Grid, self.Pos, d, c.rules.ConeSlope)
	seen := c.sightings(self.ID, d, lanes(cells))
	r := readings(d, seen)
	c.w.Bots[i].Read = r
	c.emit(event.Scanned{
		Bot: self.ID, Dir: d, Enemy: r.Enemy, Coin: r.Coin, Bullet: r.Bullet,
		Cells: eventCells(cells), Seen: seen,
	})
}

func lanes(cells []coneCell) map[geo.Pos]int {
	m := make(map[geo.Pos]int, len(cells))
	for _, c := range cells {
		m[c.Pos] = c.Off
	}
	return m
}

func eventCells(cells []coneCell) []event.Cell {
	out := make([]event.Cell, len(cells))
	for i, c := range cells {
		out[i] = event.Cell{X: c.Pos.X, Y: c.Pos.Y}
	}
	return out
}

// sightings lists the objects in the cone: other bots (ID order), coins, then
// bullets flying toward the scanner (opposite to the scan direction).
func (c *beatCtx) sightings(selfID string, d geo.Dir, off map[geo.Pos]int) []event.Sighting {
	var out []event.Sighting
	add := func(kind string, p geo.Pos) {
		if o, ok := off[p]; ok {
			out = append(out, event.Sighting{Kind: kind, X: p.X, Y: p.Y, Level: level(o)})
		}
	}
	for _, b := range c.w.Bots {
		if b.ID != selfID {
			add(event.KindEnemy, b.Pos)
		}
	}
	for _, p := range c.w.Coins {
		add(event.KindCoin, p)
	}
	for _, b := range c.w.Bullets {
		if b.Dir == d.Back() {
			add(event.KindBullet, b.Pos)
		}
	}
	return out
}

// readingSet adds field access by kind to Readings.
type readingSet struct{ brain.Readings }

// readings keeps the best level seen per kind.
func readings(d geo.Dir, seen []event.Sighting) brain.Readings {
	r := readingSet{Readings: brain.Readings{Dir: d}}
	for _, s := range seen {
		slot := r.slot(s.Kind)
		*slot = max(*slot, s.Level)
	}
	return r.Readings
}

// slot is the reading field for a sighting kind.
func (r *readingSet) slot(kind string) *int {
	switch kind {
	case event.KindEnemy:
		return &r.Enemy
	case event.KindCoin:
		return &r.Coin
	}
	return &r.Bullet
}
