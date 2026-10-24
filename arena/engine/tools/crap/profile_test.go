package main

import (
	"strings"
	"testing"
)

func TestParseProfile(t *testing.T) {
	in := "mode: set\nm/a/b.go:3.10,5.2 2 1\nm/a/c.go:7.1,9.3 4 0\n"
	got, err := parseProfile(strings.NewReader(in))
	if err != nil {
		t.Fatal(err)
	}
	want := []block{
		{File: "m/a/b.go", StartLine: 3, StartCol: 10, EndLine: 5, EndCol: 2, Stmts: 2, Count: 1},
		{File: "m/a/c.go", StartLine: 7, StartCol: 1, EndLine: 9, EndCol: 3, Stmts: 4, Count: 0},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d blocks, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("block %d: got %+v want %+v", i, got[i], want[i])
		}
	}
}

func TestParseProfileMalformed(t *testing.T) {
	for _, in := range []string{"nocolon 1 2\n", "a.go:1.2,3.4 x y\n", "a.go:1.2 3 4\n"} {
		if _, err := parseProfile(strings.NewReader(in)); err == nil {
			t.Errorf("expected error for %q", in)
		}
	}
}

func TestGroupByFileStripsModule(t *testing.T) {
	blocks := []block{{File: "mod/p/a.go", StartLine: 1}, {File: "mod/p/a.go", StartLine: 2}, {File: "mod/q/b.go"}}
	got := groupByFile(blocks, "mod")
	if len(got["p/a.go"]) != 2 || len(got["q/b.go"]) != 1 {
		t.Fatalf("unexpected grouping: %v", got)
	}
}

func TestGroupByFileMergesDuplicateBlocksKeepingMaxCount(t *testing.T) {
	blocks := []block{
		{File: "mod/p/a.go", StartLine: 1, StartCol: 1, EndLine: 2, EndCol: 2, Stmts: 3, Count: 1},
		{File: "mod/p/a.go", StartLine: 1, StartCol: 1, EndLine: 2, EndCol: 2, Stmts: 3, Count: 0},
		{File: "mod/p/a.go", StartLine: 3, StartCol: 1, EndLine: 4, EndCol: 2, Stmts: 1, Count: 0},
		{File: "mod/p/a.go", StartLine: 3, StartCol: 1, EndLine: 4, EndCol: 2, Stmts: 1, Count: 0},
	}
	got := groupByFile(blocks, "mod")["p/a.go"]
	if len(got) != 2 || got[0].Count != 1 || got[1].Count != 0 {
		t.Fatalf("unexpected merge: %+v", got)
	}
	fi := funcInfo{StartLine: 1, StartCol: 1, EndLine: 2, EndCol: 3}
	if c := fnCoverage(fi, got); c != 1 {
		t.Fatalf("coverage %v want 1", c)
	}
}
