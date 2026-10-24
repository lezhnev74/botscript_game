package scenario

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lezhnev/arena/engine/app"
	"github.com/lezhnev/arena/engine/lang"
)

const progHead = "map: |\n  #####\n  #A.B#\n  #####\nbeats: 4\n"

func TestParseHouseBotProgram(t *testing.T) {
	s, err := Parse(progHead + "bots:\n  A: program rookie\n  B: script wait\n")
	if err != nil {
		t.Fatal(err)
	}
	want, _ := lang.HouseBot("rookie")
	if s.Bots[0].Program != want || s.Bots[1].Program != "" {
		t.Errorf("programs: %q / %q", s.Bots[0].Program, s.Bots[1].Program)
	}
}

func TestParseInlineProgram(t *testing.T) {
	src := progHead + "bots:\n  A: program |   # sweeps\n    function loop() {\n      scan(E);\n    }\n\n  B: script wait\n"
	s, err := Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	if want := "function loop() {\n  scan(E);\n}\n"; s.Bots[0].Program != want {
		t.Errorf("program %q", s.Bots[0].Program)
	}
	if s.Bots[1].ID != "B" || s.Bots[1].Line != 12 {
		t.Errorf("the next bot was swallowed: %+v", s.Bots[1])
	}
}

func TestParseProgramFileRelativeToTheDirectory(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "idle.js"), []byte("function loop() { wait(); }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := ParseDir(progHead+"bots:\n  A: program idle.js\n", dir)
	if err != nil || s.Bots[0].Program != "function loop() { wait(); }\n" {
		t.Fatalf("%v %+v", err, s.Bots)
	}
}

func TestParseEdits(t *testing.T) {
	src := progHead + "bots:\n  A: program rookie\nedits:\n  beat 3: A program tracker\n  beat 2: A program |\n    function loop() { wait(); }\n"
	s, err := Parse(src)
	if err != nil || len(s.Edits) != 2 {
		t.Fatalf("%v %+v", err, s.Edits)
	}
	tracker, _ := lang.HouseBot("tracker")
	if want := (EditSpec{Line: 9, Beat: 3, Bot: "A", Program: tracker}); s.Edits[0] != want {
		t.Errorf("house edit %+v", s.Edits[0])
	}
	if want := (EditSpec{Line: 10, Beat: 2, Bot: "A", Program: "function loop() { wait(); }\n"}); s.Edits[1] != want {
		t.Errorf("inline edit %+v", s.Edits[1])
	}
}

func TestParseRebootEdit(t *testing.T) {
	s, err := Parse(progHead + "bots:\n  A: program rookie\nedits:\n  beat 2: A reboot 3 program |\n    function loop() { wait(); }\n")
	if err != nil || len(s.Edits) != 1 {
		t.Fatalf("%v %+v", err, s.Edits)
	}
	if want := (EditSpec{Line: 9, Beat: 2, Bot: "A", Program: "function loop() { wait(); }\n", Reboot: 3}); s.Edits[0] != want {
		t.Errorf("reboot edit %+v", s.Edits[0])
	}
	if rec := s.Record(); rec.EditLog[0].Reboot != 3 {
		t.Errorf("edit log %+v", rec.EditLog)
	}
}

func TestRecordCarriesProgramSourceAndEdits(t *testing.T) {
	s, err := Parse(progHead + "bots:\n  A: program rookie\n  B: script wait\nedits:\n  beat 2: A program lighthouse\n")
	if err != nil {
		t.Fatal(err)
	}
	rec := s.Record()
	rookie, _ := lang.HouseBot("rookie")
	light, _ := lang.HouseBot("lighthouse")
	if rec.Bots[0].Brain != (app.BrainSpec{Kind: app.KindProgram, Src: rookie}) || rec.Bots[1].Brain.Kind != app.KindScript {
		t.Errorf("bots %+v", rec.Bots)
	}
	if len(rec.EditLog) != 1 || rec.EditLog[0] != (app.Edit{Beat: 2, Bot: "A", Brain: app.BrainSpec{Kind: app.KindProgram, Src: light}}) {
		t.Errorf("edit log %+v", rec.EditLog)
	}
}

func TestProgramErrorsCarryScenarioLines(t *testing.T) {
	inline := "bots:\n  A: program |\n    function loop() {\n      let x = 1;\n    }\n"
	cases := []struct {
		name, src string
		line      int
		msg       string
	}{
		{"inline body line", progHead + inline, 9, "line 2"},
		{"unknown house bot", progHead + "bots:\n  A: program nobody\n", 7, "nobody"},
		{"file needs dir", progHead + "bots:\n  A: program x.js\n", 7, "x.js"},
		{"missing pipe body", progHead + "bots:\n  A: program |\n", 7, "indented"},
		{"program name missing", progHead + "bots:\n  A: program\n", 7, "program"},
		{"script with body", progHead + "bots:\n  A: script wait\n    junk\n", 7, "script"},
		{"body without pipe", progHead + "bots:\n  A: program rookie\n    junk\n", 7, "indented"},
		{"edit inline body line", progHead + "bots:\n  A: program rookie\nedits:\n  beat 2: A program |\n    function loop() {\n      let x;\n    }\n", 11, "line 2"},
		{"edit bad head", progHead + "bots:\n  A: program rookie\nedits:\n  turn 2: A program rookie\n", 9, "beat"},
		{"edit bad beat", progHead + "bots:\n  A: program rookie\nedits:\n  beat x: A program rookie\n", 9, "beat"},
		{"edit beat range", progHead + "bots:\n  A: program rookie\nedits:\n  beat 5: A program rookie\n", 9, "out of range"},
		{"edit beat zero", progHead + "bots:\n  A: program rookie\nedits:\n  beat 0: A program rookie\n", 9, "out of range"},
		{"reboot zero", progHead + "bots:\n  A: program rookie\nedits:\n  beat 2: A reboot 0 program rookie\n", 9, "reboot"},
		{"reboot not a number", progHead + "bots:\n  A: program rookie\nedits:\n  beat 2: A reboot x program rookie\n", 9, "reboot"},
		{"edit script", progHead + "bots:\n  A: program rookie\nedits:\n  beat 2: A script wait\n", 9, "program"},
		{"edit no bot id", progHead + "bots:\n  A: program rookie\nedits:\n  beat 2: program rookie\n", 9, "bot"},
		{"edit unknown bot", progHead + "bots:\n  A: program rookie\nedits:\n  beat 2: Z program rookie\n", 9, "no bot"},
		{"edit script target", progHead + "bots:\n  A: script wait\nedits:\n  beat 2: A program rookie\n", 9, "not a program"},
		{"edit repeated", progHead + "edits:\nedits:\n", 7, "repeated"},
		{"edits with args", progHead + "edits: x\n", 6, "indented block"},
	}
	for _, c := range cases {
		_, err := Parse(c.src)
		var pe *ParseError
		if !errors.As(err, &pe) {
			t.Errorf("%s: err = %v", c.name, err)
			continue
		}
		if pe.Line != c.line || !strings.Contains(pe.Msg, c.msg) {
			t.Errorf("%s: got line %d %q, want line %d containing %q", c.name, pe.Line, pe.Msg, c.line, c.msg)
		}
	}
}

func parseErrIn(t *testing.T, dir, bots string) *ParseError {
	t.Helper()
	_, err := ParseDir(progHead+bots, dir)
	var pe *ParseError
	if !errors.As(err, &pe) {
		t.Fatalf("err = %v", err)
	}
	return pe
}

func TestFileProgramErrorsNameTheFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "bad.js"), []byte("function loop() {\n let x;\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	pe := parseErrIn(t, dir, "bots:\n  A: program bad.js\n")
	if pe.Line != 7 || !strings.Contains(pe.Msg, "bad.js") || !strings.Contains(pe.Msg, "line 2") {
		t.Errorf("err = %v", pe)
	}
}

func TestProgramFileMustStayInsideTheDirectory(t *testing.T) {
	if pe := parseErrIn(t, t.TempDir(), "bots:\n  A: program ../bad.js\n"); pe.Line != 7 || !strings.Contains(pe.Msg, "inside") {
		t.Errorf("err = %v", pe)
	}
}

func TestMissingProgramFileIsReported(t *testing.T) {
	if pe := parseErrIn(t, t.TempDir(), "bots:\n  A: program gone.js\n"); pe.Line != 7 || !strings.Contains(pe.Msg, "gone.js") {
		t.Errorf("err = %v", pe)
	}
}

func TestRunAssertsMemoryAndTheLatestAct(t *testing.T) {
	src := progHead + `bots:
  A: program |
    function loop() { num[0] = num[0] + 1; wait(); }
  B: script wait
edits:
  beat 3: A program |
    function loop() { move(E); }
expect:
  at beat 2: A.num_0 2
  at beat 2: A.act 3
  at beat 3: A.num_0 2
  at beat 3: A.act 1
  final: A.num_1 0
  beat 3: Moved A E
`
	s, err := Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(s, Run(s)); err != nil {
		t.Fatal(err)
	}
	s.Expect[0].Words = []string{"A.num_0", "9"}
	if err := Verify(s, Run(s)); err == nil || !strings.Contains(err.Error(), "got 2") {
		t.Errorf("err = %v", err)
	}
}

func TestCommentsUnderASingleLineBotAreIgnored(t *testing.T) {
	src := progHead + "bots:\n  A: program rookie\n    # note\n  B: script wait\n      # another\n"
	s, err := Parse(src)
	if err != nil || len(s.Bots) != 2 || s.Bots[0].Program == "" {
		t.Fatalf("%v %+v", err, s.Bots)
	}
}
