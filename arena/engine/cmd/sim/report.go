package main

import (
	"fmt"
	"io"
	"maps"
	"slices"
	"text/tabwriter"
)

func report(w io.Writer, r result) {
	s := r.total
	fmt.Fprintf(w, "matches %d  beats %d\n", s.Matches, s.Beats)
	fmt.Fprintf(w, "kills %d (%.4f per beat)  self-kills %d  deaths %d\n", s.Kills, s.KillsPerBeat(), s.SelfKills, s.Deaths)
	fmt.Fprintf(w, "shots %d  hits %d  accuracy %.3f\n", s.Shots, s.Hits, s.Accuracy())
	fmt.Fprintf(w, "avg beats to first hit %.1f (%d of %d matches had a hit)\n", s.AvgFirstHit(), s.MatchesWithHit, s.Matches)
	fmt.Fprintf(w, "coins taken %d\n", s.Coins)
	programTable(w, r)
	fmt.Fprintf(w, "hash %s\n", r.digest)
}

// programTable prints one row per program: how it fared, to read VISION 12's goals off it.
// life = alive beats per death; idle = alive beats with no action; dodge = level-3 bullet
// readings that did not end in a hit from that side; line = mean beat of the first level-3
// enemy reading; share = the program's part of all kills.
func programTable(w io.Writer, r result) {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "program\tbots\tkills\tdeaths\tshare\tacc\tlife\tidle\tdodge\tline\tcoins\tclicks\tscore")
	for _, name := range slices.Sorted(maps.Keys(r.progs)) {
		p := r.progs[name]
		fmt.Fprintf(tw, "%s\t%d\t%d\t%d\t%.2f\t%.2f\t%.1f\t%.2f\t%.2f\t%.1f\t%d\t%d\t%d\n", name, p.Bots, p.Kills, p.Deaths,
			share(p.Kills, r.total.Kills), p.Accuracy(), p.AvgLife(), p.IdleShare(), p.DodgeRate(), p.AvgFirstLine(), p.Coins, p.Clicks, p.Score)
	}
	tw.Flush()
}

func share(a, b int) float64 {
	if b == 0 {
		return 0
	}
	return float64(a) / float64(b)
}
