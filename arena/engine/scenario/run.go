package scenario

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/lezhnev/arena/engine/app"
	"github.com/lezhnev/arena/engine/domain/event"
	"github.com/lezhnev/arena/engine/domain/sim"
)

// Result is a finished run: the beat-tagged event log and the world after
// every beat (Worlds[0] is the initial world, Worlds[n] the world after beat n).
type Result struct {
	Log    []event.Logged
	Worlds []sim.World
	Probes []map[string]int // per beat like Worlds: "<bot>.<name>" of the program bots (num_0..num_7, act)
}

// Run plays the scenario's beats through the same Match a Replay uses.
func Run(s Scenario) Result {
	m, _ := newMatch(s) // Parse already validated the start and bullets blocks
	defer m.Close()
	res := Result{Worlds: []sim.World{m.World()}, Probes: []map[string]int{m.Probes()}}
	for beat := 1; beat <= s.Beats; beat++ {
		res.Log = append(res.Log, m.Step()...)
		res.Worlds = append(res.Worlds, m.World())
		res.Probes = append(res.Probes, m.Probes())
	}
	return res
}

// newMatch builds the beat-0 match from the scenario's Record, so the start
// overrides and bullets are applied by one piece of code. A bad override is a
// ParseError at the line of the offending `start:` or `bullets:` entry.
func newMatch(s Scenario) (*app.Match, error) {
	m, err := s.Record().NewMatch()
	var se *app.SpecError
	if errors.As(err, &se) {
		return nil, errAt(specLine(s, se), "%v", se.Err)
	}
	return m, err
}

// specLine maps a SpecError index back to the scenario line it came from.
func specLine(s Scenario, se *app.SpecError) int {
	if se.Section == "bullet" {
		return s.Bullets[se.Index].Line
	}
	n := se.Index
	for _, st := range s.Start {
		if n < len(st.Fields) {
			return st.Line
		}
		n -= len(st.Fields)
	}
	return 0
}

// Verify checks every expectation and frame of the scenario against a run.
func Verify(s Scenario, res Result) error {
	return errors.Join(
		CheckEvents(s.Expect, res.Log),
		CheckStates(s.Expect, res),
		CheckFrames(s.Frames, res.Worlds),
	)
}

// CheckStates verifies the `final:` and `at beat N:` expectations. Besides
// the world fields, a program bot answers num_0..num_7 and act (the
// kind of its latest action: SCAN 0, MOVE 1, FIRE 2, WAIT 3, IDLE 4).
func CheckStates(exps []Expectation, res Result) error {
	var fails []error
	for _, x := range exps {
		if x.Kind != ExpState {
			continue
		}
		if err := checkState(x, res); err != nil {
			fails = append(fails, fmt.Errorf("line %d: `%s`: %w", x.Line, x.text(), err))
		}
	}
	return errors.Join(fails...)
}

func checkState(x Expectation, res Result) error {
	want, err := strconv.Atoi(x.Words[1])
	if err != nil {
		return fmt.Errorf("bad number %q", x.Words[1])
	}
	beat := x.Beat
	if x.Final {
		beat = len(res.Worlds) - 1
	}
	got, err := queryState(res, beat, x.Words[0])
	if err != nil {
		return err
	}
	if got != want {
		return fmt.Errorf("got %d", got)
	}
	return nil
}

func queryState(res Result, beat int, spec string) (int, error) {
	if v, ok := res.Probes[beat][spec]; ok {
		return v, nil
	}
	return res.Worlds[beat].Query(spec)
}

func normalizeFrame(s string) string {
	lines := strings.Split(strings.TrimRight(s, "\n "), "\n")
	for i, l := range lines {
		lines[i] = strings.TrimRight(l, " \t\r")
	}
	return strings.Join(lines, "\n")
}

// CheckFrames compares each frame with the ASCII render of the world after its beat.
func CheckFrames(frames []Frame, worlds []sim.World) error {
	var fails []error
	for _, f := range frames {
		want, got := normalizeFrame(f.Text), normalizeFrame(worlds[f.Beat].Render())
		if want != got {
			fails = append(fails, fmt.Errorf("line %d: frame %d differs\nexpected:\n%s\nactual:\n%s", f.Line, f.Beat, want, got))
		}
	}
	return errors.Join(fails...)
}
