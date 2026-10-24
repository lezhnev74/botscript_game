package app_test

import (
	"regexp"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"
)

var houseNameFormat = regexp.MustCompile(`^[A-Z][A-Za-z]*Bot[0-9]*$`)

func houseNamesOf(r *rig) []string {
	var names []string
	for _, b := range r.a.Bots() {
		if b.House {
			names = append(names, b.Name)
		}
	}
	return names
}

func TestHouseBotsGetFunnyNamesAndKeepTheirIDs(t *testing.T) {
	r := newRig(t, 3)
	r.tick(t, 1)
	got := houseNamesOf(r)
	if strings.Join(got, ",") != "WobbleBot,SirBeepsBot,NoodleBot" {
		t.Errorf("names %v", got)
	}
	for i, b := range r.a.Bots() {
		if want := []string{"house-1", "house-2", "house-3"}[i]; b.ID != want {
			t.Errorf("id %q, want %q", b.ID, want)
		}
	}
	for _, e := range r.a.Leaderboard() {
		if e.Name == e.Bot {
			t.Errorf("board line still shows the id: %+v", e)
		}
	}
}

func TestHouseNamesStayUniqueAndWellFormedAsHousesChurn(t *testing.T) {
	r := newRig(t, 30)
	r.tick(t, 1)
	for _, n := range []int{0, 30, 0, 30} { // each refill hands out fresh numbers, so the list wraps
		if err := r.a.SetMinBots(n); err != nil {
			t.Fatal(err)
		}
		r.tick(t, 2)
		checkHouseNames(t, houseNamesOf(r))
	}
	if got := houseNamesOf(r); len(got) != 30 || !slices.ContainsFunc(got, func(n string) bool { return strings.HasSuffix(n, "Bot2") }) {
		t.Errorf("list should cycle with a number suffix: %v", got)
	}
}

func TestHouseNamesAreDeterministic(t *testing.T) {
	a, b := newRig(t, 6), newRig(t, 6)
	a.tick(t, 3)
	b.tick(t, 3)
	if strings.Join(houseNamesOf(a), ",") != strings.Join(houseNamesOf(b), ",") {
		t.Errorf("%v vs %v", houseNamesOf(a), houseNamesOf(b))
	}
}

func TestHouseNameSkipsANameAHumanWears(t *testing.T) {
	r := newRig(t, 2)
	r.join(t, "WobbleBot")
	r.tick(t, 1)
	got := houseNamesOf(r)
	if len(got) == 0 || got[0] != "SirBeepsBot" {
		t.Errorf("house names %v", got)
	}
}

func checkHouseNames(t *testing.T, names []string) {
	t.Helper()
	seen := map[string]bool{}
	for _, name := range names {
		if seen[name] || !houseNameFormat.MatchString(name) || utf8.RuneCountInString(name) > 24 {
			t.Errorf("bad or duplicate name %q", name)
		}
		seen[name] = true
	}
}
