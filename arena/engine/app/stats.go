package app

import "github.com/lezhnev/arena/engine/domain/event"

// Stats summarises one or more matches for tuning (cmd/sim). Kills counts
// Killed events with a killer other than the victim; SelfKills the rest that
// have a killer; Deaths all of them.
type Stats struct {
	Matches, Beats              int
	Shots, Hits                 int
	Kills, SelfKills, Deaths    int
	Coins                       int
	MatchesWithHit, FirstHitSum int // matches with at least one Hit, and the sum of their first-hit beats
}

// MatchStats reads the event log of a match that ran for the given beats.
func MatchStats(log []event.Logged, beats int) Stats {
	s := Stats{Matches: 1, Beats: beats}
	for _, l := range log {
		s.count(l)
	}
	return s
}

func (s *Stats) count(l event.Logged) {
	switch e := l.Event.(type) {
	case event.Fired:
		s.Shots++
	case event.Hit:
		s.countHit(l.Beat)
	case event.Killed:
		s.countKill(e)
	case event.CoinTaken:
		s.Coins++
	}
}

func (s *Stats) countHit(beat int) {
	if s.Hits == 0 {
		s.MatchesWithHit, s.FirstHitSum = 1, beat
	}
	s.Hits++
}

func (s *Stats) countKill(e event.Killed) {
	s.Deaths++
	switch {
	case e.Killer == "":
	case e.Killer == e.Victim:
		s.SelfKills++
	default:
		s.Kills++
	}
}

// Add combines the stats of two runs.
func (s Stats) Add(o Stats) Stats {
	return Stats{
		Matches: s.Matches + o.Matches, Beats: s.Beats + o.Beats,
		Shots: s.Shots + o.Shots, Hits: s.Hits + o.Hits,
		Kills: s.Kills + o.Kills, SelfKills: s.SelfKills + o.SelfKills, Deaths: s.Deaths + o.Deaths,
		Coins:          s.Coins + o.Coins,
		MatchesWithHit: s.MatchesWithHit + o.MatchesWithHit, FirstHitSum: s.FirstHitSum + o.FirstHitSum,
	}
}

// KillsPerBeat is kills over all beats played.
func (s Stats) KillsPerBeat() float64 { return ratio(s.Kills, s.Beats) }

// AvgFirstHit is the mean beat of the first hit, over matches that had one.
func (s Stats) AvgFirstHit() float64 { return ratio(s.FirstHitSum, s.MatchesWithHit) }

// Accuracy is hits per shot fired.
func (s Stats) Accuracy() float64 { return ratio(s.Hits, s.Shots) }

func ratio(a, b int) float64 {
	if b == 0 {
		return 0
	}
	return float64(a) / float64(b)
}
