package main

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/lezhnev/arena/engine/app"
)

type fakeAdmin struct {
	bots   []app.BotInfo
	kicked []string
	err    error
	resets int
	minN   int
	beats  int // last SetTimer
	starts int
}

func (f *fakeAdmin) Bots() []app.BotInfo { return f.bots }
func (f *fakeAdmin) Kick(id string) error {
	f.kicked = append(f.kicked, id)
	return f.err
}

func (f *fakeAdmin) ResetBoard() error {
	f.resets++
	return f.err
}
func (f *fakeAdmin) SetMinBots(n int) error {
	f.minN = n
	return f.err
}

func (f *fakeAdmin) SetTimer(n int) error {
	f.beats = n
	return f.err
}

func (f *fakeAdmin) StartGame() error {
	f.starts++
	return f.err
}

func fixture() *fakeAdmin {
	return &fakeAdmin{bots: []app.BotInfo{
		{ID: "house-1", Name: "WobbleBot", House: true, Score: 4},
		{ID: "p1", Name: "Ann", Score: 12},
		{ID: "p2", Name: "Bob", Score: 0},
		{ID: "p3", Name: "Bob", Score: 7},
	}}
}

func runConsole(f *fakeAdmin, input string) string {
	var out strings.Builder
	console(strings.NewReader(input), &out, f, time.Second)
	return out.String()
}

func TestConsoleListShowsEveryBot(t *testing.T) {
	out := runConsole(fixture(), "list\n")
	for _, want := range []string{"house-1", "house", "p1", "Ann", "human", "12", "p3"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in\n%s", want, out)
		}
	}
	if lines := strings.Count(out, "\n"); lines != 4 {
		t.Errorf("want one line per bot, got %d:\n%s", lines, out)
	}
}

func TestConsoleListMarksDetachedBots(t *testing.T) {
	f := &fakeAdmin{bots: []app.BotInfo{{ID: "p1", Name: "Ann", Detached: true}, {ID: "p2", Name: "Bob"}}}
	lines := strings.Split(strings.TrimSpace(runConsole(f, "list\n")), "\n")
	if len(lines) != 2 || !strings.Contains(lines[0], "detached") || strings.Contains(lines[1], "detached") {
		t.Errorf("%q", lines)
	}
}

func TestConsoleKickByIDAndByName(t *testing.T) {
	f := fixture()
	out := runConsole(f, "kick p2\nkick Ann\n")
	if len(f.kicked) != 2 || f.kicked[0] != "p2" || f.kicked[1] != "p1" {
		t.Errorf("kicked %v", f.kicked)
	}
	if !strings.Contains(out, "kicked p1 (Ann)") {
		t.Errorf("out %q", out)
	}
}

func TestConsoleKickAmbiguousNameListsTheMatches(t *testing.T) {
	f := fixture()
	out := runConsole(f, "kick Bob\n")
	if len(f.kicked) != 0 || !strings.Contains(out, "p2") || !strings.Contains(out, "p3") || !strings.Contains(out, "matches") {
		t.Errorf("kicked %v out %q", f.kicked, out)
	}
}

func TestConsoleKickErrors(t *testing.T) {
	f := fixture()
	out := runConsole(f, "kick\nkick nobody\n")
	if len(f.kicked) != 0 || strings.Count(out, "\n") != 2 {
		t.Errorf("kicked %v out %q", f.kicked, out)
	}
	f.err = app.ErrHouseBot
	if out := runConsole(f, "kick house-1\n"); !strings.Contains(out, "house bots cannot be kicked") {
		t.Errorf("out %q", out)
	}
	f.err = errors.New("boom")
	if out := runConsole(f, "kick p1\n"); !strings.Contains(out, "boom") {
		t.Errorf("out %q", out)
	}
}

func TestConsoleHelpUnknownAndBlankLines(t *testing.T) {
	out := runConsole(fixture(), "\nhelp\nfrobnicate\n")
	if !strings.Contains(out, "kick <id|name>") || !strings.Contains(out, `unknown command "frobnicate"`) {
		t.Errorf("out %q", out)
	}
}

func TestConsoleEndsAtEOF(t *testing.T) {
	if out := runConsole(fixture(), ""); out != "" {
		t.Errorf("out %q", out)
	}
}

func TestConsoleResetBoard(t *testing.T) {
	f := fixture()
	out := runConsole(f, "reset-board\n")
	if f.resets != 1 || !strings.Contains(out, "reset") {
		t.Errorf("resets %d out %q", f.resets, out)
	}
}

func TestConsoleResetBoardWhileStopped(t *testing.T) {
	f := fixture()
	f.err = app.ErrNotRunning
	if out := runConsole(f, "reset-board\n"); !strings.Contains(out, "error: ") || strings.Contains(out, "leaderboard reset") {
		t.Errorf("out %q", out)
	}
}

func TestConsoleMinBots(t *testing.T) {
	f := fixture()
	if out := runConsole(f, "min-bots 3\n"); f.minN != 3 || !strings.Contains(out, "now 3") {
		t.Errorf("minN %d out %q", f.minN, out)
	}
	if out := runConsole(f, "min-bots x\nmin-bots\n"); strings.Count(out, "usage: min-bots <n>") != 2 {
		t.Errorf("out %q", out)
	}
	f.err = app.ErrMinBots
	if out := runConsole(f, "min-bots 99\n"); !strings.Contains(out, "min bots must be") {
		t.Errorf("out %q", out)
	}
}

func TestConsoleTimerConvertsMinutesToBeats(t *testing.T) {
	f := fixture()
	out := runConsole(f, "timer 2\n")
	if f.beats != 120 || !strings.Contains(out, "2") {
		t.Errorf("beats %d out %q", f.beats, out)
	}
	out = runConsole(f, "timer 0\n")
	if f.beats != 0 || !strings.Contains(out, "off") {
		t.Errorf("cancel: beats %d out %q", f.beats, out)
	}
}

func TestConsoleTimerRejectsBadInput(t *testing.T) {
	f := fixture()
	f.beats = 77
	for _, in := range []string{"timer\n", "timer abc\n", "timer -3\n"} {
		if out := runConsole(f, in); !strings.Contains(out, "usage") {
			t.Errorf("%q: %q", in, out)
		}
	}
	if f.beats != 77 {
		t.Errorf("timer set to %d by bad input", f.beats)
	}
	f.err = errors.New("the game is over")
	if out := runConsole(f, "timer 1\n"); !strings.Contains(out, "error: the game is over") {
		t.Errorf("out %q", out)
	}
}

func TestConsoleStart(t *testing.T) {
	f := fixture()
	if out := runConsole(f, "start\n"); f.starts != 1 || !strings.Contains(out, "started") {
		t.Errorf("starts %d out %q", f.starts, out)
	}
	f.err = errors.New("running")
	if out := runConsole(f, "start\n"); !strings.Contains(out, "error: running") {
		t.Errorf("out %q", out)
	}
}
