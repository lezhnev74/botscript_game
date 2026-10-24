package geo

import (
	"errors"
	"reflect"
	"testing"
)

const sample = `
#######
#A.$.B#
#.#...#
#######
`

func TestParseMapSample(t *testing.T) {
	m, err := ParseMap(sample)
	if err != nil {
		t.Fatal(err)
	}
	if m.Grid.Width() != 7 || m.Grid.Height() != 4 {
		t.Fatalf("size = %dx%d", m.Grid.Width(), m.Grid.Height())
	}
	wantBots := []BotStart{{'A', Pos{1, 1}}, {'B', Pos{5, 1}}}
	if !reflect.DeepEqual(m.Bots, wantBots) {
		t.Errorf("Bots = %v want %v", m.Bots, wantBots)
	}
	if !reflect.DeepEqual(m.Coins, []Pos{{3, 1}}) {
		t.Errorf("Coins = %v", m.Coins)
	}
}

func TestParseMapFloorUnderBotsAndCoins(t *testing.T) {
	m := mustParse(t, sample)
	for _, p := range []Pos{{1, 1}, {3, 1}, {5, 1}} {
		if m.Grid.IsWall(p) {
			t.Errorf("%v must be floor", p)
		}
	}
	if !m.Grid.IsWall(Pos{2, 2}) {
		t.Error("(2,2) must be a wall")
	}
}

func TestParseMapBotsSortedByLetter(t *testing.T) {
	m, err := ParseMap("ZA\nB.")
	if err != nil {
		t.Fatal(err)
	}
	want := []BotStart{{'A', Pos{1, 0}}, {'B', Pos{0, 1}}, {'Z', Pos{0, 0}}}
	if !reflect.DeepEqual(m.Bots, want) {
		t.Errorf("Bots = %v want %v", m.Bots, want)
	}
}

func TestParseMapCRLF(t *testing.T) {
	m, err := ParseMap("\r\n##\r\n#A\r\n")
	if err != nil {
		t.Fatal(err)
	}
	if m.Grid.Width() != 2 || m.Grid.Height() != 2 {
		t.Fatalf("size = %dx%d", m.Grid.Width(), m.Grid.Height())
	}
}

func TestMapBotPos(t *testing.T) {
	m := mustParse(t, "A.B")
	if p, ok := m.BotPos('B'); !ok || p != (Pos{2, 0}) {
		t.Errorf("BotPos(B) = %v,%v", p, ok)
	}
	if _, ok := m.BotPos('C'); ok {
		t.Error("BotPos(C) should be absent")
	}
}

func TestParseMapErrors(t *testing.T) {
	tests := []struct {
		name      string
		in        string
		line, col int
		msg       string
	}{
		{"empty", "", 0, 0, "empty map"},
		{"blank only", "\n  \n\n", 0, 0, "empty map"},
		{"ragged short", "###\n##\n###", 2, 3, "ragged row: width 2, want 3"},
		{"ragged long", "##\n####", 2, 3, "ragged row: width 4, want 2"},
		{"blank inside", "##\n\n##", 2, 1, "ragged row: width 0, want 2"},
		{"unknown char", "##\n#x", 2, 2, "unknown character 'x'"},
		{"leading blank", "\n\n##\n#x", 4, 2, "unknown character 'x'"},
		{"non-ascii", "##\n#é", 2, 2, "unknown character 'é'"},
		{"space", "# #", 1, 2, "unknown character ' '"},
		{"duplicate bot", "A.\n.A", 2, 2, "duplicate bot 'A' (first at 1:1)"},
		{"duplicate bot leading blank", "\n\nA.\n.A", 4, 2, "duplicate bot 'A' (first at 3:1)"},
		{"unknown after ragged check", "##\n#?#", 2, 3, "ragged row: width 3, want 2"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseMap(tt.in)
			var pe *ParseError
			if !errors.As(err, &pe) {
				t.Fatalf("err = %v, want *ParseError", err)
			}
			if pe.Line != tt.line || pe.Col != tt.col || pe.Msg != tt.msg {
				t.Errorf("got %d:%d %q want %d:%d %q", pe.Line, pe.Col, pe.Msg, tt.line, tt.col, tt.msg)
			}
		})
	}
}

func TestParseErrorString(t *testing.T) {
	if got := (&ParseError{Line: 2, Col: 3, Msg: "boom"}).Error(); got != "map 2:3: boom" {
		t.Errorf("got %q", got)
	}
	if got := (&ParseError{Msg: "empty map"}).Error(); got != "map: empty map" {
		t.Errorf("got %q", got)
	}
}

func TestRenderRoundTrip(t *testing.T) {
	inputs := []string{
		"#######\n#A.$.B#\n#.#...#\n#######\n",
		"A\n",
		"$$\n..\n",
	}
	for _, in := range inputs {
		m, err := ParseMap(in)
		if err != nil {
			t.Fatal(err)
		}
		if got := m.Render(); got != in {
			t.Errorf("Render = %q want %q", got, in)
		}
	}
}

func TestGridRenderMarks(t *testing.T) {
	m := mustParse(t, "#..\n...")
	got := m.Grid.Render(map[Pos]byte{{1, 0}: '*', {2, 1}: 'B'})
	if want := "#*.\n..B\n"; got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

func mustParse(t *testing.T, text string) Map {
	t.Helper()
	m, err := ParseMap(text)
	if err != nil {
		t.Fatalf("ParseMap(%q): %v", text, err)
	}
	return m
}
