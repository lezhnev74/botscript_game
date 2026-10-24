package scenario

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/lezhnev/arena/engine/domain/brain"
	"github.com/lezhnev/arena/engine/lang"
)

// entry is one item of a block together with the deeper-indented lines that
// belong to it (an inline program body).
type entry struct {
	head blockLine // trimmed, comment stripped
	body []blockLine
}

// entries groups a block into items: a line at the block's own indentation
// starts one, deeper or blank lines after it are its body.
func (p *parser) entries() []entry {
	lines := p.block()
	base := commonIndent(lines)
	var out []entry
	for _, l := range lines {
		switch {
		case len(out) > 0 && (strings.TrimSpace(l.Text) == "" || indentOf(l.Text) > base):
			out[len(out)-1].body = append(out[len(out)-1].body, l)
		case strings.TrimSpace(stripComment(l.Text)) != "":
			out = append(out, entry{head: blockLine{No: l.No, Text: strings.TrimSpace(stripComment(l.Text))}})
		}
	}
	for i := range out {
		out[i].body = trimBlankTail(out[i].body)
	}
	return out
}

func trimBlankTail(lines []blockLine) []blockLine {
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1].Text) == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

// hasCode reports whether the body has a line besides comments: those are
// harmless under `script` and a named program, and verbatim in `program |`.
func (e entry) hasCode() bool {
	for _, l := range e.body {
		if strings.TrimSpace(stripComment(l.Text)) != "" {
			return true
		}
	}
	return false
}

func indentOf(s string) int { return len(s) - len(strings.TrimLeft(s, " \t")) }

// behaviour reads `script <actions>` or `program <house bot | file.js | |>`.
// Exactly one of the results is meaningful: program is the BOTSCRIPT source
// of a program bot, else script is the action list.
func (p *parser) behaviour(spec string, e entry) (script []brain.Action, program string, err error) {
	kind, args, _ := strings.Cut(strings.TrimSpace(spec), " ")
	args = strings.TrimSpace(args)
	switch kind {
	case "script":
		if e.hasCode() {
			return nil, "", errAt(e.head.No, "`script` takes no indented block")
		}
		script, err = parseActions(args, e.head.No)
		return script, "", err
	case "program":
		program, err = p.program(args, e)
		return nil, program, err
	}
	return nil, "", errAt(e.head.No, "bot behaviour must start with `script` or `program`, got %q", spec)
}

// program resolves the source named after `program` and validates it.
func (p *parser) program(arg string, e entry) (string, error) {
	line := e.head.No
	if arg != "|" && e.hasCode() {
		return "", errAt(line, "unexpected indented lines: only `program |` takes a block")
	}
	switch {
	case arg == "":
		return "", errAt(line, "`program` needs a house bot name, a .js file or `|`")
	case arg == "|":
		return inlineProgram(e)
	case strings.HasSuffix(arg, ".js"):
		return p.fileProgram(arg, line)
	}
	return houseProgram(arg, line)
}

func inlineProgram(e entry) (string, error) {
	if len(e.body) == 0 {
		return "", errAt(e.head.No, "`program |` needs an indented block with the program")
	}
	src := dedentLines(e.body)
	return src, checkProgram(src, e.head.No, e.body[0].No-1, "")
}

func houseProgram(name string, line int) (string, error) {
	src, ok := lang.HouseBot(name)
	if !ok {
		return "", errAt(line, "unknown house bot %q (have %s)", name, strings.Join(lang.HouseBotNames(), ", "))
	}
	return src, nil
}

func (p *parser) fileProgram(name string, line int) (string, error) {
	switch {
	case p.dir == "":
		return "", errAt(line, "program file %s: this scenario was parsed without a directory (use ParseDir)", name)
	case !filepath.IsLocal(name):
		return "", errAt(line, "program file %s must stay inside the scenario directory", name)
	}
	b, err := os.ReadFile(filepath.Join(p.dir, name))
	if err != nil {
		return "", errAt(line, "program file %s: %v", name, err)
	}
	return string(b), checkProgram(string(b), line, line, name)
}

// checkProgram reports the first validation error. An inline body maps the
// program's line n to scenario line offset+n; a file reports at the
// declaration line and names the file.
func checkProgram(src string, declLine, offset int, file string) error {
	_, errs := lang.Validate(src)
	if len(errs) == 0 {
		return nil
	}
	if file != "" {
		return errAt(declLine, "%s: %v", file, errs[0])
	}
	return errAt(offset+errs[0].Line, "program: %v", errs[0])
}

func (p *parser) edits(rest string, line int) error {
	if rest != "" {
		return errAt(line, "`edits:` takes an indented block")
	}
	for _, e := range p.entries() {
		ed, err := p.edit(e)
		if err != nil {
			return err
		}
		p.s.Edits = append(p.s.Edits, ed)
	}
	return nil
}

// edit reads `beat N: <bot> program ...` or `beat N: <bot> reboot K program ...`.
func (p *parser) edit(e entry) (EditSpec, error) {
	line := e.head.No
	beat, id, behaviour, err := splitEdit(e.head)
	if err != nil {
		return EditSpec{}, err
	}
	reboot, behaviour, err := splitReboot(behaviour, line)
	if err != nil {
		return EditSpec{}, err
	}
	_, program, err := p.behaviour(behaviour, e)
	if err == nil && program == "" {
		err = errAt(line, "edits swap programs: write `program ...`, not `script`")
	}
	return EditSpec{Line: line, Beat: beat, Bot: id, Program: program, Reboot: reboot}, err
}

// splitReboot cuts an optional `reboot K` (K >= 1 beats off the map) off the
// front of an edit's behaviour.
func splitReboot(behaviour string, line int) (int, string, error) {
	rest, found := strings.CutPrefix(behaviour, "reboot ")
	if !found {
		return 0, behaviour, nil
	}
	num, behaviour, _ := strings.Cut(strings.TrimSpace(rest), " ")
	n, err := strconv.Atoi(num)
	if err != nil || n < 1 {
		return 0, "", errAt(line, "bad reboot %q (want `reboot K program ...` with K >= 1)", num)
	}
	return n, behaviour, nil
}

// splitEdit cuts `beat N: <bot> <behaviour>` into its three parts.
func splitEdit(head blockLine) (beat int, id, behaviour string, err error) {
	h, spec, ok := strings.Cut(head.Text, ":")
	num, found := strings.CutPrefix(strings.TrimSpace(h), "beat ")
	beat, convErr := strconv.Atoi(strings.TrimSpace(num))
	if !ok || !found || convErr != nil {
		return 0, "", "", errAt(head.No, "bad edit %q (want `beat N: <bot> program ...`)", head.Text)
	}
	id, behaviour, _ = strings.Cut(strings.TrimSpace(spec), " ")
	if len(id) != 1 || id[0] < 'A' || id[0] > 'Z' {
		return 0, "", "", errAt(head.No, "edit needs a bot id A-Z after `beat N:`, got %q", id)
	}
	return beat, id, behaviour, nil
}

func (p *parser) validateEdits() error {
	for _, ed := range p.s.Edits {
		if err := p.validateEdit(ed); err != nil {
			return err
		}
	}
	return nil
}

func (p *parser) validateEdit(ed EditSpec) error {
	switch {
	case !p.isBot(ed.Bot):
		return errAt(ed.Line, "edit: no bot %s on the map", ed.Bot)
	case !p.isProgramBot(ed.Bot):
		return errAt(ed.Line, "edit: bot %s is not a program bot (declare it with `program ...` in bots)", ed.Bot)
	case ed.Beat < 1 || ed.Beat > p.s.Beats:
		return errAt(ed.Line, "edit beat %d out of range 1..%d", ed.Beat, p.s.Beats)
	}
	return nil
}

func (p *parser) isProgramBot(id string) bool {
	for _, b := range p.s.Bots {
		if b.ID == id {
			return b.Program != ""
		}
	}
	return false
}
