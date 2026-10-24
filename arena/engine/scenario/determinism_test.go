package scenario

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/lezhnev/arena/engine/app"
)

// hashEnv, when set, names a file TestDeterminism writes "<scenario> <hash>"
// lines to. `make determinism` runs the test under several schedulers and
// compares the files byte for byte.
const hashEnv = "ARENA_HASH_OUT"

func hashOf(t *testing.T, res Result) string {
	t.Helper()
	h, err := app.HashLog(res.Log)
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func replayHash(t *testing.T, s Scenario) string {
	t.Helper()
	data, err := json.Marshal(s.Record())
	if err != nil {
		t.Fatal(err)
	}
	rec, err := app.DecodeRecord(data)
	if err != nil {
		t.Fatal(err)
	}
	log, err := app.Replay(rec)
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	return hashOf(t, Result{Log: log})
}

// TestDeterminism is proof 1: every scenario run twice, and once more through
// Replay(Record) after a JSON round trip, gives one hash.
func TestDeterminism(t *testing.T) {
	files, err := filepath.Glob(filepath.Join(scenarioDir, "*.scn"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no scenarios found (%v)", err)
	}
	sort.Strings(files)
	var lines []string
	for _, file := range files {
		name := strings.TrimSuffix(filepath.Base(file), ".scn")
		t.Run(name, func(t *testing.T) { lines = append(lines, checkHashes(t, name, file)) })
	}
	t.Run("random_matches", func(t *testing.T) { lines = append(lines, randomHashes(t)...) })
	writeHashes(t, lines)
}

// randomHashes plays a few generated 9x9 four-bot matches (the heavy kind
// FuzzMatch makes) so the scheduler comparison covers busy fights too.
func randomHashes(t *testing.T) []string {
	t.Helper()
	var lines []string
	for seed := uint64(1); seed <= 8; seed++ {
		rec := app.RandomRecord(seed, app.RandomParams{W: 9, H: 9, Bots: 4, Beats: 150})
		var hashes [2]string
		for i := range hashes {
			log, err := app.Replay(rec)
			if err != nil {
				t.Fatal(err)
			}
			hashes[i] = hashOf(t, Result{Log: log})
		}
		if hashes[0] != hashes[1] {
			t.Errorf("random seed %d: %s vs %s", seed, hashes[0], hashes[1])
		}
		lines = append(lines, fmt.Sprintf("random_%d %s", seed, hashes[0]))
	}
	return lines
}

func checkHashes(t *testing.T, name, file string) string {
	t.Helper()
	src, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	s, err := ParseDir(string(src), scenarioDir)
	if err != nil {
		t.Fatal(err)
	}
	first, second, replayed := hashOf(t, Run(s)), hashOf(t, Run(s)), replayHash(t, s)
	if first != second || first != replayed {
		t.Errorf("hashes differ: run1 %s run2 %s replay %s", first, second, replayed)
	}
	return fmt.Sprintf("%s %s", name, first)
}

func writeHashes(t *testing.T, lines []string) {
	t.Helper()
	path := os.Getenv(hashEnv)
	if path == "" {
		return
	}
	sort.Strings(lines)
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}
