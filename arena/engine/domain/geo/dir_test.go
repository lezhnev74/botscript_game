package geo

import "testing"

func TestDirTurning(t *testing.T) {
	tests := []struct {
		d                 Dir
		left, right, back Dir
	}{
		{N, W, E, S},
		{E, N, S, W},
		{S, E, W, N},
		{W, S, N, E},
	}
	for _, tt := range tests {
		t.Run(tt.d.String(), func(t *testing.T) {
			if got := tt.d.Left(); got != tt.left {
				t.Errorf("Left = %v, want %v", got, tt.left)
			}
			if got := tt.d.Right(); got != tt.right {
				t.Errorf("Right = %v, want %v", got, tt.right)
			}
			if got := tt.d.Back(); got != tt.back {
				t.Errorf("Back = %v, want %v", got, tt.back)
			}
		})
	}
}

func TestDirValues(t *testing.T) {
	if N != 0 || E != 1 || S != 2 || W != 3 {
		t.Fatalf("N E S W must be 0..3, got %d %d %d %d", N, E, S, W)
	}
}

func TestDirDelta(t *testing.T) {
	tests := []struct {
		d      Dir
		dx, dy int
	}{
		{N, 0, -1}, {E, 1, 0}, {S, 0, 1}, {W, -1, 0},
	}
	for _, tt := range tests {
		dx, dy := tt.d.Delta()
		if dx != tt.dx || dy != tt.dy {
			t.Errorf("%v.Delta() = %d,%d want %d,%d", tt.d, dx, dy, tt.dx, tt.dy)
		}
	}
}

func TestDirString(t *testing.T) {
	tests := map[Dir]string{N: "N", E: "E", S: "S", W: "W", Dir(7): "Dir(7)"}
	for d, want := range tests {
		if got := d.String(); got != want {
			t.Errorf("String(%d) = %q want %q", int(d), got, want)
		}
	}
}

func TestParseDir(t *testing.T) {
	tests := []struct {
		in   string
		want Dir
		ok   bool
	}{
		{"N", N, true}, {"E", E, true}, {"S", S, true}, {"W", W, true},
		{"", 0, false}, {"n", 0, false}, {"NE", 0, false}, {"X", 0, false},
	}
	for _, tt := range tests {
		got, ok := ParseDir(tt.in)
		if got != tt.want || ok != tt.ok {
			t.Errorf("ParseDir(%q) = %v,%v want %v,%v", tt.in, got, ok, tt.want, tt.ok)
		}
	}
}

func TestDirValid(t *testing.T) {
	if !W.Valid() || Dir(4).Valid() || Dir(-1).Valid() {
		t.Error("Valid must hold exactly for 0..3")
	}
}

func TestPosStep(t *testing.T) {
	p := Pos{X: 2, Y: 3}
	tests := []struct {
		d    Dir
		want Pos
	}{
		{N, Pos{2, 2}}, {E, Pos{3, 3}}, {S, Pos{2, 4}}, {W, Pos{1, 3}},
	}
	for _, tt := range tests {
		if got := p.Step(tt.d); got != tt.want {
			t.Errorf("Step(%v) = %v want %v", tt.d, got, tt.want)
		}
	}
}
