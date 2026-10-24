package app_test

import (
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/lezhnev/arena/engine/app"
)

// The UI offers the values from web/src/render/shapes.ts; they must be the lists the server accepts.
func TestLookListsMatchTheWebRenderer(t *testing.T) {
	src, err := os.ReadFile("../../web/src/render/shapes.ts")
	if err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string][]string{"BODIES": app.Bodies, "EYES": app.Eyes, "MOUTHS": app.Mouths, "HATS": app.Hats} {
		m := regexp.MustCompile(`export const ` + name + `\s*=\s*\[([^\]]*)\]`).FindSubmatch(src)
		if m == nil {
			t.Fatalf("%s not found in shapes.ts", name)
		}
		got := parseTSList(string(m[1]))
		if !slices.Equal(got, want) {
			t.Errorf("%s: shapes.ts has %v, Go has %v", name, got, want)
		}
	}
}

// parseTSList splits the inside of a TS array literal on commas, tolerating line breaks, quotes and a trailing comma.
func parseTSList(body string) []string {
	got := []string{}
	for _, v := range strings.Split(body, ",") {
		if v = strings.Trim(strings.TrimSpace(v), `"'`); v != "" {
			got = append(got, v)
		}
	}
	return got
}

func TestParseTSListToleratesLineBreaks(t *testing.T) {
	if got := parseTSList("\n  \"a\",\n  'b' ,\n"); !slices.Equal(got, []string{"a", "b"}) {
		t.Errorf("%v", got)
	}
}
