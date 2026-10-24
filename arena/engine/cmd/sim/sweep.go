package main

import (
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"
	"text/tabwriter"
)

// sweepAxis is one swept rule and the values it takes.
type sweepAxis struct {
	key    string
	values []int
}

func (c *config) addSweep(spec string) error {
	key, vals, ok := strings.Cut(spec, "=")
	if !ok {
		return fmt.Errorf("sweep %q: want key=lo..hi or key=a,b,c", spec)
	}
	values, err := parseValues(vals)
	if err != nil {
		return fmt.Errorf("sweep %s: %w", key, err)
	}
	for _, v := range values {
		probe := c.rules
		if err := probe.Set(key, v); err != nil {
			return fmt.Errorf("sweep: %w", err)
		}
	}
	c.sweeps = append(c.sweeps, sweepAxis{key: key, values: values})
	return nil
}

func parseValues(s string) ([]int, error) {
	if lo, hi, ok := strings.Cut(s, ".."); ok {
		return parseRange(lo, hi)
	}
	var out []int
	for _, item := range strings.Split(s, ",") {
		n, err := strconv.Atoi(item)
		if err != nil {
			return nil, fmt.Errorf("bad value %q", item)
		}
		out = append(out, n)
	}
	return out, nil
}

func parseRange(lo, hi string) ([]int, error) {
	a, errA := strconv.Atoi(lo)
	b, errB := strconv.Atoi(hi)
	if errA != nil || errB != nil || a > b {
		return nil, fmt.Errorf("bad range %s..%s", lo, hi)
	}
	var out []int
	for v := a; v <= b; v++ {
		out = append(out, v)
	}
	return out, nil
}

// combos lists every combination of the axes, the first axis varying slowest.
func combos(axes []sweepAxis) [][]int {
	out := [][]int{nil}
	for _, ax := range axes {
		var next [][]int
		for _, prefix := range out {
			for _, v := range ax.values {
				next = append(next, append(slices.Clone(prefix), v))
			}
		}
		out = next
	}
	return out
}

// runSweep plays the batch once per combination and prints a compact table.
func (c config) runSweep(w io.Writer) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	for _, ax := range c.sweeps {
		fmt.Fprintf(tw, "%s\t", ax.key)
	}
	fmt.Fprintln(tw, "kill/100b\tself%\tacc\t1st-hit\tlife\tidle%\tdodge\tcoins/m\tsmart%")
	for _, vals := range combos(c.sweeps) {
		cc := c
		for i, ax := range c.sweeps {
			_ = cc.rules.Set(ax.key, vals[i]) // keys were checked
			fmt.Fprintf(tw, "%d\t", vals[i])
		}
		res, err := cc.runBatch()
		if err != nil {
			return err
		}
		sweepRow(tw, res, c.dodgers)
	}
	return tw.Flush()
}

func sweepRow(w io.Writer, r result, dodgers []string) {
	s := r.total
	var all, dodge progStats
	smartKills := 0
	for name, p := range r.progs {
		all.BotStats = all.BotStats.Add(p.BotStats)
		if slices.Contains(dodgers, name) {
			dodge.BotStats = dodge.BotStats.Add(p.BotStats)
		}
		if name != randomName {
			smartKills += p.Kills
		}
	}
	fmt.Fprintf(w, "%.2f\t%.0f\t%.2f\t%.1f\t%.1f\t%.0f\t%.2f\t%.2f\t%.0f\n", 100*s.KillsPerBeat(),
		100*share(s.SelfKills, s.Deaths), s.Accuracy(), s.AvgFirstHit(), all.AvgLife(), 100*all.IdleShare(), dodge.DodgeRate(),
		float64(s.Coins)/float64(s.Matches), 100*share(smartKills, s.Kills))
}
