package scenario

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/lezhnev/arena/engine/domain/event"
	"github.com/lezhnev/arena/engine/domain/geo"
	"github.com/lezhnev/arena/engine/domain/sim"
)

// ParseError locates a problem in a .scn file; Line is 1-based, 0 = whole file.
type ParseError struct {
	Line int
	Msg  string
}

func (e *ParseError) Error() string {
	if e.Line == 0 {
		return "scenario: " + e.Msg
	}
	return fmt.Sprintf("line %d: %s", e.Line, e.Msg)
}

func errAt(line int, format string, args ...any) error {
	return &ParseError{Line: line, Msg: fmt.Sprintf(format, args...)}
}

type parser struct {
	lines  []string
	i      int
	s      Scenario
	hasMap bool
	seen   map[string]int // top-level section -> first line
	dir    string         // where `program file.js` is looked up; empty = no files
}

var sections = map[string]func(*parser, string, int) error{
	"name":    (*parser).name,
	"rules":   (*parser).rules,
	"seed":    (*parser).seed,
	"beats":   (*parser).beats,
	"map":     (*parser).mapBlock,
	"bots":    (*parser).bots,
	"start":   (*parser).start,
	"bullets": (*parser).bullets,
	"edits":   (*parser).edits,
	"expect":  (*parser).expect,
}

// Parse reads a .scn file whose programs are inline or house bots.
func Parse(text string) (Scenario, error) { return ParseDir(text, "") }

// ParseDir reads a .scn file; `program file.js` is read from dir.
func ParseDir(text, dir string) (Scenario, error) {
	p := &parser{lines: strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n"), seen: map[string]int{}, dir: dir}
	p.s.Rules = sim.DefaultRules()
	for p.i < len(p.lines) {
		if err := p.next(); err != nil {
			return Scenario{}, err
		}
	}
	if err := p.validate(); err != nil {
		return Scenario{}, err
	}
	return p.s, nil
}

// stripComment removes a '#' comment: one at the line start or after whitespace.
func stripComment(s string) string {
	for i := 0; i < len(s); i++ {
		if s[i] == '#' && (i == 0 || s[i-1] == ' ' || s[i-1] == '\t') {
			return s[:i]
		}
	}
	return s
}

func (p *parser) next() error {
	raw := p.lines[p.i]
	line := p.i + 1
	p.i++
	body := stripComment(raw)
	if strings.TrimSpace(body) == "" {
		return nil
	}
	if body[0] == ' ' || body[0] == '\t' {
		return errAt(line, "unexpected indented line")
	}
	key, rest, ok := strings.Cut(body, ":")
	if !ok {
		return errAt(line, "expected `key: value`, got %q", strings.TrimSpace(body))
	}
	return p.section(strings.TrimSpace(key), strings.TrimSpace(rest), line)
}

func (p *parser) section(key, rest string, line int) error {
	if h, ok := sections[key]; ok {
		if first, dup := p.seen[key]; dup {
			return errAt(line, "section %q repeated (first at line %d)", key, first)
		}
		p.seen[key] = line
		return h(p, rest, line)
	}
	if n, ok := strings.CutPrefix(key, "frame "); ok {
		return p.frame(n, rest, line)
	}
	return errAt(line, "unknown section %q", key)
}

func (p *parser) name(rest string, _ int) error {
	p.s.Name = rest
	return nil
}

func (p *parser) seed(rest string, line int) error {
	v, err := strconv.ParseUint(rest, 10, 64)
	if err != nil {
		return errAt(line, "seed must be a non-negative integer, got %q", rest)
	}
	p.s.Seed = v
	return nil
}

func (p *parser) beats(rest string, line int) error {
	v, err := strconv.Atoi(rest)
	if err != nil || v < 1 {
		return errAt(line, "beats must be a positive integer, got %q", rest)
	}
	p.s.Beats = v
	return nil
}

func (p *parser) rules(rest string, line int) error {
	keys := map[string]bool{}
	for _, tok := range strings.Fields(rest) {
		if tok == "default" {
			continue
		}
		if err := p.rule(tok, line, keys); err != nil {
			return err
		}
	}
	return nil
}

func (p *parser) rule(tok string, line int, keys map[string]bool) error {
	k, v, ok := strings.Cut(tok, "=")
	if !ok {
		return errAt(line, "rule override must be key=value, got %q", tok)
	}
	if keys[k] {
		return errAt(line, "rule %q repeated", k)
	}
	keys[k] = true
	n, err := strconv.Atoi(v)
	if err != nil {
		return errAt(line, "rule %s needs an integer, got %q", k, v)
	}
	if err := p.s.Rules.Set(k, n); err != nil {
		return errAt(line, "%v", err)
	}
	return nil
}

func (p *parser) mapBlock(rest string, line int) error {
	text, err := p.textBlock(rest, line)
	if err != nil {
		return err
	}
	m, err := geo.ParseMap(text)
	if err != nil {
		return mapError(err, line)
	}
	p.s.Map, p.hasMap = m, true
	return nil
}

func mapError(err error, header int) error {
	if pe, ok := err.(*geo.ParseError); ok && pe.Line > 0 {
		return errAt(header+pe.Line, "%v", pe)
	}
	return errAt(header, "%v", err)
}

func (p *parser) frame(num, rest string, line int) error {
	n, err := strconv.Atoi(num)
	if err != nil || n < 0 {
		return errAt(line, "frame needs a beat number, got %q", num)
	}
	key := "frame " + strconv.Itoa(n)
	if first, dup := p.seen[key]; dup {
		return errAt(line, "frame %d repeated (first at line %d)", n, first)
	}
	p.seen[key] = line
	text, err := p.textBlock(rest, line)
	if err != nil {
		return err
	}
	p.s.Frames = append(p.s.Frames, Frame{Line: line, Beat: n, Text: text})
	return nil
}

func (p *parser) validate() error {
	if !p.hasMap {
		return errAt(0, "missing `map:` section")
	}
	if p.s.Beats == 0 {
		return errAt(0, "missing `beats:` section")
	}
	return p.validateRefs()
}

func (p *parser) validateRefs() error {
	if err := p.validateBots(); err != nil {
		return err
	}
	if err := p.validateEdits(); err != nil {
		return err
	}
	if err := p.validateStart(); err != nil {
		return err
	}
	if err := p.validateExpect(); err != nil {
		return err
	}
	return p.validateFrames()
}

func (p *parser) validateStart() error {
	_, err := newMatch(p.s)
	return err
}

func (p *parser) validateBots() error {
	for _, b := range p.s.Bots {
		if _, ok := p.s.Map.BotPos(b.ID[0]); !ok {
			return errAt(b.Line, "bot %s is not on the map", b.ID)
		}
	}
	return nil
}

func (p *parser) isBot(id string) bool {
	if len(id) != 1 {
		return false
	}
	_, ok := p.s.Map.BotPos(id[0])
	return ok
}

func (p *parser) validateExpect() error {
	for _, x := range p.s.Expect {
		if x.Kind != ExpState {
			if err := event.ValidatePattern(x.Words, p.isBot); err != nil {
				return errAt(x.Line, "%v", err)
			}
		}
		lo := firstBeat(x)
		if !x.Final && (x.Beat < lo || x.Beat > p.s.Beats) {
			return errAt(x.Line, "beat %d out of range %d..%d", x.Beat, lo, p.s.Beats)
		}
	}
	return nil
}

func (p *parser) validateFrames() error {
	for _, f := range p.s.Frames {
		if f.Beat > p.s.Beats {
			return errAt(f.Line, "frame %d out of range 0..%d", f.Beat, p.s.Beats)
		}
	}
	return nil
}

// firstBeat is the lowest beat an expectation may name: state can be read
// from the initial world (0), events only exist from beat 1.
func firstBeat(x Expectation) int {
	if x.Kind == ExpState {
		return 0
	}
	return 1
}
