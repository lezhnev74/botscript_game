package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"runtime"
	"sync"

	"github.com/lezhnev/arena/engine/app"
)

// progStats are the per-program numbers summed over matches.
type progStats struct {
	Bots  int // bot instances (matches x bots playing it)
	Score int // leaderboard score summed over bots and matches (coin points: -coin-points)
	app.BotStats
}

// result is everything a batch of matches measured.
type result struct {
	total  app.Stats
	progs  map[string]progStats
	digest string
}

// outcome is one finished match.
type outcome struct {
	stats app.Stats
	progs map[string]progStats
	hash  string
	err   error
}

func run(args []string, stdout, stderr io.Writer) error {
	c, err := parseConfig(args, stderr)
	if err != nil {
		return err
	}
	if len(c.sweeps) > 0 {
		return c.runSweep(stdout)
	}
	res, err := c.runBatch()
	if err != nil {
		return err
	}
	report(stdout, res)
	return nil
}

// runBatch plays all matches (in parallel; results are folded in match order,
// so output never depends on scheduling).
func (c config) runBatch() (result, error) {
	outs := make([]outcome, c.matches)
	var wg sync.WaitGroup
	sem := make(chan struct{}, runtime.NumCPU())
	for i := range outs {
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			outs[i] = c.playMatch(i)
			<-sem
		}()
	}
	wg.Wait()
	return fold(outs)
}

func fold(outs []outcome) (result, error) {
	res := result{progs: map[string]progStats{}}
	digest := sha256.New()
	for i, o := range outs {
		if o.err != nil {
			return res, fmt.Errorf("match %d: %w", i+1, o.err)
		}
		res.total = res.total.Add(o.stats)
		for name, p := range o.progs {
			cur := res.progs[name]
			res.progs[name] = progStats{Bots: cur.Bots + p.Bots, Score: cur.Score + p.Score, BotStats: cur.BotStats.Add(p.BotStats)}
		}
		digest.Write([]byte(o.hash))
	}
	res.digest = hex.EncodeToString(digest.Sum(nil))
	return res, nil
}

func (c config) playMatch(i int) outcome {
	m, err := c.record(i)
	if err != nil {
		return outcome{err: err}
	}
	log, err := app.Replay(m.rec)
	if err != nil {
		return outcome{err: err}
	}
	hash, err := app.HashLog(log)
	ids := make([]string, len(m.rec.Bots))
	for k, b := range m.rec.Bots {
		ids[k] = b.ID
	}
	progs := map[string]progStats{}
	board := app.NewBoard(nil).WithPoints(c.points)
	board.Apply(log)
	scores := map[string]int{}
	for _, e := range board.Top() {
		scores[e.Bot] = e.Score
	}
	for id, s := range app.PerBot(log, m.rec.Beats, ids) {
		cur := progs[m.names[id]]
		progs[m.names[id]] = progStats{Bots: cur.Bots + 1, Score: cur.Score + scores[id], BotStats: cur.BotStats.Add(s)}
	}
	return outcome{stats: app.MatchStats(log, m.rec.Beats), progs: progs, hash: hash, err: err}
}
