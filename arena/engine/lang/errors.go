package lang

import (
	"fmt"
	"sort"
)

// Error is one validation problem, aimed at a non-programmer (or an AI) who
// has to fix the bot: where it is, what is wrong, and what to write instead.
type Error struct {
	Line, Col int
	Msg, Hint string
}

func (e Error) Error() string {
	return fmt.Sprintf("line %d, col %d: %s (%s)", e.Line, e.Col, e.Msg, e.Hint)
}

// sortErrors orders by position, then text, and drops exact duplicates.
func sortErrors(errs []Error) []Error {
	sort.Slice(errs, func(i, j int) bool { return errLess(errs[i], errs[j]) })
	out := errs[:0]
	for i, e := range errs {
		if i == 0 || e != errs[i-1] {
			out = append(out, e)
		}
	}
	return out
}

func errLess(a, b Error) bool {
	if a.Line != b.Line {
		return a.Line < b.Line
	}
	if a.Col != b.Col {
		return a.Col < b.Col
	}
	if a.Msg != b.Msg {
		return a.Msg < b.Msg
	}
	return a.Hint < b.Hint
}
