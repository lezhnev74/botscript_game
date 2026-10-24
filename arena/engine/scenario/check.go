package scenario

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/lezhnev/arena/engine/domain/event"
)

func (e Expectation) text() string {
	body := strings.Join(e.Words, " ")
	switch {
	case e.Kind == ExpState && e.Final:
		return "final: " + body
	case e.Kind == ExpState:
		return fmt.Sprintf("at beat %d: %s", e.Beat, body)
	case e.Kind == ExpNo:
		return fmt.Sprintf("beat %d: no %s", e.Beat, body)
	}
	return fmt.Sprintf("beat %d: %s", e.Beat, body)
}

// matches reports whether the pattern is a token prefix of the event text.
func matches(words []string, e event.Event) bool {
	toks := strings.Fields(event.Text(e))
	if len(words) > len(toks) {
		return false
	}
	for i, w := range words {
		if toks[i] != w {
			return false
		}
	}
	return true
}

func formatLog(log []event.Logged) string {
	if len(log) == 0 {
		return "  (no events)"
	}
	lines := make([]string, len(log))
	for i, l := range log {
		lines[i] = fmt.Sprintf("  beat %d: %s", l.Beat, event.Text(l.Event))
	}
	return strings.Join(lines, "\n")
}

// CheckEvents verifies the event expectations. Across beats the order is by
// beat number (file order is irrelevant); within a beat the lines match
// unordered, each consuming a distinct event, so a repeated line needs that
// many events. `no` lines must match nothing in their beat. State
// expectations are ignored here.
func CheckEvents(exps []Expectation, log []event.Logged) error {
	used := make([]bool, len(log))
	missing := make(map[int]bool) // indices into exps
	for _, i := range specificFirst(exps) {
		if exps[i].Kind == ExpEvent && !consume(exps[i], log, used) {
			missing[i] = true
		}
	}
	var fails []error
	for i, x := range exps {
		if err := verdict(x, missing[i], log); err != nil {
			fails = append(fails, err)
		}
	}
	if len(fails) == 0 {
		return nil
	}
	fails = append(fails, fmt.Errorf("actual log:\n%s", formatLog(log)))
	return errors.Join(fails...)
}

// specificFirst orders expectation indices by beat, then longest pattern
// first, so a short prefix pattern cannot steal the event a precise one needs.
func specificFirst(exps []Expectation) []int {
	idx := make([]int, len(exps))
	for i := range idx {
		idx[i] = i
	}
	sort.SliceStable(idx, func(a, b int) bool {
		xa, xb := exps[idx[a]], exps[idx[b]]
		if xa.Beat != xb.Beat {
			return xa.Beat < xb.Beat
		}
		return len(xa.Words) > len(xb.Words)
	})
	return idx
}

func consume(x Expectation, log []event.Logged, used []bool) bool {
	for i, l := range log {
		if !used[i] && l.Beat == x.Beat && matches(x.Words, l.Event) {
			used[i] = true
			return true
		}
	}
	return false
}

func verdict(x Expectation, missing bool, log []event.Logged) error {
	switch x.Kind {
	case ExpEvent:
		if missing {
			return fmt.Errorf("line %d: expected `%s` not found in beat %d", x.Line, x.text(), x.Beat)
		}
	case ExpNo:
		for _, l := range log {
			if l.Beat == x.Beat && matches(x.Words, l.Event) {
				return fmt.Errorf("line %d: `%s` violated by %q", x.Line, x.text(), event.Text(l.Event))
			}
		}
	}
	return nil
}

// GoldenJSONL renders the log as one canonical JSON object per line.
func GoldenJSONL(log []event.Logged) ([]byte, error) {
	var sb strings.Builder
	for _, l := range log {
		b, err := event.JSONLine(l)
		if err != nil {
			return nil, err
		}
		sb.Write(b)
		sb.WriteByte('\n')
	}
	return []byte(sb.String()), nil
}
