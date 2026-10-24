package scenario

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "rewrite .golden.jsonl files next to the scenarios")

const scenarioDir = "../testdata/scenarios"

// TestScenarios runs every testdata/scenarios/*.scn as a subtest:
// `go test ./scenario -run Scenarios/wait_stays_put`.
func TestScenarios(t *testing.T) {
	files, err := filepath.Glob(filepath.Join(scenarioDir, "*.scn"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no scenarios found in %s (%v)", scenarioDir, err)
	}
	for _, file := range files {
		name := strings.TrimSuffix(filepath.Base(file), ".scn")
		t.Run(name, func(t *testing.T) { runFile(t, file) })
	}
}

func runFile(t *testing.T, file string) {
	t.Helper()
	src, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	s, err := ParseDir(string(src), scenarioDir)
	if err != nil {
		t.Fatalf("%s: %v", file, err)
	}
	res := Run(s)
	if err := Verify(s, res); err != nil {
		t.Errorf("%s:\n%v", file, err)
	}
	checkGolden(t, strings.TrimSuffix(file, ".scn")+".golden.jsonl", res)
}

func checkGolden(t *testing.T, path string, res Result) {
	t.Helper()
	got, err := GoldenJSONL(res.Log)
	if err != nil {
		t.Fatal(err)
	}
	if *update {
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("golden missing (%v): run `go test ./scenario -update`", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("event log differs from %s (review, then -update):\n--- golden\n%s--- actual\n%s", path, want, got)
	}
}
