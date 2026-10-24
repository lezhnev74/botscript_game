package main

import (
	"fmt"
	"sort"
)

// result is a function with its coverage and CRAP score.
type result struct {
	funcInfo
	Cov  float64
	CRAP float64
}

// crapScore is CRAP(m) = comp^2 * (1-cov)^3 + comp.
func crapScore(comp int, cov float64) float64 {
	u := 1 - cov
	return float64(comp*comp)*u*u*u + float64(comp)
}

func before(l1, c1, l2, c2 int) bool {
	return l1 < l2 || l1 == l2 && c1 <= c2
}

func inside(fi funcInfo, b block) bool {
	return before(fi.StartLine, fi.StartCol, b.StartLine, b.StartCol) &&
		before(b.EndLine, b.EndCol, fi.EndLine, fi.EndCol)
}

// fnCoverage is the fraction of statements in fi executed at least once.
// A function without statements counts as fully covered.
func fnCoverage(fi funcInfo, blocks []block) float64 {
	total, covered := 0, 0
	for _, b := range blocks {
		if !inside(fi, b) {
			continue
		}
		total += b.Stmts
		if b.Count > 0 {
			covered += b.Stmts
		}
	}
	if total == 0 {
		return 1
	}
	return float64(covered) / float64(total)
}

// evaluate scores fi. A file absent from the profile is entirely untested.
func evaluate(fi funcInfo, byFile map[string][]block) result {
	cov := 0.0
	if blocks, ok := byFile[fi.File]; ok {
		cov = fnCoverage(fi, blocks)
	}
	return result{funcInfo: fi, Cov: cov, CRAP: crapScore(fi.Complexity, cov)}
}

// offenders returns results with CRAP >= limit, worst first.
func offenders(rs []result, limit float64) []result {
	var out []result
	for _, r := range rs {
		if r.CRAP >= limit {
			out = append(out, r)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].CRAP > out[j].CRAP })
	return out
}

func formatResult(r result) string {
	return fmt.Sprintf("%s:%d %s comp=%d cov=%.0f%% crap=%.1f",
		r.File, r.StartLine, r.Name, r.Complexity, r.Cov*100, r.CRAP)
}
