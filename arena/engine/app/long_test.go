package app

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/lezhnev/arena/engine/domain/sim"
	"github.com/lezhnev/arena/engine/lang"
)

// Long goldens (PLAN section 3, proof 4): three recorded ~300 beat matches
// kept as Record JSON next to a pinned hash of their event log. Any engine,
// language or house-bot change that alters a single event shows up here.
// `go test ./app -update` rewrites the records from longRecords and re-pins
// the hashes; review the diff before committing.
var updateLong = flag.Bool("update", false, "rewrite testdata/long records and hashes")

const (
	longDir    = "../testdata/long"
	longHashes = "hashes.txt"
	longBeats  = 300
)

func house(name string) BrainSpec { return houseSpec(name) }

func longRecords() map[string]Record {
	base := func(mapText string, seed uint64, bots ...BotSpec) Record {
		return Record{Version: RecordVersion, Rules: sim.DefaultRules(), Map: mapText, Seed: seed, Beats: longBeats, Bots: bots}
	}
	four := base("#########\n#A.....B#\n#.##.##.#\n#.......#\n#...#...#\n#.......#\n#.##.##.#\n#C.....D#\n#########\n", 49,
		BotSpec{ID: "A", Brain: house("rookie")}, BotSpec{ID: "B", Brain: house("lighthouse")},
		BotSpec{ID: "C", Brain: house("tracker")}, BotSpec{ID: "D", Brain: house("lighthouse")})
	four.EditLog = []Edit{{Beat: 120, Bot: "A", Brain: house("tracker")}}
	return map[string]Record{
		"duel_5x5": base("#####\n#A..#\n#...#\n#..B#\n#####\n", 34,
			BotSpec{ID: "A", Brain: house("lighthouse")}, BotSpec{ID: "B", Brain: house("rookie")}),
		"four_9x9": four,
		"trio_7x7": base("#######\n#A....#\n#.....#\n#..B..#\n#.....#\n#....C#\n#######\n", 33,
			BotSpec{ID: "A", Brain: house("rookie")}, BotSpec{ID: "B", Brain: house("lighthouse")},
			BotSpec{ID: "C", Brain: house("tracker")}),
	}
}

func encodeRecord(rec Record) []byte {
	b, _ := json.MarshalIndent(rec, "", "  ") // a Record always marshals
	return append(b, '\n')
}

func recordPath(name string) string { return filepath.Join(longDir, name+".record.json") }

func replayHashOf(rec Record) (string, error) {
	log, err := Replay(rec)
	if err != nil {
		return "", err
	}
	return HashLog(log)
}

// writeLong regenerates the record files and the hash file.
func writeLong(t *testing.T) {
	t.Helper()
	recs := longRecords()
	var lines []string
	for name, rec := range recs {
		h, err := replayHashOf(rec)
		if err != nil {
			t.Fatal(err)
		}
		lines = append(lines, name+" "+h)
		if err := os.WriteFile(recordPath(name), encodeRecord(rec), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	sort.Strings(lines)
	if err := os.WriteFile(filepath.Join(longDir, longHashes), []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func pinnedHashes(t *testing.T) map[string]string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(longDir, longHashes))
	if err != nil {
		t.Fatalf("%v: run `go test ./app -update`", err)
	}
	out := map[string]string{}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	for _, l := range lines {
		name, h, _ := strings.Cut(l, " ")
		out[name] = h
	}
	return out
}

func TestLongGoldenReplay(t *testing.T) {
	if *updateLong {
		writeLong(t)
	}
	pinned := pinnedHashes(t)
	if len(pinned) != len(longRecords()) {
		t.Fatalf("%d pinned hashes for %d records", len(pinned), len(longRecords()))
	}
	for name, want := range pinned {
		t.Run(name, func(t *testing.T) { checkLong(t, name, want) })
	}
}

func loadLong(t *testing.T, name string) Record {
	t.Helper()
	data, err := os.ReadFile(recordPath(name))
	if err != nil {
		t.Fatal(err)
	}
	rec, err := DecodeRecord(data)
	if err != nil {
		t.Fatal(err)
	}
	return rec
}

func checkLong(t *testing.T, name, want string) {
	t.Helper()
	rec := loadLong(t, name)
	log, err := Replay(rec)
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := HashLog(log); got != want {
		t.Errorf("event log hash changed:\n got  %s\n want %s\n(if intended: go test ./app -update, review, commit)", got, want)
	}
	if s := MatchStats(log, rec.Beats); rec.Beats < longBeats || s.Kills < 3 || s.Shots == 0 {
		t.Errorf("a long golden must be a real fight: beats %d, %+v", rec.Beats, s)
	}
}

func TestLongRecordFilesMatchTheirBuilders(t *testing.T) {
	for name, rec := range longRecords() {
		got, err := os.ReadFile(recordPath(name))
		if err != nil || !bytes.Equal(got, encodeRecord(rec)) {
			t.Errorf("%s: record file differs from longRecords (go test ./app -update): %v", name, err)
		}
	}
}

func TestLongHouseProgramsAreEmbeddedInTheRecords(t *testing.T) {
	for name, rec := range longRecords() {
		for _, b := range rec.Bots {
			if _, errs := lang.Validate(b.Brain.Src); b.Brain.Kind != KindProgram || len(errs) > 0 {
				t.Errorf("%s bot %s: %+v %v", name, b.ID, b.Brain.Kind, errs)
			}
		}
	}
}
