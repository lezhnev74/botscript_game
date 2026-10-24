package store_test

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/lezhnev/arena/engine/adapters/store"
	"github.com/lezhnev/arena/engine/app"
)

func TestNeverWrittenStoreLoadsEmpty(t *testing.T) {
	s := store.New(t.TempDir())
	b, err := s.LoadBoard()
	if err != nil || len(b) != 0 {
		t.Errorf("board %v %v", b, err)
	}
	bots, err := s.LoadBots()
	if err != nil || len(bots) != 0 {
		t.Errorf("bots %v %v", bots, err)
	}
}

func TestSavedDataSurvivesANewStore(t *testing.T) {
	dir := t.TempDir()
	board := []app.Entry{{Bot: "a", Name: "Ann", Score: 4, Kills: 1, Prey: map[string]int{"b": 1}}}
	bots := []app.SavedBot{{Token: "t", ID: "a", Name: "Ann", Source: "function loop() {}", Look: app.Look{Color: "#112233", Body: "ghost", Eyes: "x", Mouth: "o", Hat: "bow"}}}
	s := store.New(dir)
	if err := s.SaveBoard(board); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveBots(bots); err != nil {
		t.Fatal(err)
	}
	again := store.New(dir)
	gotB, err := again.LoadBoard()
	if err != nil || !reflect.DeepEqual(gotB, board) {
		t.Errorf("board %+v %v", gotB, err)
	}
	gotK, err := again.LoadBots()
	if err != nil || !reflect.DeepEqual(gotK, bots) {
		t.Errorf("bots %+v %v", gotK, err)
	}
}

func TestSaveCreatesTheDirAndLeavesNoTempFiles(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "deep", "data")
	s := store.New(dir)
	if err := s.SaveBoard(nil); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveBoard([]app.Entry{{Bot: "a"}}); err != nil {
		t.Fatal(err)
	}
	names := entries(t, dir)
	if len(names) != 1 || names[0] != "board.json" {
		t.Errorf("files %v", names)
	}
}

func TestBotsFileIsPrivate(t *testing.T) {
	dir := t.TempDir()
	if err := store.New(dir).SaveBots([]app.SavedBot{{Token: "secret"}}); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Join(dir, "bots.json"))
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Errorf("mode %v %v", info.Mode(), err)
	}
}

func TestCorruptFileIsAnErrorNotSilentlyEmpty(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "board.json"), []byte("{oops"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.New(dir).LoadBoard(); err == nil || !strings.Contains(err.Error(), "board.json") {
		t.Errorf("err %v", err)
	}
}

func TestSaveFailsWhenTheDirIsAFile(t *testing.T) {
	file := filepath.Join(t.TempDir(), "f")
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := store.New(file).SaveBoard(nil); err == nil {
		t.Error("want an error")
	}
}

func TestFailedWriteKeepsTheOldFile(t *testing.T) {
	dir := t.TempDir()
	s := store.New(dir)
	if err := s.SaveBoard([]app.Entry{{Bot: "old"}}); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(dir, 0o700) //nolint:errcheck
	failed := s.SaveBoard([]app.Entry{{Bot: "new"}}) != nil
	os.Chmod(dir, 0o700) //nolint:errcheck
	got, err := s.LoadBoard()
	want := "old"
	if !failed { // running as root: the directory is writable anyway
		want = "new"
	}
	if err != nil || len(got) != 1 || got[0].Bot != want {
		t.Errorf("board %+v %v (write failed: %v)", got, err, failed)
	}
}

func entries(t *testing.T, dir string) []string {
	t.Helper()
	list, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, e := range list {
		out = append(out, e.Name())
	}
	return out
}

func TestNeverWrittenMapIsEmpty(t *testing.T) {
	if got, err := store.New(t.TempDir()).LoadMap(); err != nil || got != "" {
		t.Fatalf("empty store: %q %v", got, err)
	}
}

func TestMapRoundTripsAsPlainText(t *testing.T) {
	dir := t.TempDir()
	text := "####\n#..#\n####\n"
	if err := store.New(dir).SaveMap(text); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "map.txt"))
	if err != nil || string(raw) != text {
		t.Errorf("map.txt %q %v", raw, err)
	}
	if got, err := store.New(dir).LoadMap(); err != nil || got != text {
		t.Errorf("reload: %q %v", got, err)
	}
}

func TestLoadMapReportsAnUnreadableFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "map.txt"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := store.New(dir).LoadMap(); err == nil {
		t.Error("want an error")
	}
}

func TestLastBoardSurvivesANewStoreAndIsReplaced(t *testing.T) {
	dir := t.TempDir()
	never, _ := store.New(dir).LoadLast()
	second := []app.Entry{{Bot: "b", Name: "Bob", Score: 9}}
	s := store.New(dir)
	for _, e := range [][]app.Entry{{{Bot: "a", Name: "Ann", Score: 4}}, second} {
		if err := s.SaveLast(e); err != nil {
			t.Fatal(err)
		}
	}
	got, err := store.New(dir).LoadLast()
	if !reflect.DeepEqual([]any{len(never), got, err, entries(t, dir)}, []any{0, second, nil, []string{"last.json"}}) {
		t.Errorf("never written %v, then %+v %v, files %v", never, got, err, entries(t, dir))
	}
}

func TestBotsSavedBeforeLooksExistLoadWithAZeroLook(t *testing.T) {
	dir := t.TempDir()
	old := `[{"token":"t","id":"a","name":"Ann","source":"function loop() {}"}]`
	if err := os.WriteFile(filepath.Join(dir, "bots.json"), []byte(old), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := store.New(dir).LoadBots()
	if err != nil || len(got) != 1 || got[0].Look != (app.Look{}) || got[0].Name != "Ann" {
		t.Errorf("%+v %v", got, err)
	}
}

func TestDetachedFieldsSurviveANewStoreAndAreOmittedWhenEmpty(t *testing.T) {
	dir := t.TempDir()
	bots := []app.SavedBot{
		{Token: "t1", ID: "a", Name: "Ann", Source: "s", Detached: true, LeftBy: "time"},
		{Token: "t2", ID: "b", Name: "Bob", Source: "s", StayBeats: 17},
	}
	if err := store.New(dir).SaveBots(bots); err != nil {
		t.Fatal(err)
	}
	got, err := store.New(dir).LoadBots()
	if err != nil || !reflect.DeepEqual(got, bots) {
		t.Errorf("%+v %v", got, err)
	}
	raw, _ := os.ReadFile(filepath.Join(dir, "bots.json"))
	if strings.Count(string(raw), "detached") != 1 || strings.Count(string(raw), "stayBeats") != 1 {
		t.Errorf("zero values must be omitted: %s", raw)
	}
}
