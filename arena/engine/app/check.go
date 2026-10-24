package app

import (
	"errors"
	"fmt"

	"github.com/lezhnev/arena/engine/domain/brain"
	"github.com/lezhnev/arena/engine/domain/event"
	"github.com/lezhnev/arena/engine/domain/sim"
	"github.com/lezhnev/arena/engine/lang"
)

// The trial: the checked program is bot A in trialRoom against three house
// bots, with a fixed seed. Everything is a pure function of the source.
const (
	trialBeats = 100
	trialSeed  = 1
	trialBot   = "A"
)

// trialRoom is small so that the checked bot meets enemies within the trial.
const trialRoom = `#########
#.......#
#.......#
#...#...#
#.......#
#.......#
#########
`

var trialFoes = [...]string{"hound", "forager", "tracker"}

// CheckResult is the answer of Check: the validation errors of a bad program,
// or the trial of a good one.
type CheckResult struct {
	Errors []lang.Error
	Trial  *Trial
}

// Actions counts the actions a bot chose, by kind.
type Actions struct{ Scan, Move, Fire, Wait int }

// Trial summarises what a program did in the trial match.
type Trial struct {
	Beats     int // beats played
	DeadBeats int // beats the bot spent dead, waiting to respawn
	Actions   Actions
	Idle      int // beats missed because the code spent its op budget
	NoAction  int // beats that became a wait because loop() returned without an action
	Clicks    int // fire with the gun not loaded
	HitsTaken int
	Hits      int // own bullets that landed
	Kills     int
	Deaths    int
	Coins     int
	Warnings  []string
}

// Check validates a BOTSCRIPT program and, when it is valid, plays a short
// headless trial of it. It touches no live arena and reads no clock.
func Check(src string) (CheckResult, error) {
	if _, errs := lang.Validate(src); len(errs) > 0 {
		return CheckResult{Errors: errs}, nil
	}
	tr, err := playTrial(src)
	if err != nil {
		return CheckResult{}, err
	}
	return CheckResult{Trial: &tr}, nil
}

func trialRecord(src string) (Record, error) {
	mapText, err := PlaceBots(trialRoom, trialSeed, 1+len(trialFoes))
	if err != nil {
		return Record{}, err
	}
	rec := Record{Version: RecordVersion, Rules: sim.DefaultRules(), Map: mapText, Seed: trialSeed, Beats: trialBeats}
	rec.Bots = []BotSpec{{ID: trialBot, Brain: BrainSpec{Kind: KindProgram, Src: src}}}
	for i, name := range trialFoes {
		rec.Bots = append(rec.Bots, BotSpec{ID: string(rune('B' + i)), Brain: houseSpec(name)})
	}
	return rec, nil
}

func playTrial(src string) (Trial, error) {
	rec, err := trialRecord(src)
	if err != nil {
		return Trial{}, err
	}
	m, err := rec.NewMatch()
	if err != nil {
		return Trial{}, err
	}
	defer m.Close()
	sb, ok := m.brains[trialBot].(*lang.ScriptBrain)
	if !ok {
		return Trial{}, errors.New("trial: the program did not become a script brain")
	}
	tally := &tally{sb: sb}
	m.brains[trialBot] = tally
	var log []event.Logged
	for range trialBeats {
		log = append(log, m.Step()...)
		if err := m.Err(); err != nil {
			return Trial{}, fmt.Errorf("trial: %w", err)
		}
	}
	return summarize(log, tally, sb), nil
}

// tally is the trial bot's brain: the program's, counting what it chooses.
type tally struct {
	sb      *lang.ScriptBrain
	actions Actions
	idle    int
}

func (t *tally) Decide(p brain.Perception) brain.Action {
	missed := t.sb.Missed()
	a := t.sb.Decide(p)
	switch {
	case t.sb.Missed() > missed:
		t.idle++
	case a.Kind == brain.Scan:
		t.actions.Scan++
	case a.Kind == brain.Move:
		t.actions.Move++
	case a.Kind == brain.Fire:
		t.actions.Fire++
	default:
		t.actions.Wait++
	}
	return a
}

// Close stops the program's goroutine (Match.Close finds it by this method).
func (t *tally) Close() { t.sb.Close() }

func summarize(log []event.Logged, t *tally, sb *lang.ScriptBrain) Trial {
	s := PerBot(log, trialBeats, []string{trialBot})[trialBot]
	tr := Trial{
		Beats: trialBeats, DeadBeats: trialBeats - s.AliveBeats, Actions: t.actions, Idle: t.idle,
		NoAction: sb.Fallthroughs(), Clicks: s.Clicks, HitsTaken: hitsTaken(log),
		Hits: s.Hits, Kills: s.Kills, Deaths: s.Deaths, Coins: s.Coins,
	}
	tr.Warnings = tr.warnings()
	return tr
}

func hitsTaken(log []event.Logged) int {
	n := 0
	for _, l := range log {
		if h, ok := l.Event.(event.Hit); ok && h.Bot == trialBot {
			n++
		}
	}
	return n
}

// warnings words the obvious problems, in a fixed order.
func (t Trial) warnings() []string {
	all := []struct {
		on  bool
		msg string
	}{
		{t.Idle > 0, fmt.Sprintf("missed %d beats: code is too slow (it used up its op budget before choosing an action)", t.Idle)},
		{t.NoAction > 0, fmt.Sprintf("no action in loop() on %d beats - each became a wait", t.NoAction)},
		{t.Actions.Fire == 0, "never fired in the trial (it may just not have met an enemy)"},
		{t.Actions.Move == 0, "never moved in the trial"},
		{t.Actions.Scan == 0, "never scanned in the trial"},
		{t.Clicks > 0, fmt.Sprintf("fired %d times while the gun was still reloading - each wasted a beat", t.Clicks)},
	}
	out := []string{}
	for _, w := range all {
		if w.on {
			out = append(out, w.msg)
		}
	}
	return out
}
