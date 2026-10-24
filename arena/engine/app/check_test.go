package app_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/lezhnev/arena/engine/app"
	"github.com/lezhnev/arena/engine/domain/geo"
	"github.com/lezhnev/arena/engine/lang"
)

func check(t *testing.T, src string) app.CheckResult {
	t.Helper()
	res, err := app.Check(src)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func hasWarning(tr *app.Trial, part string) bool {
	for _, w := range tr.Warnings {
		if strings.Contains(w, part) {
			return true
		}
	}
	return false
}

func TestCheckRejectsABrokenProgramWithItsErrors(t *testing.T) {
	res := check(t, "function loop() { jump(N); }")
	if res.Trial != nil || len(res.Errors) == 0 {
		t.Fatalf("want errors and no trial, got %+v", res)
	}
	if e := res.Errors[0]; e.Line != 1 || e.Msg == "" || e.Hint == "" {
		t.Errorf("error lacks line/message/hint: %+v", e)
	}
}

func TestCheckPlaysATrialOfAGoodProgram(t *testing.T) {
	src, _ := lang.HouseBot("rookie")
	res := check(t, src)
	if len(res.Errors) != 0 || res.Trial == nil {
		t.Fatalf("want a trial, got %+v", res)
	}
	tr := res.Trial
	acts := tr.Actions
	if tr.Beats != 100 {
		t.Errorf("Beats = %d, want 100", tr.Beats)
	}
	if acts.Scan+acts.Move+acts.Fire+acts.Wait+tr.Idle != tr.Beats-tr.DeadBeats {
		t.Errorf("actions %+v + idle %d do not add up to the %d alive beats", acts, tr.Idle, tr.Beats-tr.DeadBeats)
	}
	if acts.Scan == 0 {
		t.Errorf("rookie should scan: %+v", acts)
	}
}

func TestTrialMeetsEnemiesForTheStationaryHouseBots(t *testing.T) {
	for _, name := range []string{"rookie", "lighthouse"} {
		src, _ := lang.HouseBot(name)
		tr := check(t, src).Trial
		if tr.Actions.Fire == 0 || tr.Hits == 0 || hasWarning(tr, "never fired") {
			t.Errorf("%s did not fight in the trial: %+v", name, tr)
		}
	}
}

func TestCheckIsDeterministic(t *testing.T) {
	src, _ := lang.HouseBot("tracker")
	if a, b := check(t, src), check(t, src); !reflect.DeepEqual(a, b) {
		t.Errorf("same program, different answers:\n%+v\n%+v", a.Trial, b.Trial)
	}
}

func TestCheckWarnsAboutObviousProblems(t *testing.T) {
	cases := []struct{ name, src, warning string }{
		{"never fired", "function loop() { move(N); scan(E); }", "never fired"},
		{"never moved", "function loop() { fire(N); scan(E); }", "never moved"},
		{"never scanned", "function loop() { fire(N); move(E); }", "never scanned"},
		{"too slow", "function loop() { while (1) { num[0]++; } }", "missed"},
		{"no action", "function loop() { num[0]++; }", "no action in loop()"},
		{"empty gun", "function loop() { fire(N); }", "reloading"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			tr := check(t, c.src).Trial
			if tr == nil || !hasWarning(tr, c.warning) {
				t.Errorf("want a warning with %q, got %+v", c.warning, tr)
			}
		})
	}
}

func TestCheckHasNoWarningsForAHealthyBot(t *testing.T) {
	src := "function loop() { scan(E); move(pick(N, E, S, W)); scan(W); wait(); wait(); wait(); fire(pick(N, E, S, W)); move(S); }"
	if tr := check(t, src).Trial; len(tr.Warnings) != 0 {
		t.Errorf("unexpected warnings %v", tr.Warnings)
	}
}

func TestCheckCountsWhatTheBotDid(t *testing.T) {
	tr := check(t, "function loop() { fire(N); }").Trial
	if tr.Actions.Fire == 0 || tr.Actions.Move != 0 || tr.Clicks == 0 {
		t.Errorf("a bot that only fires: %+v", tr)
	}
}

func TestCheckLeavesALiveArenaAlone(t *testing.T) {
	r := newRig(t, 2)
	r.tick(t, 3)
	before := r.a.World()
	src, _ := lang.HouseBot("rookie")
	check(t, src)
	if !reflect.DeepEqual(before, r.a.World()) {
		t.Error("Check changed a live arena")
	}
}

func TestDefaultRoomIsAnEmpty19x13Room(t *testing.T) {
	m, err := geo.ParseMap(app.DefaultRoom)
	if err != nil {
		t.Fatal(err)
	}
	if m.Grid.Width() != 19 || m.Grid.Height() != 13 || len(m.Bots) != 0 {
		t.Errorf("room is %dx%d with %d bots", m.Grid.Width(), m.Grid.Height(), len(m.Bots))
	}
}
