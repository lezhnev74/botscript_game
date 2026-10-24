package app

import (
	"testing"

	"github.com/lezhnev/arena/engine/domain/event"
	"github.com/lezhnev/arena/engine/domain/geo"
)

func scanned(bot string, d geo.Dir, enemy, bullet int) event.Scanned {
	return event.Scanned{Bot: bot, Dir: d, Enemy: enemy, Bullet: bullet}
}

func at(beat int, e event.Event) event.Logged { return event.Logged{Beat: beat, Event: e} }

func botLog() []event.Logged {
	return []event.Logged{
		at(1, scanned("A", geo.E, 0, 0)),
		at(1, event.Moved{Bot: "B", Dir: geo.W}),
		at(2, scanned("A", geo.E, 3, 0)),
		at(2, event.Fired{Bot: "B", Dir: geo.W}),
		at(3, event.Fired{Bot: "A", Dir: geo.E}),
		at(3, scanned("B", geo.W, 0, 3)),
		at(4, event.Hit{Bot: "B", From: geo.W, Shooter: "A"}),
		at(4, event.Killed{Killer: "A", Victim: "B"}),
		at(5, event.Clicked{Bot: "A", Dir: geo.E}),
		at(6, event.CoinTaken{Bot: "A"}),
		at(7, event.Respawned{Bot: "B"}),
	}
}

func TestPerBotCountsActionsAndOutcomes(t *testing.T) {
	got := PerBot(botLog(), 8, []string{"A", "B"})
	type counts struct{ kills, deaths, coins, shots, hits, clicks int }
	c := func(s BotStats) counts { return counts{s.Kills, s.Deaths, s.Coins, s.Shots, s.Hits, s.Clicks} }
	if a, want := c(got["A"]), (counts{kills: 1, shots: 1, hits: 1, coins: 1, clicks: 1}); a != want {
		t.Errorf("A %+v", a)
	}
	if b, want := c(got["B"]), (counts{deaths: 1, shots: 1}); b != want {
		t.Errorf("B %+v", b)
	}
}

func TestPerBotAliveAndIdleBeats(t *testing.T) {
	got := PerBot(botLog(), 8, []string{"A", "B"})
	// A acts in beats 1,2,3,5 (scan, scan, fire, click): alive 8, idle 4.
	if a := got["A"]; a.AliveBeats != 8 || a.IdleBeats != 4 {
		t.Errorf("A %+v", a)
	}
	// B is dead for beats 5..7 (killed in 4, respawned in 7): alive 5; acts in 1,2,3 and is hit in 4 (no action): idle 2.
	if b := got["B"]; b.AliveBeats != 5 || b.IdleBeats != 2 {
		t.Errorf("B %+v", b)
	}
}

func TestPerBotDeadUntilTheEnd(t *testing.T) {
	log := []event.Logged{at(2, event.Killed{Victim: "B"})}
	if b := PerBot(log, 6, []string{"B"})["B"]; b.AliveBeats != 2 {
		t.Errorf("a bot that never respawns is alive for beats 1..2: %+v", b)
	}
}

func TestPerBotFirstLinedUpScan(t *testing.T) {
	a := PerBot(botLog(), 8, []string{"A", "B"})["A"]
	if a.FirstLineN != 1 || a.FirstLineSum != 2 {
		t.Errorf("A first saw an enemy on its line at beat 2: %+v", a)
	}
}

func TestPerBotBulletSeenAtLevel3IsDodgedUnlessItHitsFromThatSide(t *testing.T) {
	b := PerBot(botLog(), 8, []string{"A", "B"})["B"]
	if b.Seen3 != 1 || b.Dodged3 != 0 {
		t.Errorf("B saw a bullet from W at beat 3 and was hit from W at beat 4: %+v", b)
	}
	log := []event.Logged{at(3, scanned("B", geo.W, 0, 3)), at(5, event.Hit{Bot: "B", From: geo.N})}
	if b := PerBot(log, 8, []string{"B"})["B"]; b.Seen3 != 1 || b.Dodged3 != 1 {
		t.Errorf("a hit from another side is not the seen bullet: %+v", b)
	}
	late := []event.Logged{at(3, scanned("B", geo.W, 0, 3)), at(3+dodgeWindow+1, event.Hit{Bot: "B", From: geo.W})}
	if b := PerBot(late, 20, []string{"B"})["B"]; b.Dodged3 != 1 {
		t.Errorf("a hit after the window is another bullet: %+v", b)
	}
}

func TestPerBotCountsOneSightingPerWindowAndDirection(t *testing.T) {
	log := []event.Logged{
		at(3, scanned("B", geo.W, 0, 3)), at(4, scanned("B", geo.W, 0, 3)),
		at(4, scanned("B", geo.N, 0, 3)), at(3+dodgeWindow+2, scanned("B", geo.W, 0, 3)),
	}
	if b := PerBot(log, 20, []string{"B"})["B"]; b.Seen3 != 3 {
		t.Errorf("repeated scans of one bullet count once: %+v", b)
	}
}

func TestBotStatsAddSums(t *testing.T) {
	a := BotStats{Kills: 2, Deaths: 1, Shots: 4, Hits: 2, AliveBeats: 30, IdleBeats: 6, Seen3: 4, Dodged3: 3, FirstLineSum: 10, FirstLineN: 2, Coins: 1}
	if s := a.Add(a); s.Kills != 4 || s.AliveBeats != 60 || s.FirstLineN != 4 {
		t.Fatalf("%+v", s)
	}
}

func TestBotStatsRatios(t *testing.T) {
	s := BotStats{Deaths: 2, Shots: 4, Hits: 2, AliveBeats: 60, IdleBeats: 12, Seen3: 8, Dodged3: 6, FirstLineSum: 20, FirstLineN: 4}
	got := []float64{s.Accuracy(), s.IdleShare(), s.DodgeRate(), s.AvgLife(), s.AvgFirstLine()}
	want := []float64{0.5, 0.2, 0.75, 30, 5}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("ratio %d: got %v want %v", i, got[i], want[i])
		}
	}
	if (BotStats{AliveBeats: 9}).AvgLife() != 9 {
		t.Error("a bot that never died lived at least its alive beats")
	}
}

func TestPerBotIgnoresOwnerlessBullets(t *testing.T) {
	got := PerBot([]event.Logged{at(1, event.Hit{Bot: "B", From: geo.W})}, 3, []string{"B"})
	if _, ok := got[""]; ok || len(got) != 1 {
		t.Errorf("%+v", got)
	}
}
