package app

import (
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/lezhnev/arena/engine/domain/event"
	"github.com/lezhnev/arena/engine/domain/sim"
)

const firer = "function loop() { fire(E); }"
const idler = "function loop() { wait(); }"

func liveRecord() Record {
	rec := Record{Version: RecordVersion, Rules: duel().Rules, Map: "#######\n#.....#\n#.....#\n#######\n", Seed: 5, Beats: 8}
	rec.Joins = []Join{
		{Beat: 1, Bot: "p1", Brain: BrainSpec{Kind: KindProgram, Src: idler}},
		{Beat: 3, Bot: "p2", Brain: BrainSpec{Kind: KindProgram, Src: counter}},
	}
	rec.EditLog = []Edit{{Beat: 5, Bot: "p2", Brain: BrainSpec{Kind: KindProgram, Src: firer}}}
	rec.Leaves = []Leave{{Beat: 7, Bot: "p1"}}
	return rec
}

func TestReplayAppliesJoinsEditsAndLeaves(t *testing.T) {
	m, err := liveRecord().NewMatch()
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	for _, c := range []struct {
		beat int
		want string
	}{{1, "p1"}, {2, "p1"}, {3, "p1,p2"}, {6, "p1,p2"}, {7, "p2"}, {8, "p2"}} {
		beat, want := c.beat, c.want
		for m.World().Beat < beat {
			m.Step()
		}
		if got := ids(m); got != want {
			t.Errorf("beat %d: bots %q, want %q", beat, got, want)
		}
	}
	if err := m.Err(); err != nil {
		t.Error(err)
	}
}

func ids(m *Match) string {
	var out []string
	for _, b := range m.World().Bots {
		out = append(out, b.ID)
	}
	return strings.Join(out, ",")
}

func TestReplayOfAJoinRecordIsDeterministic(t *testing.T) {
	var hashes [2]string
	for i := range hashes {
		log, err := Replay(liveRecord())
		if err != nil {
			t.Fatal(err)
		}
		if hashes[i], err = HashLog(log); err != nil {
			t.Fatal(err)
		}
	}
	if hashes[0] != hashes[1] {
		t.Error("hashes differ")
	}
}

func TestLeaveRemovesTheBotAndItsBrain(t *testing.T) {
	m, err := liveRecord().NewMatch()
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	stepN(m, 1)
	if err := m.Leave("p1"); err != nil {
		t.Fatal(err)
	}
	if ids(m) != "" || m.Leave("p1") == nil {
		t.Errorf("bots %q, second leave must fail", ids(m))
	}
}

func TestJoinFailsOnDuplicateIDOrFullArena(t *testing.T) {
	m, err := duel().NewMatch()
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	br, _ := NewBrain(BrainSpec{Kind: KindProgram, Src: idler}, 1, "A")
	if m.Join("A", br) == nil {
		t.Error("duplicate id accepted")
	}
}

func TestJoinFailsWhenNoCellIsFree(t *testing.T) {
	rec := Record{Version: RecordVersion, Rules: duel().Rules, Map: "###\n#.#\n###\n", Beats: 2}
	rec.Joins = []Join{
		{Beat: 1, Bot: "p1", Brain: BrainSpec{Kind: KindProgram, Src: idler}},
		{Beat: 2, Bot: "p2", Brain: BrainSpec{Kind: KindProgram, Src: idler}},
	}
	if _, err := Replay(rec); err == nil || !strings.Contains(err.Error(), "p2") {
		t.Errorf("err = %v", err)
	}
}

func TestSwapNeedsAProgramBot(t *testing.T) {
	m, err := duel().NewMatch()
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	p, _ := compile(idler)
	if m.Swap("A", p) == nil || m.Swap("Z", p) == nil {
		t.Error("swap accepted on a scripted or missing bot")
	}
}

func TestAnEditedJoinedBotActsOnItsEditBeat(t *testing.T) {
	log, err := Replay(liveRecord())
	if err != nil {
		t.Fatal(err)
	}
	var acted []int
	for _, l := range log {
		if e, ok := l.Event.(event.Fired); ok && e.Bot == "p2" {
			acted = append(acted, l.Beat)
		}
	}
	if len(acted) == 0 || slices.Min(acted) != 5 {
		t.Errorf("p2 fired on beats %v, want the first on 5", acted)
	}
}

func TestBadScheduleEntriesAreRejected(t *testing.T) {
	cases := map[string]func(*Record){
		"join beat zero":  func(r *Record) { r.Joins[0].Beat = 0 },
		"join on map bot": func(r *Record) { r.Joins[0].Bot = "A" },
		"join twice":      func(r *Record) { r.Joins[1].Bot = "p1" },
		"join bad brain":  func(r *Record) { r.Joins[0].Brain.Src = "nope" },
		"join empty id":   func(r *Record) { r.Joins[0].Bot = "" },
		"leave unknown":   func(r *Record) { r.Leaves[0].Bot = "zz" },
		"leave beat zero": func(r *Record) { r.Leaves[0].Beat = 0 },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			rec := liveRecord()
			rec.Map = "#######\n#A....#\n#.....#\n#######\n"
			mutate(&rec)
			if _, err := Replay(rec); err == nil {
				t.Error("want an error")
			}
		})
	}
}

func TestJoinsAndLeavesAreLoggedAtTheirBeat(t *testing.T) {
	log, err := Replay(liveRecord())
	if err != nil {
		t.Fatal(err)
	}
	var texts []string
	for _, l := range log {
		switch l.Event.(type) {
		case event.Joined, event.Left:
			texts = append(texts, strconv.Itoa(l.Beat)+" "+event.Text(l.Event)[:6])
		}
	}
	want := []string{"1 Joined", "3 Joined", "7 Left p"}
	if !slices.Equal(texts, want) {
		t.Errorf("membership events %q, want %q", texts, want)
	}
}

func TestJoinedEventCarriesTheSpawnCell(t *testing.T) {
	m, err := liveRecord().NewMatch()
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	evs := m.Step()
	j, ok := evs[0].Event.(event.Joined)
	pos := m.World().Bots[0].Pos
	if !ok || j.Bot != "p1" || j.X != pos.X || j.Y != pos.Y {
		t.Errorf("first event %+v, bot at %v", evs[0].Event, pos)
	}
}

func TestInvariantsAcceptABotLeavingWhileDead(t *testing.T) {
	log := []event.Logged{
		{Beat: 1, Event: event.Joined{Bot: "a"}}, {Beat: 1, Event: event.Joined{Bot: "b"}},
		{Beat: 2, Event: event.Killed{Killer: "a", Victim: "b"}},
		{Beat: 3, Event: event.Left{Bot: "b"}},
	}
	w := sim.NewWorld(mustMap(t, "####\n#..#\n####\n"), sim.DefaultRules(), 1)
	if err := checkDeaths(w, log); err != nil {
		t.Error(err)
	}
	if err := checkDeaths(w, log[:3]); err == nil {
		t.Error("an unexplained death passed")
	}
}
