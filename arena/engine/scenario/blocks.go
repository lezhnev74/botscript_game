package scenario

import (
	"sort"
	"strconv"
	"strings"

	"github.com/lezhnev/arena/engine/domain/brain"
	"github.com/lezhnev/arena/engine/domain/event"
	"github.com/lezhnev/arena/engine/domain/geo"
)

type blockLine struct {
	No   int
	Text string // raw, with indentation
}

// block consumes the indented (or blank) lines that follow a section header.
func (p *parser) block() []blockLine {
	var out []blockLine
	for p.i < len(p.lines) {
		l := p.lines[p.i]
		if l != "" && l[0] != ' ' && l[0] != '\t' {
			break
		}
		out = append(out, blockLine{No: p.i + 1, Text: l})
		p.i++
	}
	return out
}

// items are the non-blank, comment-stripped, trimmed lines of a block.
func (p *parser) items() []blockLine {
	var out []blockLine
	for _, l := range p.block() {
		if t := strings.TrimSpace(stripComment(l.Text)); t != "" {
			out = append(out, blockLine{No: l.No, Text: t})
		}
	}
	return out
}

// textBlock reads a `|` block verbatim, removing the common indentation.
func (p *parser) textBlock(rest string, line int) (string, error) {
	if t := strings.TrimSpace(stripComment(rest)); t != "|" {
		return "", errAt(line, "expected `|` after the section name, got %q", rest)
	}
	return dedentLines(p.block()), nil
}

// dedentLines joins block lines with their common indentation removed.
func dedentLines(lines []blockLine) string {
	indent := commonIndent(lines)
	var sb strings.Builder
	for _, l := range lines {
		sb.WriteString(strings.TrimRight(dedent(l.Text, indent), " \t"))
		sb.WriteByte('\n')
	}
	return strings.TrimRight(sb.String(), "\n") + "\n"
}

func commonIndent(lines []blockLine) int {
	best := -1
	for _, l := range lines {
		if strings.TrimSpace(l.Text) == "" {
			continue
		}
		n := len(l.Text) - len(strings.TrimLeft(l.Text, " \t"))
		if best < 0 || n < best {
			best = n
		}
	}
	return max(best, 0)
}

func dedent(s string, n int) string {
	if len(s) < n {
		return ""
	}
	return s[n:]
}

func (p *parser) bots(rest string, line int) error {
	if rest != "" {
		return errAt(line, "`bots:` takes an indented block")
	}
	for _, e := range p.entries() {
		if err := p.bot(e); err != nil {
			return err
		}
	}
	sort.Slice(p.s.Bots, func(i, j int) bool { return p.s.Bots[i].ID < p.s.Bots[j].ID })
	return nil
}

func (p *parser) bot(e entry) error {
	id, spec, err := p.botID(e.head)
	if err != nil {
		return err
	}
	b := BotSpec{Line: e.head.No, ID: id}
	if b.Script, b.Program, err = p.behaviour(spec, e); err != nil {
		return err
	}
	p.s.Bots = append(p.s.Bots, b)
	return nil
}

// splitBotLine splits `<id>: <spec>` and checks the id is one letter A-Z.
func splitBotLine(it blockLine) (id, spec string, err error) {
	id, spec, ok := strings.Cut(it.Text, ":")
	id = strings.TrimSpace(id)
	if !ok || len(id) != 1 || id[0] < 'A' || id[0] > 'Z' {
		return "", "", errAt(it.No, "bot id must be one letter A-Z followed by `:`, got %q", it.Text)
	}
	return id, spec, nil
}

func (p *parser) botID(it blockLine) (id, spec string, err error) {
	id, spec, err = splitBotLine(it)
	if err != nil {
		return "", "", err
	}
	for _, b := range p.s.Bots {
		if b.ID == id {
			return "", "", errAt(it.No, "duplicate bot %s", id)
		}
	}
	return id, spec, nil
}

func (p *parser) start(rest string, line int) error {
	if rest != "" {
		return errAt(line, "`start:` takes an indented block")
	}
	for _, it := range p.items() {
		if err := p.startLine(it); err != nil {
			return err
		}
	}
	return nil
}

func (p *parser) startLine(it blockLine) error {
	id, spec, err := splitBotLine(it)
	if err != nil {
		return err
	}
	for _, st := range p.s.Start {
		if st.ID == id {
			return errAt(it.No, "duplicate start for bot %s", id)
		}
	}
	fields, err := parseStartFields(spec, it.No)
	if err != nil {
		return err
	}
	p.s.Start = append(p.s.Start, StartSpec{Line: it.No, ID: id, Fields: fields})
	return nil
}

func (p *parser) bullets(rest string, line int) error {
	if rest != "" {
		return errAt(line, "`bullets:` takes an indented block")
	}
	for _, it := range p.items() {
		b, err := parseBullet(it)
		if err != nil {
			return err
		}
		p.s.Bullets = append(p.s.Bullets, b)
	}
	return nil
}

// parseBullet reads `x,y,DIR` or `x,y,DIR,OWNER`.
func parseBullet(it blockLine) (BulletSpec, error) {
	f := strings.Split(strings.ReplaceAll(it.Text, " ", ""), ",")
	if len(f) < 3 || len(f) > 4 {
		return BulletSpec{}, errAt(it.No, "bullet must be x,y,DIR or x,y,DIR,OWNER, got %q", it.Text)
	}
	x, errX := strconv.Atoi(f[0])
	y, errY := strconv.Atoi(f[1])
	d, okDir := geo.ParseDir(f[2])
	switch {
	case errX != nil || errY != nil:
		return BulletSpec{}, errAt(it.No, "bullet x and y need an integer, got %q", it.Text)
	case !okDir:
		return BulletSpec{}, errAt(it.No, "bad direction %q (want N, E, S or W)", f[2])
	}
	b := BulletSpec{Line: it.No, Pos: geo.Pos{X: x, Y: y}, Dir: d}
	if len(f) == 4 {
		b.Owner = f[3]
	}
	return b, nil
}

func parseStartFields(spec string, line int) ([]StartField, error) {
	var out []StartField
	seen := map[string]bool{}
	for _, tok := range strings.Fields(spec) {
		k, v, ok := strings.Cut(tok, "=")
		n, err := strconv.Atoi(v)
		switch {
		case !ok:
			return nil, errAt(line, "start field must be key=value, got %q", tok)
		case err != nil:
			return nil, errAt(line, "start %s needs an integer, got %q", k, v)
		case n < 0:
			return nil, errAt(line, "start %s must not be negative, got %d", k, n)
		case seen[k]:
			return nil, errAt(line, "start field %q repeated", k)
		}
		seen[k] = true
		out = append(out, StartField{Key: k, Value: n})
	}
	return out, nil
}

func parseActions(list string, line int) ([]brain.Action, error) {
	acts, err := brain.ParseScript(list)
	if err != nil {
		return nil, errAt(line, "script: %v", err)
	}
	return acts, nil
}

func (p *parser) expect(rest string, line int) error {
	if rest != "" {
		return errAt(line, "`expect:` takes an indented block")
	}
	for _, it := range p.items() {
		x, err := parseExpectation(it)
		if err != nil {
			return err
		}
		p.s.Expect = append(p.s.Expect, x)
	}
	return nil
}

func parseExpectation(it blockLine) (Expectation, error) {
	head, body, ok := strings.Cut(it.Text, ":")
	head, words := strings.TrimSpace(head), strings.Fields(body)
	if !ok {
		return Expectation{}, badExpectation(it)
	}
	x := Expectation{Line: it.No, Words: words}
	if head == "final" {
		x.Kind, x.Final = ExpState, true
		return x, validateState(x)
	}
	if n, found := strings.CutPrefix(head, "at beat "); found {
		x.Kind = ExpState
		return withBeat(x, n)
	}
	if n, found := strings.CutPrefix(head, "beat "); found {
		return withBeat(x, n)
	}
	return Expectation{}, badExpectation(it)
}

func badExpectation(it blockLine) error {
	return errAt(it.No, "bad expectation %q (want `beat N: ...`, `at beat N: ...` or `final: ...`)", it.Text)
}

func withBeat(x Expectation, num string) (Expectation, error) {
	n, err := strconv.Atoi(strings.TrimSpace(num))
	if err != nil || n < 0 {
		return x, errAt(x.Line, "bad beat number %q", num)
	}
	x.Beat = n
	if x.Kind == ExpState {
		return x, validateState(x)
	}
	return eventPattern(x)
}

func eventPattern(x Expectation) (Expectation, error) {
	if len(x.Words) > 0 && x.Words[0] == "no" {
		x.Kind, x.Words = ExpNo, x.Words[1:]
	}
	if len(x.Words) == 0 {
		return x, errAt(x.Line, "expected an event pattern like `Moved B S`")
	}
	if !event.Known(x.Words[0]) {
		return x, errAt(x.Line, "unknown event %q", x.Words[0])
	}
	return x, nil
}

func validateState(x Expectation) error {
	if len(x.Words) != 2 {
		return errAt(x.Line, "state expectation must be `<bot>.<field> <value>`")
	}
	if _, err := strconv.Atoi(x.Words[1]); err != nil {
		return errAt(x.Line, "state value must be a number, got %q", x.Words[1])
	}
	return nil
}
