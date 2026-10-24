package protocol_test

import (
	"encoding/json"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/lezhnev/arena/engine/adapters/protocol"
	"github.com/lezhnev/arena/engine/app"
	"github.com/lezhnev/arena/engine/app/apptest"
	"github.com/lezhnev/arena/engine/domain/event"
	"github.com/lezhnev/arena/engine/domain/geo"
	"github.com/lezhnev/arena/engine/domain/sim"
	"github.com/lezhnev/arena/engine/lang"
)

const room = "#########\n#.......#\n#.......#\n#.......#\n#.......#\n#.......#\n#########\n"

// play runs an arena with house bots and returns the n-th update.
func play(t *testing.T, beats int) app.Update {
	t.Helper()
	feed := &apptest.Feed{}
	a, err := app.NewArena(
		app.Config{Rules: sim.DefaultRules(), Map: room, Seed: 3, MinBots: 3},
		app.Deps{Store: &apptest.MemStore{}, Broadcaster: feed, IDs: &apptest.Seq{}},
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(a.Close)
	for range beats {
		if err := a.Tick(); err != nil {
			t.Fatal(err)
		}
	}
	return feed.Last()
}

func TestFrameDescribesTheWorld(t *testing.T) {
	m := protocol.FromUpdate(play(t, 4), protocol.TypeBeat)
	f := m.Frame
	if got := []int{f.Beat, f.Width, f.Height, len(f.Grid)}; !slices.Equal(got, []int{4, 9, 7, 7}) {
		t.Errorf("frame header %+v", f)
	}
	if got := f.Grid[:2]; !slices.Equal(got, []string{"#########", "#.......#"}) {
		t.Errorf("grid %q", got)
	}
	if len(f.Bots) != 3 {
		t.Fatalf("bots %+v", f.Bots)
	}
	if f.MaxHP != sim.DefaultRules().HP || f.Reload != sim.DefaultRules().Reload {
		t.Errorf("maxHp %d reload %d", f.MaxHP, f.Reload)
	}
	b := f.Bots[0]
	if got := []bool{b.ID != "", b.Name != "", b.House, b.HP == sim.DefaultRules().HP, min(b.X, b.Y) >= 1}; slices.Contains(got, false) {
		t.Errorf("bot %+v", b)
	}
}

// findScan plays beats until some bot scans a non-empty cone.
func findScan(t *testing.T) protocol.Event {
	t.Helper()
	for beats := 1; beats <= 12; beats++ {
		for _, e := range protocol.FromUpdate(play(t, beats), protocol.TypeBeat).Events {
			if e.Type == "Scanned" && len(e.Cells) > 0 {
				return e
			}
		}
	}
	t.Fatal("no scan with cells in 12 beats")
	return protocol.Event{}
}

func TestEventsKeepScanRecords(t *testing.T) {
	scanned := findScan(t)
	if len(scanned.Args) != 5 || scanned.Beat == 0 || len(scanned.Seen) > len(scanned.Cells) {
		t.Fatalf("scan event %+v", scanned)
	}
}

func TestEmptyListsAreArrays(t *testing.T) {
	m := protocol.FromUpdate(app.Update{Events: nil}, protocol.TypeBeat)
	if m.Events == nil || len(m.Events) != 0 || m.Board == nil {
		t.Errorf("empty lists must be [] in JSON, got %+v", m)
	}
	b, err := json.Marshal(m)
	if err != nil || !strings.Contains(string(b), `"events":[]`) || !strings.Contains(string(b), `"board":[]`) {
		t.Errorf("%s %v", b, err)
	}
}

func TestBoardDropsPreyAndSnapshotHasNoEvents(t *testing.T) {
	u := play(t, 3)
	u.Board = []app.Entry{{Bot: "x", Name: "X", Score: 5, Kills: 2, Prey: map[string]int{"y": 2}}}
	m := protocol.FromUpdate(u, protocol.TypeSnapshot)
	if m.Type != "snapshot" || len(m.Events) != 0 {
		t.Errorf("snapshot %+v", m)
	}
	if got := m.Board[0]; got.Bot != "x" || got.Score != 5 || got.Kills != 2 {
		t.Errorf("board %+v", got)
	}
}

func TestMemoryComesFromProbes(t *testing.T) {
	u := play(t, 3)
	id := u.World.Bots[0].ID
	u.Probes = map[string]int{id + ".num_0": 4, id + ".num_1": 5, id + ".num_2": 6, id + ".act": 2, id + ".line": 12, id + ".edited": 7}
	b := protocol.FromUpdate(u, protocol.TypeBeat).Frame.Bots[0]
	if [3]int(b.Num[:3]) != [3]int{4, 5, 6} || b.Act != 2 || b.Line != 12 || b.EditedBeat != 7 {
		t.Errorf("%+v", b)
	}
}

func TestBotStateHasPins(t *testing.T) {
	pins := [lang.PinSize]lang.Place{5: {X: 2, Y: 6, Set: true}}
	st := protocol.FromBotState("p1", app.BotState{InArena: true, Pin: pins})
	if st.Pin[5] == nil || *st.Pin[5] != (protocol.Cell{X: 2, Y: 6}) || st.Pin[4] != nil {
		t.Errorf("pins %+v", st.Pin)
	}
}

func TestBotStateAndErrors(t *testing.T) {
	st := protocol.FromBotState("p1", app.BotState{InArena: true, Act: 1})
	if st.ID != "p1" || !st.InArena || st.Trail == nil {
		t.Errorf("%+v", st)
	}
	ve := protocol.FromValidation(&app.ValidationError{})
	if ve.Errors == nil {
		t.Error("errors must be a list")
	}
}

func TestHitNamesTheShooter(t *testing.T) {
	u := app.Update{Events: []event.Logged{{Beat: 2, Event: event.Hit{Bot: "a", From: geo.W, Shooter: "b"}}}}
	e := protocol.FromUpdate(u, protocol.TypeBeat).Events[0]
	if e.Type != "Hit" || e.By != "b" || e.Beat != 2 || e.Args[1] != "W" {
		t.Errorf("%+v", e)
	}
}

func TestBotCarriesItsCooldown(t *testing.T) {
	u := play(t, 1)
	u.World.Bots[0].Cooldown, u.World.Bots[0].Bonus = 2, 1
	b := protocol.FromUpdate(u, protocol.TypeBeat).Frame.Bots[0]
	if b.Cooldown != 2 || !b.Loaded {
		t.Errorf("bot %+v, want cooldown 2 and loaded (bonus)", b)
	}
}

func TestMessageCarriesTimerAndGameOver(t *testing.T) {
	u := play(t, 1)
	u.Remaining, u.GameOver = 42, true
	m := protocol.FromUpdate(u, protocol.TypeSnapshot)
	if m.Remaining != 42 || !m.GameOver {
		t.Errorf("remaining %d gameOver %v", m.Remaining, m.GameOver)
	}
	b, _ := json.Marshal(m)
	if !strings.Contains(string(b), `"remaining":42`) || !strings.Contains(string(b), `"gameOver":true`) {
		t.Errorf("%s", b)
	}
}

func TestBotsAndFallenCarryTheirLook(t *testing.T) {
	m := protocol.FromUpdate(play(t, 4), protocol.TypeBeat)
	for _, b := range m.Frame.Bots {
		if want := protocol.FromLook(app.DefaultLook(b.ID)); b.Look != want {
			t.Errorf("%s: %+v want %+v", b.ID, b.Look, want)
		}
	}
	u := play(t, 4)
	u.World.Fallen = []sim.Fallen{{Bot: u.World.Bots[0], Wait: 3}}
	f := protocol.FromUpdate(u, protocol.TypeBeat).Frame.Fallen
	if len(f) != 1 || f[0].Look != protocol.FromLook(app.DefaultLook(f[0].ID)) {
		t.Errorf("fallen %+v", f)
	}
}

func TestFrameHasAnEmptyRebootingListWhenNobodyReboots(t *testing.T) {
	if f := protocol.FromUpdate(play(t, 4), protocol.TypeBeat).Frame; f.Rebooting == nil || len(f.Rebooting) != 0 {
		t.Fatalf("want an empty list, got %#v", f.Rebooting)
	}
}

func TestFrameListsTheRebootingBots(t *testing.T) {
	u := play(t, 4)
	b := u.World.Bots[0]
	u.World, _ = u.World.Reboot(b.ID, 10)
	u.World.Rebooting[0].Left = 7
	f := protocol.FromUpdate(u, protocol.TypeBeat).Frame
	want := []protocol.RebootingBot{{Bot: b.ID, Name: f.Rebooting[0].Name, Look: protocol.FromLook(app.DefaultLook(b.ID)), Cell: protocol.Cell{X: b.Pos.X, Y: b.Pos.Y}, HP: b.HP, BeatsLeft: 7, Total: 10}}
	if f.Rebooting[0].Name == "" || !reflect.DeepEqual(f.Rebooting, want) || len(f.Bots) != len(u.World.Bots) {
		t.Errorf("rebooting %+v, want %+v", f.Rebooting, want)
	}
	if raw, _ := json.Marshal(f); !strings.Contains(string(raw), `"rebooting":[{"bot":`) || !strings.Contains(string(raw), `"beatsLeft":7`) || !strings.Contains(string(raw), `"hp":`) {
		t.Errorf("%s", raw)
	}
}

func TestLookRoundTripsAndUsesCamelCaseKeys(t *testing.T) {
	l := app.Look{Color: "#010203", Body: "diamond", Eyes: "visor", Mouth: "teeth", Hat: "crown"}
	if got := protocol.ToLook(protocol.FromLook(l)); got != l {
		t.Errorf("%+v", got)
	}
	b, _ := json.Marshal(protocol.FromLook(l))
	if string(b) != `{"color":"#010203","body":"diamond","eyes":"visor","mouth":"teeth","hat":"crown"}` {
		t.Errorf("%s", b)
	}
}

func TestFromCheckMapsErrors(t *testing.T) {
	bad := protocol.FromCheck(app.CheckResult{Errors: []lang.Error{{Line: 2, Col: 3, Msg: "m", Hint: "h"}}})
	want := protocol.CheckResponse{Errors: []protocol.CheckError{{Line: 2, Col: 3, Message: "m", Hint: "h"}}}
	if !reflect.DeepEqual(bad, want) {
		t.Errorf("errors: %+v", bad)
	}
}

func TestFromCheckMapsTheTrial(t *testing.T) {
	good := protocol.FromCheck(app.CheckResult{Trial: &app.Trial{Beats: 100, Actions: app.Actions{Fire: 4}, Kills: 1, Warnings: []string{"w"}}})
	want := protocol.CheckResponse{Ok: true, Trial: &protocol.CheckTrial{
		Beats: 100, Actions: protocol.CheckActions{Fire: 4}, Kills: 1, Warnings: []string{"w"},
	}}
	if !reflect.DeepEqual(good, want) {
		t.Errorf("trial: %+v", good)
	}
}
