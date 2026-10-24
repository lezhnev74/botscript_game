package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/lezhnev/arena/engine/adapters/protocol"
)

const (
	scnShot = "../../testdata/scenarios/bullet_hits_stationary_target.scn"
	scnSwap = "../../testdata/scenarios/program_hot_swap_keeps_memory_hp_and_reload.scn"
	recDuel = "../../testdata/long/duel_5x5.record.json"
)

func runReplay(t *testing.T, args ...string) (string, error) {
	t.Helper()
	var out, errOut bytes.Buffer
	err := run(args, &out, &errOut)
	return out.String(), err
}

func readFixture(t *testing.T, path string) []protocol.Message {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var msgs []protocol.Message
	if err := json.Unmarshal(data, &msgs); err != nil {
		t.Fatal(err)
	}
	return msgs
}

func TestPrintsTheHashOfAScenario(t *testing.T) {
	out, err := runReplay(t, scnShot)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out, "hash ") || len(strings.TrimSpace(out)) != len("hash ")+64 {
		t.Errorf("want 'hash <sha256>', got %q", out)
	}
}

func TestRecordAndScenarioReplayToTheirHash(t *testing.T) {
	out, err := runReplay(t, recDuel)
	if err != nil || !strings.HasPrefix(out, "hash ") {
		t.Fatalf("%q %v", out, err)
	}
	hash := strings.TrimSpace(strings.TrimPrefix(out, "hash "))
	if _, err := runReplay(t, "-hash", hash, recDuel); err != nil {
		t.Errorf("matching hash must pass: %v", err)
	}
	if _, err := runReplay(t, "-hash", "00ff", recDuel); err == nil || !strings.Contains(err.Error(), "hash") {
		t.Errorf("wrong hash must fail, got %v", err)
	}
}

func TestExportWritesSnapshotThenOneBeatMessagePerBeat(t *testing.T) {
	dir := t.TempDir()
	if _, err := runReplay(t, "-export", dir, scnShot); err != nil {
		t.Fatal(err)
	}
	msgs := readFixture(t, filepath.Join(dir, "bullet_hits_stationary_target.json"))
	checkSequence(t, msgs)
	if !hasEvent(msgs, "Hit") {
		t.Error("no Hit event in the fixture")
	}
}

func checkSequence(t *testing.T, msgs []protocol.Message) {
	t.Helper()
	if len(msgs) < 2 {
		t.Fatalf("got %d messages", len(msgs))
	}
	if !isSnapshot(msgs[0]) {
		t.Errorf("first message must be the beat 0 snapshot, got %+v", msgs[0])
	}
	for i, m := range msgs[1:] {
		if m.Type != protocol.TypeBeat || m.Frame.Beat != i+1 {
			t.Fatalf("message %d: %s beat %d", i+1, m.Type, m.Frame.Beat)
		}
	}
}

func isSnapshot(m protocol.Message) bool {
	return m.Type == protocol.TypeSnapshot && m.Frame.Beat == 0 && len(m.Events) == 0
}

func hasEvent(msgs []protocol.Message, typ string) bool {
	for _, m := range msgs {
		for _, e := range m.Events {
			if e.Type == typ {
				return true
			}
		}
	}
	return false
}

func TestExportNameOverride(t *testing.T) {
	dir := t.TempDir()
	if _, err := runReplay(t, "-export", dir, "-name", "shot", scnShot); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "shot.json")); err != nil {
		t.Error(err)
	}
}

func TestExportCarriesBoardRosterAndHotSwapBadge(t *testing.T) {
	dir := t.TempDir()
	if _, err := runReplay(t, "-export", dir, scnSwap); err != nil {
		t.Fatal(err)
	}
	msgs := readFixture(t, filepath.Join(dir, "program_hot_swap_keeps_memory_hp_and_reload.json"))
	last := msgs[len(msgs)-1]
	edited := false
	for _, b := range last.Frame.Bots {
		if b.Name == "" {
			t.Errorf("bot %s has no name", b.ID)
		}
		edited = edited || b.EditedBeat > 0
	}
	if !edited {
		t.Errorf("no bot carries the edited beat: %+v", last.Frame.Bots)
	}
}

func TestExportIsByteStable(t *testing.T) {
	a, b := t.TempDir(), t.TempDir()
	for _, d := range []string{a, b} {
		if _, err := runReplay(t, "-export", d, recDuel); err != nil {
			t.Fatal(err)
		}
	}
	x, _ := os.ReadFile(filepath.Join(a, "duel_5x5.json"))
	y, _ := os.ReadFile(filepath.Join(b, "duel_5x5.json"))
	if len(x) == 0 || !bytes.Equal(x, y) {
		t.Error("two exports differ")
	}
}

func TestUsageAndInputErrors(t *testing.T) {
	bad := filepath.Join(t.TempDir(), "bad.scn")
	_ = os.WriteFile(bad, []byte("nonsense"), 0o644)
	badRec := filepath.Join(t.TempDir(), "bad.record.json")
	_ = os.WriteFile(badRec, []byte("{}"), 0o644)
	for name, args := range map[string][]string{
		"no file":           {},
		"two files":         {scnShot, scnShot},
		"missing file":      {"nope.scn"},
		"bad scenario":      {bad},
		"bad record":        {badRec},
		"unknown flag":      {"-wat", scnShot},
		"negative coin":     {"-coin-points", "-1", scnShot},
		"export unwritable": {"-export", filepath.Join(bad, "x"), scnShot},
	} {
		if _, err := runReplay(t, args...); err == nil {
			t.Errorf("%s: want an error", name)
		}
	}
}

func TestCoinPointsFlagSetsTheExportedScore(t *testing.T) {
	const coinScn = "../../testdata/scenarios/coin_pickup_gives_bonus.scn"
	score := func(args ...string) int {
		dir := t.TempDir()
		if _, err := runReplay(t, append([]string{"-export", dir}, append(args, coinScn)...)...); err != nil {
			t.Fatal(err)
		}
		msgs := readFixture(t, filepath.Join(dir, "coin_pickup_gives_bonus.json"))
		for _, e := range msgs[len(msgs)-1].Board {
			if e.Bot == "A" {
				return e.Score
			}
		}
		t.Fatal("no board line for A")
		return 0
	}
	if got := score(); got != 1 {
		t.Errorf("default score %d, want 1", got)
	}
	if got := score("-coin-points", "7"); got != 7 {
		t.Errorf("score %d, want 7", got)
	}
}

// boardHasAfterKick exports the duel record, optionally with bot B kicked at
// beat 150, and reports whether B has a line on the last board.
func boardHasAfterKick(t *testing.T, kick bool) bool {
	t.Helper()
	data, err := os.ReadFile(recDuel)
	if err != nil {
		t.Fatal(err)
	}
	var rec map[string]any
	if err := json.Unmarshal(data, &rec); err != nil {
		t.Fatal(err)
	}
	if kick {
		rec["leaves"] = []map[string]any{{"beat": 150, "bot": "B", "kicked": true}}
	}
	dir := t.TempDir()
	in := filepath.Join(dir, "kick.record.json")
	out, _ := json.Marshal(rec)
	if err := os.WriteFile(in, out, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := runReplay(t, "-export", dir, in); err != nil {
		t.Fatal(err)
	}
	msgs := readFixture(t, filepath.Join(dir, "kick.json"))
	return slices.ContainsFunc(msgs[len(msgs)-1].Board, func(e protocol.LeaderboardEntry) bool { return e.Bot == "B" })
}

func TestExportBoardHasNoLineForAKickedBot(t *testing.T) {
	if !boardHasAfterKick(t, false) {
		t.Fatal("precondition: B has a line without the kick")
	}
	if boardHasAfterKick(t, true) {
		t.Error("kicked bot B still has a board line")
	}
}
