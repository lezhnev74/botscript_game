package geo

import (
	"strings"
	"testing"
)

func countWalls(g Grid) (walls, interior int) {
	for y := 1; y < g.Height()-1; y++ {
		for x := 1; x < g.Width()-1; x++ {
			interior++
			if g.IsWall(Pos{x, y}) {
				walls++
			}
		}
	}
	return walls, interior
}

func TestGenerateShape(t *testing.T) {
	for seed := uint64(0); seed < 60; seed++ {
		text, err := Generate(19, 13, DefaultWallPercent, seed)
		if err != nil {
			t.Fatal(err)
		}
		m := mustParse(t, text)
		if m.Grid.Width() != 19 || m.Grid.Height() != 13 || len(m.Bots) != 0 || len(m.Coins) != 0 {
			t.Fatalf("seed %d: bad map\n%s", seed, text)
		}
		checkBorder(t, m.Grid, seed)
		checkDensity(t, m.Grid, seed, DefaultWallPercent)
	}
}

func checkDensity(t *testing.T, g Grid, seed uint64, percent int) {
	t.Helper()
	walls, interior := countWalls(g)
	if walls != interior*percent/100 {
		t.Errorf("seed %d: %d walls of %d interior cells", seed, walls, interior)
	}
}

// checkBorder wants a wall on every edge cell and a floor ring just inside.
func checkBorder(t *testing.T, g Grid, seed uint64) {
	t.Helper()
	for y := range g.Height() {
		for x := range g.Width() {
			p := Pos{x, y}
			if !borderOK(g, p) {
				t.Fatalf("seed %d: border rule broken at %v", seed, p)
			}
		}
	}
}

func borderOK(g Grid, p Pos) bool {
	switch {
	case isEdge(g, p):
		return g.IsWall(p)
	case isRing(g, p):
		return !g.IsWall(p)
	}
	return true
}

func isEdge(g Grid, p Pos) bool {
	return p.X == 0 || p.Y == 0 || p.X == g.Width()-1 || p.Y == g.Height()-1
}

func isRing(g Grid, p Pos) bool {
	return p.X == 1 || p.Y == 1 || p.X == g.Width()-2 || p.Y == g.Height()-2
}

func TestGenerateWallBudget(t *testing.T) {
	for _, percent := range []int{0, 1, 6, MaxWallPercent} {
		for seed := uint64(0); seed < 20; seed++ {
			text, err := Generate(19, 13, percent, seed)
			if err != nil {
				t.Fatal(err)
			}
			checkDensity(t, mustParse(t, text).Grid, seed, percent)
		}
	}
}

func TestGenerateIsConnectedOverManySeedsAndSizes(t *testing.T) {
	sizes := [][2]int{{9, 7}, {10, 8}, {19, 13}, {41, 31}, {41, 7}, {9, 31}}
	for _, s := range sizes {
		for _, percent := range []int{DefaultWallPercent, MaxWallPercent} {
			for seed := uint64(0); seed < 200; seed++ {
				checkConnected(t, s[0], s[1], percent, seed)
			}
		}
	}
}

func checkConnected(t *testing.T, w, h, percent int, seed uint64) {
	t.Helper()
	text, err := Generate(w, h, percent, seed)
	if err != nil {
		t.Fatal(err)
	}
	if !Connected(mustParse(t, text).Grid) {
		t.Fatalf("%dx%d %d%% seed %d is not connected\n%s", w, h, percent, seed, text)
	}
}

func TestGenerateIsDeterministicAndVaries(t *testing.T) {
	a, _ := Generate(19, 13, DefaultWallPercent, 7)
	b, _ := Generate(19, 13, DefaultWallPercent, 7)
	c, _ := Generate(19, 13, DefaultWallPercent, 8)
	if a != b {
		t.Error("same seed gave different maps")
	}
	if a == c {
		t.Error("different seeds gave the same map")
	}
	if !strings.Contains(a, "#") || strings.Count(a, "\n") != 13 {
		t.Errorf("odd map:\n%s", a)
	}
}

func TestGenerateSizeLimits(t *testing.T) {
	for _, s := range [][2]int{{8, 13}, {42, 13}, {19, 6}, {19, 32}, {0, 0}, {-1, 9}} {
		if _, err := Generate(s[0], s[1], DefaultWallPercent, 1); err == nil {
			t.Errorf("%dx%d accepted", s[0], s[1])
		}
	}
	for _, s := range [][2]int{{9, 7}, {41, 31}} {
		if _, err := Generate(s[0], s[1], DefaultWallPercent, 1); err != nil {
			t.Errorf("%dx%d: %v", s[0], s[1], err)
		}
	}
	for _, walls := range []int{-1, MaxWallPercent + 1} {
		if _, err := Generate(19, 13, walls, 1); err == nil {
			t.Errorf("walls %d accepted", walls)
		}
	}
}

func TestIsConnected(t *testing.T) {
	if !Connected(mustParse(t, "...\n.#.\n...").Grid) {
		t.Error("ring should be connected")
	}
	if Connected(mustParse(t, ".#.").Grid) {
		t.Error("split row should not be connected")
	}
	if !Connected(mustParse(t, "###").Grid) {
		t.Error("no floor is trivially connected")
	}
}

func TestGenerateGivesUpAfterTheDrawCap(t *testing.T) {
	old := maxDraws
	maxDraws = 0 // no draw is allowed: the cap error path
	defer func() { maxDraws = old }()
	if _, err := Generate(11, 9, DefaultWallPercent, 1); err == nil {
		t.Error("no error without any draw")
	}
}
