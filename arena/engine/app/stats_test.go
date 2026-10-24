package app

import (
	"testing"

	"github.com/lezhnev/arena/engine/domain/event"
	"github.com/lezhnev/arena/engine/domain/geo"
)

func statsLog() []event.Logged {
	return []event.Logged{
		{Beat: 1, Event: event.Fired{Bot: "A", Dir: geo.E}},
		{Beat: 1, Event: event.Fired{Bot: "B", Dir: geo.W}},
		{Beat: 4, Event: event.Hit{Bot: "B", From: geo.W, Shooter: "A"}},
		{Beat: 4, Event: event.Killed{Killer: "A", Victim: "B"}},
		{Beat: 6, Event: event.Hit{Bot: "A", From: geo.E, Shooter: "A"}},
		{Beat: 6, Event: event.Killed{Killer: "A", Victim: "A"}},
		{Beat: 7, Event: event.CoinTaken{Bot: "A", X: 1, Y: 1}},
	}
}

func TestMatchStatsCountsEvents(t *testing.T) {
	s := MatchStats(statsLog(), 10)
	want := Stats{Matches: 1, Beats: 10, Shots: 2, Hits: 2, Kills: 1, SelfKills: 1, Deaths: 2, Coins: 1,
		MatchesWithHit: 1, FirstHitSum: 4}
	if s != want {
		t.Errorf("got %+v\nwant %+v", s, want)
	}
}

func TestStatsAddAndRatios(t *testing.T) {
	a := MatchStats(statsLog(), 10)
	total := a.Add(MatchStats(nil, 30)).Add(a)
	want := Stats{Matches: 3, Beats: 50, Shots: 4, Hits: 4, Kills: 2, SelfKills: 2, Deaths: 4, Coins: 2,
		MatchesWithHit: 2, FirstHitSum: 8}
	if total != want {
		t.Fatalf("total %+v", total)
	}
	if total.KillsPerBeat() != 2.0/50 || total.AvgFirstHit() != 4 || total.Accuracy() != 1 {
		t.Errorf("ratios %v %v %v", total.KillsPerBeat(), total.AvgFirstHit(), total.Accuracy())
	}
}

func TestStatsRatiosOfNothingAreZero(t *testing.T) {
	var s Stats
	if s.KillsPerBeat() != 0 || s.AvgFirstHit() != 0 || s.Accuracy() != 0 {
		t.Errorf("%+v", s)
	}
}
