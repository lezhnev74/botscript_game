package main

import (
	"math"
	"strings"
	"testing"
)

func TestCrapScore(t *testing.T) {
	cases := []struct {
		comp int
		cov  float64
		want float64
	}{
		{1, 1, 1},
		{7, 1, 7},
		{5, 0, 30},
		{10, 0.5, 100*0.125 + 10},
	}
	for _, c := range cases {
		if got := crapScore(c.comp, c.cov); math.Abs(got-c.want) > 1e-9 {
			t.Errorf("crapScore(%d,%v)=%v want %v", c.comp, c.cov, got, c.want)
		}
	}
}

func TestFnCoverage(t *testing.T) {
	fi := funcInfo{StartLine: 10, StartCol: 1, EndLine: 20, EndCol: 2}
	blocks := []block{
		{StartLine: 11, StartCol: 2, EndLine: 12, EndCol: 3, Stmts: 2, Count: 1},
		{StartLine: 13, StartCol: 2, EndLine: 14, EndCol: 3, Stmts: 3, Count: 0},
		{StartLine: 30, StartCol: 1, EndLine: 31, EndCol: 2, Stmts: 9, Count: 0},
		{StartLine: 5, StartCol: 1, EndLine: 6, EndCol: 2, Stmts: 9, Count: 1},
	}
	if got := fnCoverage(fi, blocks); math.Abs(got-0.4) > 1e-9 {
		t.Fatalf("got %v want 0.4", got)
	}
	if got := fnCoverage(fi, nil); got != 1 {
		t.Fatalf("no statements must count as covered, got %v", got)
	}
}

func TestEvaluateUnknownFileIsUncovered(t *testing.T) {
	fi := funcInfo{File: "p/a.go", Complexity: 3}
	r := evaluate(fi, map[string][]block{})
	if r.Cov != 0 || r.CRAP != crapScore(3, 0) {
		t.Fatalf("unexpected: %+v", r)
	}
}

func TestOffendersSortedAndFiltered(t *testing.T) {
	rs := []result{{CRAP: 3}, {CRAP: 20}, {CRAP: 8}, {CRAP: 9}}
	got := offenders(rs, 8)
	if len(got) != 3 || got[0].CRAP != 20 || got[1].CRAP != 9 || got[2].CRAP != 8 {
		t.Fatalf("unexpected offenders: %+v", got)
	}
}

func TestFormatResult(t *testing.T) {
	r := result{funcInfo: funcInfo{File: "p/a.go", StartLine: 4, Name: "F", Complexity: 9}, Cov: 0.5, CRAP: 19.5}
	s := formatResult(r)
	for _, part := range []string{"p/a.go:4", "F", "comp=9", "cov=50%", "crap=19.5"} {
		if !strings.Contains(s, part) {
			t.Errorf("%q missing %q", s, part)
		}
	}
}
