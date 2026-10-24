package geo

import "testing"

func TestGridBounds(t *testing.T) {
	m := mustParse(t, "#..\n.#.")
	g := m.Grid
	tests := []struct {
		p       Pos
		in      bool
		wall    bool
		blocked bool
	}{
		{Pos{0, 0}, true, true, true},
		{Pos{1, 0}, true, false, false},
		{Pos{1, 1}, true, true, true},
		{Pos{2, 1}, true, false, false},
		{Pos{-1, 0}, false, false, true},
		{Pos{0, -1}, false, false, true},
		{Pos{3, 0}, false, false, true},
		{Pos{0, 2}, false, false, true},
	}
	for _, tt := range tests {
		if got := g.InBounds(tt.p); got != tt.in {
			t.Errorf("InBounds(%v) = %v", tt.p, got)
		}
		if got := g.IsWall(tt.p); got != tt.wall {
			t.Errorf("IsWall(%v) = %v", tt.p, got)
		}
		if got := g.Blocked(tt.p); got != tt.blocked {
			t.Errorf("Blocked(%v) = %v", tt.p, got)
		}
	}
}

func TestGridSees(t *testing.T) {
	g := mustParse(t, ".....\n.#...\n..#..\n.....").Grid
	tests := []struct {
		name string
		a, b Pos
		want bool
	}{
		{"same cell", Pos{0, 0}, Pos{0, 0}, true},
		{"neighbour", Pos{0, 0}, Pos{1, 0}, true},
		{"open row", Pos{0, 0}, Pos{4, 0}, true},
		{"wall in the row", Pos{0, 2}, Pos{4, 2}, false},
		{"wall in the column", Pos{1, 0}, Pos{1, 3}, false},
		{"line cuts a wall interior", Pos{0, 1}, Pos{4, 2}, false},
		{"line passes above the wall", Pos{0, 0}, Pos{4, 1}, true},
		{"diagonal grazing two wall corners", Pos{0, 3}, Pos{3, 0}, true},
		{"diagonal neighbours grazing a wall corner", Pos{0, 1}, Pos{1, 0}, true},
		{"diagonal through a wall", Pos{0, 0}, Pos{3, 3}, false},
		{"to a wall cell itself", Pos{0, 1}, Pos{1, 1}, true},
	}
	for _, tt := range tests {
		if got := g.Sees(tt.a, tt.b); got != tt.want {
			t.Errorf("%s: Sees(%v, %v) = %v, want %v", tt.name, tt.a, tt.b, got, tt.want)
		}
		if got := g.Sees(tt.b, tt.a); got != tt.want {
			t.Errorf("%s: Sees(%v, %v) = %v, want %v (not symmetric)", tt.name, tt.b, tt.a, got, tt.want)
		}
	}
}
