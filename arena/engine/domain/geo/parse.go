package geo

import (
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"
)

// BotStart is the start cell of the bot named by an upper-case letter.
type BotStart struct {
	Letter byte
	Pos    Pos
}

// Map is a parsed ASCII map: the grid plus where bots and coins start.
type Map struct {
	Grid  Grid
	Bots  []BotStart // sorted by letter
	Coins []Pos      // row-major order
}

// BotPos returns the start cell of the bot with the given letter.
func (m Map) BotPos(letter byte) (Pos, bool) {
	for _, b := range m.Bots {
		if b.Letter == letter {
			return b.Pos, true
		}
	}
	return Pos{}, false
}

// Render is the inverse of ParseMap.
func (m Map) Render() string {
	marks := make(map[Pos]byte, len(m.Bots)+len(m.Coins))
	for _, c := range m.Coins {
		marks[c] = '$'
	}
	for _, b := range m.Bots {
		marks[b.Pos] = b.Letter
	}
	return m.Grid.Render(marks)
}

// ParseError locates a map problem. Line and Col are 1-based; both are 0 when
// the error concerns the whole map.
type ParseError struct {
	Line, Col int
	Msg       string
}

func (e *ParseError) Error() string {
	if e.Line == 0 {
		return "map: " + e.Msg
	}
	return fmt.Sprintf("map %d:%d: %s", e.Line, e.Col, e.Msg)
}

// ParseMap parses an ASCII map: '#' wall, '.' floor, 'A'..'Z' bot start,
// '$' coin. Blank lines around the map are ignored; all rows must have equal width.
func ParseMap(text string) (Map, error) {
	rows, first := trimBlankLines(strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n"))
	if len(rows) == 0 {
		return Map{}, &ParseError{Msg: "empty map"}
	}
	b := builder{w: utf8.RuneCountInString(rows[0]), first: first, seen: map[byte]Pos{}}
	b.grid = Grid{w: b.w, h: len(rows), walls: make([]bool, b.w*len(rows))}
	for y, row := range rows {
		if err := b.row(y, []rune(row)); err != nil {
			return Map{}, err
		}
	}
	sort.Slice(b.bots, func(i, j int) bool { return b.bots[i].Letter < b.bots[j].Letter })
	return Map{Grid: b.grid, Bots: b.bots, Coins: b.coins}, nil
}

// trimBlankLines drops blank lines at both ends and returns the number of
// lines dropped from the front, so error lines stay relative to the input.
func trimBlankLines(rows []string) ([]string, int) {
	first := 0
	for len(rows) > 0 && strings.TrimSpace(rows[0]) == "" {
		rows = rows[1:]
		first++
	}
	for len(rows) > 0 && strings.TrimSpace(rows[len(rows)-1]) == "" {
		rows = rows[:len(rows)-1]
	}
	return rows, first
}

type builder struct {
	w     int
	first int // blank lines trimmed before the first row
	grid  Grid
	bots  []BotStart
	coins []Pos
	seen  map[byte]Pos
}

func (b *builder) row(y int, row []rune) error {
	line := b.first + y + 1
	if len(row) != b.w {
		return &ParseError{Line: line, Col: min(len(row), b.w) + 1,
			Msg: fmt.Sprintf("ragged row: width %d, want %d", len(row), b.w)}
	}
	for x := range row {
		if err := b.cell(Pos{x, y}, row[x]); err != nil {
			return err
		}
	}
	return nil
}

func (b *builder) cell(p Pos, c rune) error {
	switch {
	case c == '#':
		b.grid.walls[p.Y*b.w+p.X] = true
	case c == '.':
	case c == '$':
		b.coins = append(b.coins, p)
	case c >= 'A' && c <= 'Z':
		return b.bot(p, byte(c))
	default:
		return &ParseError{Line: b.first + p.Y + 1, Col: p.X + 1, Msg: fmt.Sprintf("unknown character %q", c)}
	}
	return nil
}

func (b *builder) bot(p Pos, c byte) error {
	if first, dup := b.seen[c]; dup {
		return &ParseError{Line: b.first + p.Y + 1, Col: p.X + 1,
			Msg: fmt.Sprintf("duplicate bot %q (first at %d:%d)", c, b.first+first.Y+1, first.X+1)}
	}
	b.seen[c] = p
	b.bots = append(b.bots, BotStart{Letter: c, Pos: p})
	return nil
}
