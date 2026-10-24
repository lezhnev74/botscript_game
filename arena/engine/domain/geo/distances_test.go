package geo

import (
	"reflect"
	"testing"
)

func TestDistances(t *testing.T) {
	rows := []struct {
		name string
		text string
		to   Pos
		want []int
	}{
		{"open room", "...\n...", Pos{0, 0}, []int{0, 1, 2, 1, 2, 3}},
		{"corridor", "....", Pos{3, 0}, []int{3, 2, 1, 0}},
		{"around a pillar", "...\n.#.\n...", Pos{0, 0}, []int{0, 1, 2, 1, -1, 3, 2, 3, 4}},
		{"dead end", "#.#\n#.#\n...", Pos{1, 0}, []int{-1, 0, -1, -1, 1, -1, 3, 2, 3}},
		{"unreachable pocket", ".#.\n##.", Pos{0, 0}, []int{0, -1, -1, -1, -1, -1}},
		{"target is a wall", ".#.", Pos{1, 0}, []int{-1, -1, -1}},
		{"target off the map", "...", Pos{3, 0}, []int{-1, -1, -1}},
		{"target negative", "...", Pos{-1, 0}, []int{-1, -1, -1}},
	}
	for _, r := range rows {
		t.Run(r.name, func(t *testing.T) {
			got := mustParse(t, r.text).Grid.Distances(r.to)
			if !reflect.DeepEqual(got, r.want) {
				t.Fatalf("Distances(%v) = %v, want %v", r.to, got, r.want)
			}
		})
	}
}

func TestDistanceAt(t *testing.T) {
	g := mustParse(t, "..\n.#").Grid
	d := g.Distances(Pos{0, 0})
	for _, c := range []struct {
		p    Pos
		want int
	}{{Pos{1, 0}, 1}, {Pos{1, 1}, -1}, {Pos{2, 0}, -1}, {Pos{0, -1}, -1}} {
		if got := g.DistanceAt(d, c.p); got != c.want {
			t.Errorf("DistanceAt(%v) = %d, want %d", c.p, got, c.want)
		}
	}
}
