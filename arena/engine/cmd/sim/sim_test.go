package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func runSim(t *testing.T, args ...string) (string, error) {
	t.Helper()
	var out, errOut bytes.Buffer
	err := run(args, &out, &errOut)
	return out.String(), err
}

func TestRunPrintsStats(t *testing.T) {
	out, err := runSim(t, "-matches", "5", "-beats", "60", "-seed", "3")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"matches 5", "beats 300", "kills", "avg beats to first hit", "coins taken", "hash "} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
}

func TestRunIsReproducible(t *testing.T) {
	a, _ := runSim(t, "-matches", "4", "-beats", "80", "-seed", "9")
	b, _ := runSim(t, "-matches", "4", "-beats", "80", "-seed", "9")
	c, _ := runSim(t, "-matches", "4", "-beats", "80", "-seed", "10")
	if a != b || a == c {
		t.Errorf("same seed must print the same, other seed differ:\n%s\n%s\n%s", a, b, c)
	}
}

func TestRunOnAMapFileWithScripts(t *testing.T) {
	path := filepath.Join(t.TempDir(), "duel.map")
	if err := os.WriteFile(path, []byte("#######\n#A...B#\n#######\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := runSim(t, "-map", path, "-matches", "1", "-beats", "8",
		"-script", "A=fire E, wait, wait, wait, fire E", "-script", "B=wait")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "shots 2 ") || !strings.Contains(out, "hits 2 ") {
		t.Errorf("A fires twice at idle B:\n%s", out)
	}
}

func TestRunAppliesRuleOverrides(t *testing.T) {
	path := filepath.Join(t.TempDir(), "duel.map")
	_ = os.WriteFile(path, []byte("#######\n#A...B#\n#######\n"), 0o644)
	out, err := runSim(t, "-map", path, "-matches", "1", "-beats", "4", "-rule", "hp=1", "-rule", "bullet_speed=4",
		"-script", "A=fire E")
	if err != nil || !strings.Contains(out, "kills 1 ") {
		t.Errorf("hp=1 and a fast bullet should kill B in one beat: %v\n%s", err, out)
	}
}

func TestRunRejectsBadInput(t *testing.T) {
	cases := map[string][]string{
		"unknown flag":   {"-nope"},
		"bad size":       {"-size", "big"},
		"missing map":    {"-map", "/no/such/file"},
		"bad rule":       {"-rule", "luck=3"},
		"rule no value":  {"-rule", "hp"},
		"bad script":     {"-script", "A=jump N"},
		"script no id":   {"-script", "fire E"},
		"script unknown": {"-map", "", "-script", "Z=wait"},
		"zero matches":   {"-matches", "0"},
		"negative coin":  {"-coin-points", "-1"},
	}
	for name, args := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := runSim(t, args...); err == nil {
				t.Error("want an error")
			}
		})
	}
}

func writeMap(t *testing.T, text string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "room.map")
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

const openRoom = "#########\n#.......#\n#.......#\n#.......#\n#.......#\n#.......#\n#########\n"

func TestRunNamesHouseProgramsAndPrintsAPerProgramTable(t *testing.T) {
	out, err := runSim(t, "-map", writeMap(t, openRoom), "-bots", "rookie,lighthouse,tracker,random", "-matches", "6", "-beats", "80")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"program", "rookie", "lighthouse", "tracker", "random", "share", "dodge", "idle"} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
}

func TestRunProgramFlagAssignsAHouseBotToAMapLetter(t *testing.T) {
	path := writeMap(t, "#######\n#A...B#\n#######\n")
	out, err := runSim(t, "-map", path, "-matches", "1", "-beats", "10", "-program", "A=rookie", "-script", "B=wait")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "rookie") || !strings.Contains(out, "script") {
		t.Errorf("rookie and the scripted bot should be listed:\n%s", out)
	}
}

func TestRunCountsBotsOnABotFreeMap(t *testing.T) {
	out, err := runSim(t, "-map", writeMap(t, openRoom), "-bots", "5", "-matches", "2", "-beats", "20")
	if err != nil || !strings.Contains(out, "random") {
		t.Errorf("%v\n%s", err, out)
	}
}

func TestRunRejectsBadLineups(t *testing.T) {
	room := writeMap(t, openRoom)
	duel := writeMap(t, "#######\n#A...B#\n#######\n")
	cases := map[string][]string{
		"unknown program":      {"-map", room, "-bots", "rookie,nobody"},
		"too many for a room":  {"-bots", "rookie,rookie,rookie,rookie,rookie"},
		"list vs map bots":     {"-map", duel, "-bots", "rookie,rookie,rookie"},
		"program no id":        {"-program", "rookie"},
		"program unknown name": {"-map", duel, "-program", "A=nobody"},
		"program unknown bot":  {"-map", duel, "-program", "Z=rookie"},
		"zero bots":            {"-map", room, "-bots", "0"},
	}
	for name, args := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := runSim(t, args...); err == nil {
				t.Error("want an error")
			}
		})
	}
}

func TestSweepPrintsOneRowPerCombination(t *testing.T) {
	out, err := runSim(t, "-map", writeMap(t, openRoom), "-bots", "rookie,tracker", "-matches", "3", "-beats", "40",
		"-sweep", "bullet_speed=2..3", "-sweep", "reload=2,4,6")
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 1+6 {
		t.Fatalf("want a header and 6 rows, got %d:\n%s", len(lines), out)
	}
	for _, want := range []string{"bullet_speed", "reload", "kill/100b", "acc", "life", "idle%", "dodge", "smart%"} {
		if !strings.Contains(lines[0], want) {
			t.Errorf("header lacks %q: %s", want, lines[0])
		}
	}
	if !strings.HasPrefix(strings.Fields(lines[1])[0], "2") || !strings.HasPrefix(strings.Fields(lines[6])[0], "3") {
		t.Errorf("rows should run through the first sweep slowest:\n%s", out)
	}
}

func TestSweepIsReproducible(t *testing.T) {
	args := []string{"-map", writeMap(t, openRoom), "-bots", "rookie,lighthouse,tracker", "-matches", "4", "-beats", "50", "-sweep", "reload=2..3"}
	a, _ := runSim(t, args...)
	b, _ := runSim(t, args...)
	if a != b || a == "" {
		t.Errorf("sweeps must not depend on goroutine timing:\n%s\n%s", a, b)
	}
}

func TestSweepRejectsBadSpecs(t *testing.T) {
	for name, spec := range map[string]string{
		"no equals": "reload", "unknown rule": "luck=1..2", "reversed": "reload=4..2", "not numbers": "reload=a..b",
		"empty list": "reload=", "bad item": "reload=1,x",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := runSim(t, "-sweep", spec); err == nil {
				t.Error("want an error")
			}
		})
	}
}

func TestBotsListAcceptsRepeatCounts(t *testing.T) {
	c, err := parseConfig([]string{"-bots", "rookie:2,tracker,random:3"}, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"rookie", "rookie", "tracker", "random", "random", "random"}
	if !slices.Equal(c.lineup, want) {
		t.Errorf("lineup %v", c.lineup)
	}
	for _, bad := range []string{"rookie:0", "rookie:x", "rookie:-1"} {
		if _, err := parseConfig([]string{"-bots", bad}, io.Discard); err == nil {
			t.Errorf("%q should fail", bad)
		}
	}
}

func TestSweepAndRuleRejectOutOfRangeValues(t *testing.T) {
	for _, args := range [][]string{{"-sweep", "bullet_speed=0..2"}, {"-sweep", "hp=-1,2"}, {"-rule", "reload=0"}} {
		if _, err := runSim(t, args...); err == nil {
			t.Errorf("%v should fail", args)
		}
	}
}

// column returns the header column of the first row accepted by row (a sweep or single-run table).
func column(t *testing.T, out, header string, row func(string) bool) string {
	t.Helper()
	lines := strings.Split(strings.TrimSpace(out), "\n")
	var cols []string
	for _, l := range lines {
		f := strings.Fields(l)
		if len(f) > 0 && slices.Contains(f, header) {
			cols = f
		}
		if len(f) > 0 && row(l) {
			return f[slices.Index(cols, header)]
		}
	}
	t.Fatalf("no row for %q in:\n%s", header, out)
	return ""
}

func TestSweepDodgeColumnCoversOnlyTheDodgers(t *testing.T) {
	room := writeMap(t, openRoom)
	base := []string{"-map", room, "-bots", "rookie,tracker", "-matches", "30", "-beats", "100"}
	single, err := runSim(t, base...)
	if err != nil {
		t.Fatal(err)
	}
	want := column(t, single, "dodge", func(l string) bool { return strings.HasPrefix(l, "tracker") })
	sweep, err := runSim(t, append(base, "-sweep", "reload=3", "-dodgers", "tracker")...)
	if err != nil {
		t.Fatal(err)
	}
	if got := column(t, sweep, "dodge", func(l string) bool { return strings.HasPrefix(l, "3 ") }); got != want {
		t.Errorf("sweep dodge %s, tracker's own dodge %s", got, want)
	}
	if _, err := runSim(t, append(base, "-dodgers", "nobody")...); err == nil {
		t.Error("an unknown dodger program should fail")
	}
}

func TestCoinPointsChangeScoresButNotTheEventLog(t *testing.T) {
	base := []string{"-matches", "6", "-beats", "150", "-seed", "5"}
	one, err := runSim(t, append(base, "-coin-points", "1")...)
	if err != nil {
		t.Fatal(err)
	}
	nine, _ := runSim(t, append(base, "-coin-points", "9")...)
	hash := func(s string) string { return s[strings.LastIndex(s, "hash "):] }
	if hash(one) != hash(nine) {
		t.Error("coin points must not change the event log")
	}
	if one == nine || !strings.Contains(one, "score") {
		t.Errorf("scores should differ with coin points:\n%s\n%s", one, nine)
	}
}
