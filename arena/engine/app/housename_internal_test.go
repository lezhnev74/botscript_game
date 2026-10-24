package app

import "testing"

func TestHouseNameCyclesWithANumberSuffix(t *testing.T) {
	n := len(houseNames)
	for k, want := range map[int]string{0: "WobbleBot", 1: "SirBeepsBot", n - 1: houseNames[n-1], n: "WobbleBot2", n + 2: "NoodleBot2", 2 * n: "WobbleBot3"} {
		if got := houseName(k); got != want {
			t.Errorf("houseName(%d) = %q, want %q", k, got, want)
		}
	}
}

func TestHouseNamesListIsFortyUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, name := range houseNames {
		if seen[name] {
			t.Errorf("duplicate %q", name)
		}
		seen[name] = true
	}
	if len(houseNames) < 40 {
		t.Errorf("%d names", len(houseNames))
	}
}
