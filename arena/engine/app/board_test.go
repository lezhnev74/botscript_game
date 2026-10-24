package app

import (
	"maps"
	"testing"

	"github.com/lezhnev/arena/engine/domain/event"
)

func killed(beat int, killer, victim string) event.Logged {
	return event.Logged{Beat: beat, Event: event.Killed{Killer: killer, Victim: victim}}
}

func entry(b *Board, id string) Entry {
	for _, e := range b.Top() {
		if e.Bot == id {
			return e
		}
	}
	return Entry{}
}

func coin(beat int, bot string) event.Logged {
	return event.Logged{Beat: beat, Event: event.CoinTaken{Bot: bot}}
}

// line is score, kills, deaths, coins.
func line(e Entry) [4]int { return [4]int{e.Score, e.Kills, e.Deaths, e.Coins} }

func TestBoardScoresLikeVisionSection9(t *testing.T) {
	b := NewBoard(nil)
	changed := b.Apply([]event.Logged{
		killed(1, "A", "B"), killed(2, "A", "B"), killed(3, "B", "B"), killed(4, "", "C"),
		coin(5, "A"), {Beat: 5, Event: event.Moved{Bot: "A"}},
		killed(6, "A", "C"), killed(7, "A", "B"),
	})
	if !changed {
		t.Error("Apply should report a change")
	}
	for id, want := range map[string][4]int{
		"A": {3 + 1 + 1 + 3 + 1, 4, 0, 1}, // first B +3, repeat B +1, coin +1, first C +3, repeat B +1
		"B": {0, 0, 4, 0},                 // deaths never push the score below 0
		"C": {0, 0, 2, 0},
	} {
		if got := line(entry(b, id)); got != want {
			t.Errorf("%s = %v, want %v (score, kills, deaths, coins)", id, got, want)
		}
	}
}

func TestBoardScoreFloorDependsOnEventOrder(t *testing.T) {
	deathFirst, coinFirst := NewBoard(nil), NewBoard(nil)
	deathFirst.Apply([]event.Logged{killed(1, "A", "B"), coin(2, "B")})
	coinFirst.Apply([]event.Logged{coin(1, "B"), killed(2, "A", "B")})
	if a, b := entry(deathFirst, "B").Score, entry(coinFirst, "B").Score; a != 1 || b != 0 {
		t.Errorf("death then coin = %d, coin then death = %d; want 1 and 0", a, b)
	}
}

func TestBoardRemembersPreyAcrossRestarts(t *testing.T) {
	first := NewBoard(nil)
	first.Apply([]event.Logged{killed(1, "A", "B")})
	reloaded := NewBoard(first.Top())
	reloaded.Apply([]event.Logged{killed(9, "A", "B")})
	if got := entry(reloaded, "A").Score; got != 3+1 {
		t.Errorf("score %d: a kill after a reload should count as a repeat", got)
	}
	if entry(first, "A").Score != 3 {
		t.Error("reloading changed the original board")
	}
}

func TestBoardIgnoresEventsThatDoNotScore(t *testing.T) {
	b := NewBoard(nil)
	if b.Apply([]event.Logged{{Beat: 1, Event: event.Moved{Bot: "A"}}}) || len(b.Top()) != 0 {
		t.Error("a move changed the board")
	}
}

func TestBoardTopOrdersByScoreThenKillsThenID(t *testing.T) {
	b := NewBoard([]Entry{
		{Bot: "d", Score: 4, Kills: 1}, {Bot: "c", Score: 4, Kills: 3}, {Bot: "b", Score: 4, Kills: 1},
		{Bot: "a", Score: 9}, {Bot: "e"},
	})
	var order string
	for _, e := range b.Top() {
		order += e.Bot
	}
	if order != "acbde" {
		t.Errorf("order %s, want acbde", order)
	}
}

func TestBoardSetNameCreatesAndKeepsStats(t *testing.T) {
	b := NewBoard([]Entry{{Bot: "p1", Kills: 4}})
	if !b.SetName("p1", "Ann") || !b.SetName("p2", "Bob") || b.SetName("p1", "Ann") {
		t.Error("SetName must report a change exactly when the name changed")
	}
	if e := entry(b, "p1"); e.Name != "Ann" || e.Kills != 4 {
		t.Errorf("p1 = %+v", e)
	}
	if e := entry(b, "p2"); e.Name != "Bob" {
		t.Errorf("p2 = %+v", e)
	}
}

func TestBoardTopIsACopy(t *testing.T) {
	b := NewBoard([]Entry{{Bot: "a", Kills: 1, Prey: map[string]int{"x": 1}}})
	b.Top()[0].Prey["x"] = 99
	b.Top()[0].Kills = 99
	if entry(b, "a").Kills != 1 || entry(b, "a").Prey["x"] != 1 {
		t.Error("Top leaked internal state")
	}
}

// projection counts the log through the generic Name/Args view of events,
// without using Board: the independent oracle of the leaderboard tests. It
// returns score, kills, deaths, coins per bot.
func projection(log []event.Logged) map[string][4]int {
	out := map[string][4]int{}
	killedBefore := map[[2]string]bool{}
	for _, l := range log {
		a := l.Event.Args()
		switch l.Event.Name() {
		case "Killed":
			add(out, a[1], -1, 0, 1, 0)
			if a[0] != "" && a[0] != a[1] {
				add(out, a[0], killPoints(killedBefore, a[0], a[1]), 1, 0, 0)
			}
		case "CoinTaken":
			add(out, a[0], 1, 0, 0, 1)
		}
	}
	return out
}

// killPoints is 3 for a first kill of the victim by the killer, 1 after that.
func killPoints(before map[[2]string]bool, killer, victim string) int {
	pair := [2]string{killer, victim}
	defer func() { before[pair] = true }()
	if before[pair] {
		return 1
	}
	return 3
}

func add(out map[string][4]int, id string, score, kills, deaths, coins int) {
	v := out[id]
	v[0] = max(v[0]+score, 0)
	v[1], v[2], v[3] = v[1]+kills, v[2]+deaths, v[3]+coins
	out[id] = v
}

func boardStats(b *Board) map[string][4]int {
	got := map[string][4]int{}
	for _, e := range b.Top() {
		got[e.Bot] = line(e)
	}
	return got
}

func TestBoardEqualsTheIndependentProjectionOfRandomMatches(t *testing.T) {
	deaths := 0
	for seed := uint64(1); seed <= 40; seed++ {
		rec := RandomRecord(seed, RandomParams{W: 5 + int(seed%5), H: 5 + int(seed%4), Bots: 2 + int(seed%3), Beats: 150})
		log, err := Replay(rec)
		if err != nil {
			t.Fatal(err)
		}
		want := projection(log)
		if got := boardStats(ProjectBoard(log)); !maps.Equal(got, want) {
			t.Fatalf("seed %d: board %v, independent projection %v", seed, got, want)
		}
		for _, v := range want {
			deaths += v[2]
		}
	}
	if deaths == 0 {
		t.Error("no seed produced a death: the test proves nothing")
	}
}

func TestCheckBoardReportsMismatches(t *testing.T) {
	log := []event.Logged{killed(1, "A", "B")}
	b := NewBoard([]Entry{{Bot: "A", Kills: 1, Score: 3}, {Bot: "B", Deaths: 1}})
	if err := CheckBoard(b, log); err != nil {
		t.Fatal(err)
	}
	for name, bad := range map[string]*Board{
		"extra kill":  NewBoard([]Entry{{Bot: "A", Kills: 2, Score: 3}, {Bot: "B", Deaths: 1}}),
		"wrong score": NewBoard([]Entry{{Bot: "A", Kills: 1, Score: 4}, {Bot: "B", Deaths: 1}}),
		"missing bot": NewBoard([]Entry{{Bot: "A", Kills: 1, Score: 3}}),
		"extra bot":   NewBoard([]Entry{{Bot: "A", Kills: 1, Score: 3}, {Bot: "B", Deaths: 1}, {Bot: "Z", Kills: 1}}),
	} {
		if CheckBoard(bad, log) == nil {
			t.Errorf("%s: not detected", name)
		}
	}
}

func TestCheckBoardAllowsJoinedBotsWithoutStats(t *testing.T) {
	b := NewBoard([]Entry{{Bot: "A", Kills: 1, Score: 3}, {Bot: "B", Deaths: 1}})
	b.SetName("Z", "idle")
	if err := CheckBoard(b, []event.Logged{killed(1, "A", "B")}); err != nil {
		t.Error(err)
	}
}

func TestProjectionOracleAgreesWithBoardOnAHandLog(t *testing.T) {
	log := []event.Logged{killed(1, "A", "B"), killed(2, "B", "B"), {Beat: 3, Event: event.CoinTaken{Bot: "A"}}}
	want := map[string][4]int{"A": {4, 1, 0, 1}, "B": {0, 0, 2, 0}}
	if got := projection(log); !maps.Equal(got, want) {
		t.Errorf("oracle %v, want %v", got, want)
	}
}

func TestBoardUsesConfiguredPoints(t *testing.T) {
	b := NewBoard(nil).WithPoints(Points{Kill: 5, Repeat: 2, Coin: 3, Death: -2})
	b.Apply([]event.Logged{coin(1, "A"), coin(2, "B"), coin(3, "B"), killed(4, "A", "B"), killed(5, "A", "B")})
	if a, bb := line(entry(b, "A")), line(entry(b, "B")); a != [4]int{3 + 5 + 2, 2, 0, 1} || bb != [4]int{2, 0, 2, 2} {
		t.Errorf("A = %v, B = %v", a, bb)
	}
	log := []event.Logged{coin(1, "A"), killed(2, "A", "B")}
	if err := CheckBoard(b, log); err == nil {
		t.Error("CheckBoard should use the board's own points, and this board has other events")
	}
	same := NewBoard(nil).WithPoints(Points{Coin: 3})
	same.Apply(log)
	if err := CheckBoard(same, log); err != nil {
		t.Error(err)
	}
}

func TestDefaultPointsAreVisionSection9(t *testing.T) {
	if got := DefaultPoints(); got != (Points{Kill: 3, Repeat: 1, Coin: 1, Death: -1}) {
		t.Errorf("%+v", got)
	}
}

func TestBoardRemoveDeletesTheLineForGood(t *testing.T) {
	b := NewBoard([]Entry{{Bot: "A", Score: 5}, {Bot: "B"}})
	b.Remove("A")
	b.Apply([]event.Logged{killed(1, "A", "B")}) // a bullet of A still in flight
	if b.Has("A") || entry(b, "B").Deaths != 1 {
		t.Errorf("%+v", b.Top())
	}
}
