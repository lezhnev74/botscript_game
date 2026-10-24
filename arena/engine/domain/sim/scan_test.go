package sim

import (
	"fmt"
	"strings"
	"testing"

	"github.com/lezhnev/arena/engine/domain/brain"
	"github.com/lezhnev/arena/engine/domain/event"
	"github.com/lezhnev/arena/engine/domain/geo"
)

func sc(d geo.Dir) brain.Action { return brain.Action{Kind: brain.Scan, Dir: d} }

// scanOf resolves one beat in which only A acts and returns A's Scanned event.
func scanOf(t *testing.T, w World, act brain.Action) (event.Scanned, World) {
	t.Helper()
	return scanOfRules(t, w, act, DefaultRules())
}

// slowRules makes bullets crawl one cell a beat, so a test can place one and
// read where it stands after the beat's flight.
func slowRules() Rules {
	r := DefaultRules()
	r.BulletSpeed = 1
	return r
}

func scanOfRules(t *testing.T, w World, act brain.Action, r Rules) (event.Scanned, World) {
	t.Helper()
	next, evs := Resolve(w, map[string]brain.Action{"A": act}, r)
	for _, e := range evs {
		if s, ok := e.(event.Scanned); ok {
			return s, next
		}
	}
	t.Fatalf("no Scanned event in %v", texts(evs))
	return event.Scanned{}, next
}

const open9 = "#########\n#.......#\n#.......#\n#..A....#\n#.......#\n#.......#\n#########\n"

const open7 = "#######\n#.....#\n#.....#\n#..A..#\n#.....#\n#.....#\n#######\n"

func TestConeShape(t *testing.T) {
	g := mustMap(t, open7).Grid
	cells := cone(g, geo.Pos{X: 3, Y: 3}, geo.E, fullWedge)
	// Along E from x=3: column x=4 holds 3 cells, column x=5 holds 5 (rows 1..5).
	// Column x=6 is the wall, and wall cells are never scanned.
	if len(cells) != 3+5 {
		t.Fatalf("got %d cells: %v", len(cells), cells)
	}
	for _, c := range cells {
		if c.Pos.X <= 3 || c.Pos.X > 5 {
			t.Errorf("cell %v outside the wedge", c.Pos)
		}
		if want := abs(c.Pos.Y - 3); c.Off != want {
			t.Errorf("cell %v offset %d, want %d", c.Pos, c.Off, want)
		}
	}
}

func TestConeIsWedgeInEveryDirection(t *testing.T) {
	g := mustMap(t, "#########\n#.......#\n#.......#\n#.......#\n#...A...#\n#.......#\n#.......#\n#.......#\n#########\n").Grid
	from := geo.Pos{X: 4, Y: 4}
	for _, d := range []geo.Dir{geo.N, geo.E, geo.S, geo.W} {
		for _, c := range cone(g, from, d, fullWedge) {
			dx, dy := d.Delta()
			along := (c.Pos.X-from.X)*dx + (c.Pos.Y-from.Y)*dy
			if along < 1 || c.Off > along {
				t.Errorf("%v: cell %v along %d off %d breaks the wedge", d, c.Pos, along, c.Off)
			}
		}
	}
}

func TestConeEmptyAtWall(t *testing.T) {
	g := mustMap(t, "####\n#A.#\n####\n").Grid
	if cells := cone(g, geo.Pos{X: 1, Y: 1}, geo.W, fullWedge); len(cells) != 0 {
		t.Errorf("cone into a wall: %v", cells)
	}
}

func TestConeStopsAtWallsAndSkipsWallCells(t *testing.T) {
	g := mustMap(t, "#####\n#A#.#\n#####\n").Grid
	if cells := cone(g, geo.Pos{X: 1, Y: 1}, geo.E, fullWedge); len(cells) != 0 {
		t.Errorf("cells %v, want none (the wall hides (3,1))", cells)
	}
}

// A pillar at (3,2) shadows a widening wedge behind it; lines that only graze
// its corners ((4,1), (4,3)) still see.
func TestConeLeavesOutThePillarShadow(t *testing.T) {
	g := mustMap(t, "#######\n#.....#\n#A.#..#\n#.....#\n#######\n").Grid
	in := map[geo.Pos]bool{}
	for _, c := range cone(g, geo.Pos{X: 1, Y: 2}, geo.E, fullWedge) {
		in[c.Pos] = true
	}
	for _, p := range []geo.Pos{{X: 4, Y: 2}, {X: 5, Y: 2}, {X: 5, Y: 1}, {X: 5, Y: 3}} {
		if in[p] {
			t.Errorf("%v is behind the pillar but in the cone", p)
		}
	}
	for _, p := range []geo.Pos{{X: 2, Y: 2}, {X: 2, Y: 1}, {X: 4, Y: 1}, {X: 4, Y: 3}} {
		if !in[p] {
			t.Errorf("%v is in plain view but missing from the cone", p)
		}
	}
}

// Enemy and coin share the offset -> level rule: 0 -> 3, 1 -> 2, >=2 -> 1,
// outside the cone -> 0. A scans E from (3,3); the target sits at (5, 3+off).
func TestEnemyAndCoinLevels(t *testing.T) {
	cases := []struct {
		name   string
		tx, ty int
		want   int
	}{
		{"on the line", 5, 3, 3},
		{"adjacent on the line", 4, 3, 3},
		{"one lane off", 5, 4, 2},
		{"one lane off, other side", 5, 2, 2},
		{"two lanes off, inside the wedge", 5, 5, 1},
		{"two lanes off, other side", 5, 1, 1},
		{"behind the scanner", 1, 3, 0},
		{"beside the scanner (along 0)", 3, 5, 0},
		{"outside the wedge", 4, 5, 0},
		{"one lane off but along 0", 3, 4, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) { checkLevels(t, c.tx, c.ty, c.want) })
	}
}

func checkLevels(t *testing.T, tx, ty, want int) {
	t.Helper()
	m := mustMap(t, open7)
	w := NewWorld(m, DefaultRules(), 1)
	w.Bots = append(w.Bots, Bot{ID: "B", Pos: geo.Pos{X: tx, Y: ty}, HP: 3})
	s, next := scanOf(t, w, sc(geo.E))
	if s.Enemy != want || s.Coin != 0 || s.Bullet != 0 {
		t.Errorf("enemy scan: %d %d %d, want enemy %d", s.Enemy, s.Coin, s.Bullet, want)
	}
	if r := next.Bots[0].Read; r != (brain.Readings{Dir: geo.E, Enemy: want}) {
		t.Errorf("readings %+v", r)
	}
	w = NewWorld(m, DefaultRules(), 1)
	w.Coins = []geo.Pos{{X: tx, Y: ty}}
	if s, _ = scanOf(t, w, sc(geo.E)); s.Coin != want || s.Enemy != 0 {
		t.Errorf("coin scan: %d %d, want coin %d", s.Enemy, s.Coin, want)
	}
}

func TestBestAlignedObjectWins(t *testing.T) {
	w := NewWorld(mustMap(t, open7), DefaultRules(), 1)
	w.Bots = append(w.Bots,
		Bot{ID: "B", Pos: geo.Pos{X: 5, Y: 5}, HP: 3}, // level 1
		Bot{ID: "C", Pos: geo.Pos{X: 5, Y: 4}, HP: 3}, // level 2
	)
	s, _ := scanOf(t, w, sc(geo.E))
	if s.Enemy != 2 {
		t.Errorf("enemy %d, want 2", s.Enemy)
	}
	w.Bots = append(w.Bots, Bot{ID: "D", Pos: geo.Pos{X: 4, Y: 3}, HP: 3}) // level 3
	if s, _ = scanOf(t, w, sc(geo.E)); s.Enemy != 3 {
		t.Errorf("enemy %d, want 3", s.Enemy)
	}
}

func TestScanNeverReportsSelf(t *testing.T) {
	s, _ := scanOf(t, NewWorld(mustMap(t, open7), DefaultRules(), 1), sc(geo.E))
	if s.Enemy != 0 || len(s.Seen) != 0 {
		t.Errorf("self reported: %+v", s)
	}
}

func TestBulletLevels(t *testing.T) {
	cases := []struct {
		name string
		pos  geo.Pos
		dir  geo.Dir
		want int
	}{
		{"toward me on my line", geo.Pos{X: 6, Y: 3}, geo.W, 3},
		{"toward me one lane off", geo.Pos{X: 6, Y: 4}, geo.W, 2},
		{"toward me two lanes off", geo.Pos{X: 6, Y: 5}, geo.W, 1},
		{"flying away on my line", geo.Pos{X: 5, Y: 3}, geo.E, 0},
		{"crossing my line", geo.Pos{X: 5, Y: 3}, geo.N, 0},
		{"crossing the other way", geo.Pos{X: 5, Y: 3}, geo.S, 0},
		{"toward me but behind me", geo.Pos{X: 1, Y: 3}, geo.W, 0},
		{"toward the scan side but outside the cone", geo.Pos{X: 4, Y: 5}, geo.W, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			w := NewWorld(mustMap(t, open9), DefaultRules(), 1)
			w.Bullets = []Bullet{{Pos: c.pos, Dir: c.dir}}
			s, next := scanOfRules(t, w, sc(geo.E), slowRules())
			if s.Bullet != c.want || next.Bots[0].Read.Bullet != c.want {
				t.Errorf("bullet %d (read %d), want %d", s.Bullet, next.Bots[0].Read.Bullet, c.want)
			}
		})
	}
}

func TestBulletLevelsInOtherScanDirections(t *testing.T) {
	// A at (3,3) scans N; a bullet at (3,1) flying S comes at A.
	w := NewWorld(mustMap(t, open7), DefaultRules(), 1)
	w.Bullets = []Bullet{{Pos: geo.Pos{X: 3, Y: 1}, Dir: geo.S}}
	if s, _ := scanOfRules(t, w, sc(geo.N), slowRules()); s.Bullet != 3 {
		t.Errorf("north scan bullet %d, want 3", s.Bullet)
	}
	if s, _ := scanOfRules(t, w, sc(geo.E), slowRules()); s.Bullet != 0 {
		t.Errorf("east scan bullet %d, want 0", s.Bullet)
	}
}

func TestScanRunsAfterMoves(t *testing.T) {
	w := NewWorld(mustMap(t, "#######\n#A....#\n#..B..#\n#######\n"), DefaultRules(), 1)
	acts := map[string]brain.Action{"A": sc(geo.E), "B": mv(geo.N)}
	_, evs := Resolve(w, acts, DefaultRules())
	if got := texts(evs); !equal(got, []string{"Moved B N", "Scanned A E 3 0 0"}) {
		t.Errorf("events %v", got)
	}
}

func TestScanHappensFromPostMovePosition(t *testing.T) {
	w := NewWorld(mustMap(t, "#######\n#A....#\n#.....#\n#..B..#\n#######\n"), DefaultRules(), 1)
	// A scans E: B at (3,3) is two lanes off (level 1). B moves N to (3,2): one lane off (level 2).
	acts := map[string]brain.Action{"A": sc(geo.E), "B": mv(geo.N)}
	next, _ := Resolve(w, acts, DefaultRules())
	if next.Bots[0].Read.Enemy != 2 {
		t.Errorf("reading %+v, want enemy 2 on post-move positions", next.Bots[0].Read)
	}
}

func TestScanRecord(t *testing.T) {
	w := NewWorld(mustMap(t, "######\n#A...#\n#..B.#\n#...$#\n######\n"), DefaultRules(), 1)
	w.Bullets = []Bullet{{Pos: geo.Pos{X: 4, Y: 1}, Dir: geo.W}, {Pos: geo.Pos{X: 4, Y: 2}, Dir: geo.E}}
	s, _ := scanOfRules(t, w, sc(geo.E), slowRules()) // the first bullet crawls on to (3,1)
	want := []event.Sighting{
		{Kind: event.KindEnemy, X: 3, Y: 2, Level: 2},
		{Kind: event.KindCoin, X: 4, Y: 3, Level: 1},
		{Kind: event.KindBullet, X: 3, Y: 1, Level: 3},
	}
	if fmt.Sprint(s.Seen) != fmt.Sprint(want) {
		t.Errorf("seen %v, want %v", s.Seen, want)
	}
	// cone from (1,1) E: x=2 -> y 0..2 clipped to 1,2 ; x=3 -> y 1..3 ; x=4 -> y 1..3 (5 clipped by walls)
	if len(s.Cells) != 2+3+3 || s.Cells[0] != (event.Cell{X: 2, Y: 1}) {
		t.Errorf("cells %v", s.Cells)
	}
}

func TestOnlyValidScansCount(t *testing.T) {
	w := NewWorld(mustMap(t, open7), DefaultRules(), 1)
	for _, a := range []brain.Action{{Kind: brain.Wait}, mv(geo.E), {Kind: brain.Scan, Dir: geo.Dir(9)}} {
		_, evs := Resolve(w, map[string]brain.Action{"A": a}, DefaultRules())
		for _, e := range evs {
			if e.Name() == "Scanned" {
				t.Errorf("%v produced a scan record", a)
			}
		}
	}
}

func TestReadingsPersistUntilNextScan(t *testing.T) {
	w := NewWorld(mustMap(t, "#######\n#A.B..#\n#######\n"), DefaultRules(), 1)
	next, _ := Resolve(w, map[string]brain.Action{"A": sc(geo.E)}, DefaultRules())
	next, _ = Resolve(next, map[string]brain.Action{"A": {Kind: brain.Wait}}, DefaultRules())
	if p := next.Perception("A", 3); p.Read != (brain.Readings{Dir: geo.E, Enemy: 3}) {
		t.Errorf("readings after a wait: %+v", p.Read)
	}
	next, _ = Resolve(next, map[string]brain.Action{"A": sc(geo.W)}, DefaultRules())
	if p := next.Perception("A", 4); p.Read != (brain.Readings{Dir: geo.W}) {
		t.Errorf("readings after a west scan: %+v", p.Read)
	}
}

const fullWedge = 100 // cone_slope of the 90 degree wedge

const slopeRoom = "###########\n#.........#\n#.........#\n#.........#\n#.........#\n#A........#\n#.........#\n#.........#\n#.........#\n#.........#\n###########\n"

func TestConeSlopeSetsTheMostOffsetPerCellAlong(t *testing.T) {
	g := mustMap(t, slopeRoom).Grid
	from := geo.Pos{X: 1, Y: 5}
	for _, slope := range []int{0, 50, 100, 200} {
		for _, c := range cone(g, from, geo.E, slope) {
			if along := c.Pos.X - from.X; c.Off > along*slope/100 {
				t.Errorf("slope %d: cell %v along %d off %d is outside", slope, c.Pos, along, c.Off)
			}
		}
	}
}

func TestConeSlopeWidensTheWedge(t *testing.T) {
	g := mustMap(t, slopeRoom).Grid
	from := geo.Pos{X: 1, Y: 5}
	count := func(slope int) int { return len(cone(g, from, geo.E, slope)) }
	if !(count(0) < count(50) && count(50) < count(100) && count(100) < count(200)) {
		t.Errorf("a wider slope must cover more cells: %d %d %d %d", count(0), count(50), count(100), count(200))
	}
	if count(0) != 8 {
		t.Errorf("slope 0 is just the line: %d cells", count(0))
	}
}

func TestRulesSetConeSlope(t *testing.T) {
	r := DefaultRules()
	if r.ConeSlope != 100 {
		t.Errorf("default cone_slope %d", r.ConeSlope)
	}
	if err := r.Set("cone_slope", 50); err != nil || r.ConeSlope != 50 {
		t.Errorf("%v %+v", err, r)
	}
}

func TestScanUsesTheConeSlopeRule(t *testing.T) {
	// B sits 2 along and 2 off A's line: inside the 90 degree wedge, outside a half-width one.
	w := NewWorld(mustMap(t, "#######\n#A....#\n#.....#\n#..B..#\n#.....#\n#######\n"), DefaultRules(), 1)
	level := func(slope int) int {
		r := DefaultRules()
		r.ConeSlope = slope
		_, evs := Resolve(w, map[string]brain.Action{"A": sc(geo.E)}, r)
		for _, e := range evs {
			if s, ok := e.(event.Scanned); ok {
				return s.Enemy
			}
		}
		return -1
	}
	if level(100) != 1 || level(50) != 0 {
		t.Errorf("levels %d %d, want 1 and 0", level(100), level(50))
	}
}

// The default wedge is exact: no cell is ever farther off the line than along
// it, even 100 cells out, where a slope of 101 already admits the next lane.
func TestDefaultConeNeverAdmitsAnOffsetBeyondItsDistance(t *testing.T) {
	rows := make([]string, 205)
	for y := range rows {
		rows[y] = "#" + strings.Repeat(".", 203) + "#"
		if y == 0 || y == len(rows)-1 {
			rows[y] = strings.Repeat("#", 205)
		}
	}
	g := mustMap(t, strings.Join(rows, "\n")+"\n").Grid
	from := geo.Pos{X: 1, Y: 102}
	widest := func(slope int) int {
		w := 0
		for _, c := range cone(g, from, geo.E, slope) {
			w = max(w, c.Off-(c.Pos.X-from.X))
		}
		return w
	}
	if w := widest(DefaultRules().ConeSlope); w > 0 {
		t.Errorf("default cone has a cell %d lanes beyond the wedge", w)
	}
	if widest(101) <= 0 {
		t.Error("the check cannot see a slope of 101")
	}
}

func TestScanSeesBulletsAfterTheirFlight(t *testing.T) {
	// A at (1,1) scans E. A bullet at (6,1) flying W moves 3 cells to (3,1)
	// and is still in the air: the scan reads it there, level 3.
	w := NewWorld(mustMap(t, "#########\n#A......#\n#########\n"), DefaultRules(), 1)
	w.Bullets = []Bullet{{Pos: geo.Pos{X: 6, Y: 1}, Dir: geo.W}}
	s, _ := scanOf(t, w, sc(geo.E))
	if s.Bullet != 3 || len(s.Seen) != 1 || s.Seen[0].X != 3 {
		t.Errorf("seen %+v, want one bullet at x=3", s)
	}
}

func TestScanDoesNotSeeBulletsThatStruckThisBeat(t *testing.T) {
	// The bullet flies into the wall at (3,2) this beat: gone, not counted.
	w := NewWorld(mustMap(t, "#########\n#A......#\n#.......#\n#.......#\n#.#.....#\n#########\n"), DefaultRules(), 1)
	w.Bullets = []Bullet{{Pos: geo.Pos{X: 4, Y: 4}, Dir: geo.W}}
	if s, _ := scanOf(t, w, sc(geo.E)); s.Bullet != 0 {
		t.Errorf("bullet %d, want 0 (struck a wall)", s.Bullet)
	}
}

const scanLane = "##########\n#A..B....#\n##########\n"

func TestScanReadsABulletFiredOnTheSameBeat(t *testing.T) {
	// A fires E at B, 5 cells away; B scans W on the same beat. The shot has
	// flown 3 cells to (4,1) by the time B looks: two cells off, on its line.
	w := NewWorld(mustMap(t, "##########\n#A....B..#\n##########\n"), DefaultRules(), 1)
	next, _ := Resolve(w, map[string]brain.Action{"A": fr(geo.E), "B": sc(geo.W)}, DefaultRules())
	b := bot(next, "B")
	if b.Read.Bullet != 3 {
		t.Errorf("B reads bullet %d, want 3", b.Read.Bullet)
	}
}

func TestSidestepAfterTheShotAvoidsTheHit(t *testing.T) {
	w := NewWorld(mustMap(t, "##########\n#A....B..#\n#........#\n##########\n"), DefaultRules(), 1)
	w, _ = Resolve(w, map[string]brain.Action{"A": fr(geo.E), "B": sc(geo.W)}, DefaultRules())
	next, evs := Resolve(w, map[string]brain.Action{"B": {Kind: brain.Move, Dir: geo.S}}, DefaultRules())
	for _, e := range evs {
		if _, ok := e.(event.Hit); ok {
			t.Fatalf("unexpected hit: %v", texts(evs))
		}
	}
	if bot(next, "B").HP != DefaultRules().HP {
		t.Errorf("B hp %d, want full", bot(next, "B").HP)
	}
}

func TestScanDoesNotSeeABulletThatHitAnotherBotThisBeat(t *testing.T) {
	// B stands between the scanner A and the bullet at (7,1): the bullet ends in B.
	w := NewWorld(mustMap(t, scanLane), DefaultRules(), 1)
	w.Bullets = []Bullet{{Pos: geo.Pos{X: 7, Y: 1}, Dir: geo.W, Owner: "X"}}
	s, next := scanOf(t, w, sc(geo.E))
	if s.Bullet != 0 || bot(next, "A").Read.Bullet != 0 {
		t.Errorf("bullet %d, want 0 (spent on B)", s.Bullet)
	}
	if bot(next, "B").HP != DefaultRules().HP-1 {
		t.Errorf("B hp %d, want one hit", bot(next, "B").HP)
	}
}

func TestScannerStillTakesTheHitOnItsScanBeat(t *testing.T) {
	w := NewWorld(mustMap(t, "#########\n#A......#\n#########\n"), DefaultRules(), 1)
	w.Bullets = []Bullet{{Pos: geo.Pos{X: 3, Y: 1}, Dir: geo.W, Owner: "X"}}
	next, evs := Resolve(w, map[string]brain.Action{"A": sc(geo.E)}, DefaultRules())
	a := bot(next, "A")
	if !a.Last.Hit || a.Last.HitDir != geo.E || a.HP != DefaultRules().HP-1 {
		t.Errorf("last %+v hp %d, want hit from E", a.Last, a.HP)
	}
	var hit bool
	for _, e := range evs {
		_, ok := e.(event.Hit)
		hit = hit || ok
	}
	if !hit {
		t.Errorf("no Hit event in %v", texts(evs))
	}
}
