package scenario

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/lezhnev/arena/engine/domain/brain"
	"github.com/lezhnev/arena/engine/domain/geo"
	"github.com/lezhnev/arena/engine/domain/sim"
)

const full = `# a comment
name: target that moves wins
rules: reload=4 bullet_speed=2   # overrides
seed: 7
map: |
  #######
  #A...B#
  #######
bots:
  A: script fire E, move S, wait   # trailing comment
  B: script
beats: 2
expect:
  beat 1: Fired A E
  beat 2: no Hit
  at beat 1: B.x 5
  final: B.hp 3

frame 1: |      # snapshot
  #######
  #A...B#
  #######
`

func parseFull(t *testing.T) Scenario {
	t.Helper()
	s, err := Parse(full)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestParseScalarsRulesMap(t *testing.T) {
	s := parseFull(t)
	rules := sim.DefaultRules()
	rules.Reload, rules.BulletSpeed = 4, 2
	if got := []any{s.Name, s.Seed, s.Beats, s.Rules}; !reflect.DeepEqual(got, []any{"target that moves wins", uint64(7), 2, rules}) {
		t.Errorf("got %+v", got)
	}
	if len(s.Map.Bots) != 2 || s.Map.Grid.Width() != 7 {
		t.Errorf("map: %+v", s.Map)
	}
}

func TestParseBots(t *testing.T) {
	s := parseFull(t)
	want := []brain.Action{{Kind: brain.Fire, Dir: geo.E}, {Kind: brain.Move, Dir: geo.S}, {Kind: brain.Wait}}
	if len(s.Bots) != 2 || s.Bots[0].ID != "A" || len(s.Bots[1].Script) != 0 {
		t.Fatalf("bots: %+v", s.Bots)
	}
	for i, w := range want {
		if s.Bots[0].Script[i] != w {
			t.Errorf("action %d = %v, want %v", i, s.Bots[0].Script[i], w)
		}
	}
}

func TestParseExpect(t *testing.T) {
	want := []Expectation{
		{Line: 14, Kind: ExpEvent, Beat: 1, Words: []string{"Fired", "A", "E"}},
		{Line: 15, Kind: ExpNo, Beat: 2, Words: []string{"Hit"}},
		{Line: 16, Kind: ExpState, Beat: 1, Words: []string{"B.x", "5"}},
		{Line: 17, Kind: ExpState, Final: true, Words: []string{"B.hp", "3"}},
	}
	if got := parseFull(t).Expect; !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v\nwant %+v", got, want)
	}
}

func TestParseFrames(t *testing.T) {
	f := parseFull(t).Frames
	if len(f) != 1 || f[0].Beat != 1 || f[0].Line != 19 || f[0].Text != "#######\n#A...B#\n#######\n" {
		t.Errorf("frames: %+v", f)
	}
}

func TestParseDefaultRules(t *testing.T) {
	s, err := Parse("rules: default\nmap: |\n  #A#\nbeats: 1\n")
	if err != nil || s.Rules.Reload != 3 {
		t.Fatalf("%v %+v", err, s.Rules)
	}
}

// The optional `start:` block overrides a bot's initial state (hp, bonus,
// cooldown) as `<bot>: key=value ...`; omitted fields keep their defaults and
// entries keep file order.
func TestParseStart(t *testing.T) {
	src := "map: |\n  #AB#\nstart:\n  B: hp=2   # wounded\n  A: bonus=2 cooldown=3\nbeats: 1\n"
	s, err := Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	want := []StartSpec{
		{Line: 4, ID: "B", Fields: []StartField{{"hp", 2}}},
		{Line: 5, ID: "A", Fields: []StartField{{"bonus", 2}, {"cooldown", 3}}},
	}
	if !reflect.DeepEqual(s.Start, want) {
		t.Errorf("got %+v\nwant %+v", s.Start, want)
	}
}

// The optional `bullets:` block places bullets in flight at the start:
// `x,y,DIR` or `x,y,DIR,OWNER`, one per line, file order kept.
func TestParseBullets(t *testing.T) {
	src := "map: |\n  #A...#\nbullets:\n  3,0,W   # comment\n  2,0,E,A\nbeats: 1\n"
	s, err := Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	want := []BulletSpec{
		{Line: 4, Pos: geo.Pos{X: 3, Y: 0}, Dir: geo.W},
		{Line: 5, Pos: geo.Pos{X: 2, Y: 0}, Dir: geo.E, Owner: "A"},
	}
	if !reflect.DeepEqual(s.Bullets, want) {
		t.Errorf("got %+v\nwant %+v", s.Bullets, want)
	}
}

func TestRunStartsWithPlacedBullets(t *testing.T) {
	s, err := Parse("map: |\n  #####\n  #A..#\n  #####\nbullets:\n  2,1,W\nbeats: 1\n")
	if err != nil {
		t.Fatal(err)
	}
	if got := Run(s).Worlds[0].Render(); got != "#####\n#A*.#\n#####\n" {
		t.Errorf("initial render %q", got)
	}
}

const okTail = "map: |\n  #A.#\nbeats: 2\n"

func TestParseErrors(t *testing.T) {
	cases := []struct {
		name, src string
		line      int
		msg       string
	}{
		{"no colon", "oops\n", 1, "key: value"},
		{"indented", "  x: 1\n", 1, "unexpected indented"},
		{"unknown key", "name: x\nfoo: 1\n", 2, "unknown section"},
		{"bad seed", "seed: abc\n", 1, "seed"},
		{"bad beats", "beats: 0\n", 1, "beats"},
		{"bad rule", "rules: reload\n", 1, "key=value"},
		{"unknown rule", "rules: nope=1\n", 1, "unknown rule"},
		{"bad rule value", "rules: reload=x\n", 1, "reload"},
		{"map needs pipe", "map: abc\n", 1, "|"},
		{"map error line", "name: x\nmap: |\n  #A#\n  #A.#\n", 4, "ragged"},
		{"map whole error", "map: |\n  \nbeats: 1\n", 1, "empty map"},
		{"bad bot id", "bots:\n  AB: script\n", 2, "bot id"},
		{"no bot colon", "bots:\n  A script\n", 2, "bot id"},
		{"bot kind", "bots:\n  A: dance\n", 2, "script"},
		{"bad action", "bots:\n  A: script fire E, jump\n", 2, "jump"},
		{"dup bot", "bots:\n  A: script\n  A: script\n", 3, "duplicate"},
		{"expect no colon", "expect:\n  Moved\n", 2, "expectation"},
		{"expect head", "expect:\n  turn 1: Moved\n", 2, "expectation"},
		{"expect beat num", "expect:\n  beat x: Moved\n", 2, "beat"},
		{"expect empty", "expect:\n  beat 1:\n", 2, "event"},
		{"expect no empty", "expect:\n  beat 1: no\n", 2, "event"},
		{"expect unknown event", "expect:\n  beat 1: no Moovd B\n", 2, "unknown event"},
		{"state words", "expect:\n  final: B.hp\n", 2, "<bot>.<field> <value>"},
		{"state number", "expect:\n  final: B.hp q\n", 2, "number"},
		{"frame header", "frame x: |\n", 1, "frame"},
		{"frame pipe", "frame 1: abc\n", 1, "|"},
		{"missing map", "beats: 1\n", 0, "map"},
		{"missing beats", "map: |\n  #A#\n", 0, "beats"},
		{"unknown bot", okTail + "bots:\n  Z: script\n", 5, "not on the map"},
		{"event beat zero", okTail + "expect:\n  beat 0: Moved\n", 5, "out of range"},
		{"event beat range", okTail + "expect:\n  beat 3: Moved\n", 5, "out of range"},
		{"state beat range", okTail + "expect:\n  at beat 3: A.x 1\n", 5, "out of range"},
		{"no unknown bot", okTail + "expect:\n  beat 1: no Moved Z\n", 5, "no bot"},
		{"no bad dir", okTail + "expect:\n  beat 1: no Hit A X\n", 5, "direction"},
		{"too many args", okTail + "expect:\n  beat 1: Moved A S N\n", 5, "too many"},
		{"dup map", okTail + "map: |\n  #A#\n", 4, "repeated"},
		{"dup beats", okTail + "beats: 1\n", 4, "repeated"},
		{"dup name", "name: a\nname: b\n", 2, "repeated"},
		{"dup bots", "bots:\n  A: script\nbots:\n  B: script\n", 3, "repeated"},
		{"dup expect", "expect:\n  beat 1: Moved\nexpect:\n", 3, "repeated"},
		{"dup frame", okTail + "frame 1: |\n  #A.#\nframe 1: |\n  #A.#\n", 6, "repeated"},
		{"dup rule key", "rules: reload=3 reload=4\n", 1, "repeated"},
		{"start with args", okTail + "start: x\n", 4, "indented block"},
		{"start bad id", okTail + "start:\n  AB: hp=1\n", 5, "bot id"},
		{"start no colon", okTail + "start:\n  A hp=1\n", 5, "bot id"},
		{"start dup bot", okTail + "start:\n  A: hp=1\n  A: hp=2\n", 6, "duplicate"},
		{"start unknown bot", okTail + "start:\n  Z: hp=1\n", 5, "no bot"},
		{"start no key", okTail + "start:\n  A: hp\n", 5, "key=value"},
		{"start bad value", okTail + "start:\n  A: hp=x\n", 5, "integer"},
		{"start negative", okTail + "start:\n  A: hp=-1\n", 5, "negative"},
		{"start dup key", okTail + "start:\n  A: hp=1 hp=2\n", 5, "repeated"},
		{"start unknown field", okTail + "start:\n  A: x=1\n", 5, "cannot set"},
		{"start bonus over cap", okTail + "start:\n  A: bonus=3\n", 5, "bonus_max"},
		{"dup start", okTail + "start:\n  A: hp=1\nstart:\n", 6, "repeated"},
		{"bullets with args", okTail + "bullets: x\n", 4, "indented block"},
		{"bullet fields", okTail + "bullets:\n  1,1\n", 5, "x,y,DIR"},
		{"bullet too many", okTail + "bullets:\n  1,1,E,A,B\n", 5, "x,y,DIR"},
		{"bullet bad x", okTail + "bullets:\n  q,1,E\n", 5, "integer"},
		{"bullet bad y", okTail + "bullets:\n  1,q,E\n", 5, "integer"},
		{"bullet bad dir", okTail + "bullets:\n  1,1,X\n", 5, "direction"},
		{"bullet in wall", okTail + "bullets:\n  0,0,E\n", 5, "floor"},
		{"bullet outside", okTail + "bullets:\n  9,9,E\n", 5, "floor"},
		{"bullet unknown owner", okTail + "bullets:\n  1,0,E,Z\n", 5, "no bot"},
		{"bullet on a bot", okTail + "bullets:\n  1,0,E\n", 5, "bot"},
		{"dup bullets", okTail + "bullets:\nbullets:\n", 5, "repeated"},
		{"frame range", okTail + "frame 5: |\n  #A.#\n", 4, "out of range"},
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

func TestParseErrorText(t *testing.T) {
	if got := (&ParseError{Line: 3, Msg: "x"}).Error(); got != "line 3: x" {
		t.Error(got)
	}
	if got := (&ParseError{Msg: "x"}).Error(); got != "scenario: x" {
		t.Error(got)
	}
}
