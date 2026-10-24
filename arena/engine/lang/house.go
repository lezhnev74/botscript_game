package lang

import (
	"embed"
	"sort"
	"strings"
)

//go:embed bots/*.js
var houseFS embed.FS

// HouseBot returns the source of a house bot (rookie, lighthouse, tracker, forager, hound),
// the sample programs of LANGUAGE.md section 9.
func HouseBot(name string) (string, bool) {
	b, err := houseFS.ReadFile("bots/" + name + ".js")
	if err != nil {
		return "", false
	}
	return string(b), true
}

// HouseBotNames lists the house bots, sorted.
func HouseBotNames() []string {
	entries, _ := houseFS.ReadDir("bots") // the embedded directory always exists
	names := make([]string, len(entries))
	for i, e := range entries {
		names[i] = strings.TrimSuffix(e.Name(), ".js")
	}
	sort.Strings(names)
	return names
}
