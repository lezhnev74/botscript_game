package app_test

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/lezhnev/arena/engine/app"
	"github.com/lezhnev/arena/engine/domain/event"
)

var goodLook = app.Look{Color: "#12aB9f", Body: "ghost", Eyes: "hearts", Mouth: "tongue", Hat: "bow"}

func TestLookValidationNamesTheBadField(t *testing.T) {
	cases := map[string]func(l *app.Look){
		"color": func(l *app.Look) { l.Color = "red" },
		"body":  func(l *app.Look) { l.Body = "blob" },
		"eyes":  func(l *app.Look) { l.Eyes = "" },
		"mouth": func(l *app.Look) { l.Mouth = "Smile" },
		"hat":   func(l *app.Look) { l.Hat = "top" },
	}
	if err := goodLook.Validate(); err != nil {
		t.Fatalf("good look: %v", err)
	}
	for field, mutate := range cases {
		l := goodLook
		mutate(&l)
		err := l.Validate()
		if !errors.Is(err, app.ErrBadLook) || fieldOf(err) != field {
			t.Errorf("%s: %v", field, err)
		}
	}
}

func fieldOf(err error) string {
	var le *app.LookError
	if !errors.As(err, &le) {
		return ""
	}
	return le.Field
}

func TestLookColorMustBeExactlySixHexDigits(t *testing.T) {
	for _, c := range []string{"#12345", "#1234567", "123456", "#12345g", ""} {
		l := goodLook
		l.Color = c
		if fieldOf(l.Validate()) != "color" {
			t.Errorf("%q accepted", c)
		}
	}
}

func TestLookErrorTextNamesTheField(t *testing.T) {
	l := goodLook
	l.Hat = "top"
	if got := l.Validate().Error(); !strings.Contains(got, "hat") {
		t.Errorf("%q", got)
	}
}

func TestDefaultLookIsValidStableAndVaried(t *testing.T) {
	seen := map[string]map[string]bool{"c": {}, "b": {}, "e": {}, "m": {}, "h": {}}
	for i := range 60 {
		id := "house-" + string(rune('0'+i/10)) + string(rune('0'+i%10))
		l := app.DefaultLook(id)
		if err := l.Validate(); err != nil || l != app.DefaultLook(id) {
			t.Fatalf("%s: %+v %v", id, l, err)
		}
		if !slices.Contains(app.Palette, l.Color) {
			t.Errorf("%s: color %s not in the palette", id, l.Color)
		}
		seen["c"][l.Color], seen["b"][l.Body], seen["e"][l.Eyes], seen["m"][l.Mouth], seen["h"][l.Hat] = true, true, true, true, true
	}
	for part, vals := range seen {
		if len(vals) < 4 {
			t.Errorf("part %s: only %d distinct values over 60 ids", part, len(vals))
		}
	}
}

func TestEveryBotHasADefaultLookInTheRosterAndGetBot(t *testing.T) {
	r := newRig(t, 1)
	token, id := r.join(t, "Ann")
	r.tick(t, 2)
	v, _ := r.a.GetBot(token)
	if v.Look != app.DefaultLook(id) {
		t.Errorf("GetBot look %+v", v.Look)
	}
	for _, p := range r.feed.Last().Roster {
		if p.Look != app.DefaultLook(p.ID) {
			t.Errorf("%s: %+v", p.ID, p.Look)
		}
	}
}

func TestSetProfileRenamesAndRestylesAtTheNextUpdate(t *testing.T) {
	r := newRig(t, 0)
	token, id := r.join(t, "Ann")
	r.tick(t, 2)
	if err := r.a.SetProfile(token, "  Zed  ", goodLook); err != nil {
		t.Fatal(err)
	}
	r.tick(t, 1)
	u := r.feed.Last()
	wantRoster := []app.Player{{ID: id, Name: "Zed", Look: goodLook}}
	if !slices.Equal(u.Roster, wantRoster) {
		t.Errorf("roster %+v", u.Roster)
	}
	if got := boardLine(u.Board, id); got != "Zed" {
		t.Errorf("board name %q", got)
	}
	v, _ := r.a.GetBot(token)
	if got := (app.BotView{ID: id, Name: "Zed", Source: v.Source, Look: goodLook, InArena: true, StayLeft: -1, RebootLeft: -1}); v != got {
		t.Errorf("GetBot %+v", v)
	}
}

func boardLine(board []app.Entry, id string) string {
	for _, e := range board {
		if e.Bot == id {
			return e.Name
		}
	}
	return ""
}

func TestSetProfileTwiceInABeatKeepsTheLatest(t *testing.T) {
	r := newRig(t, 0)
	token, _ := r.join(t, "Ann")
	r.tick(t, 2)
	other := goodLook
	other.Hat = "crown"
	for _, l := range []app.Look{goodLook, other} {
		if err := r.a.SetProfile(token, "Zed", l); err != nil {
			t.Fatalf("not rate limited: %v", err)
		}
	}
	r.tick(t, 1)
	if got := r.feed.Last().Roster[0].Look; got != other {
		t.Errorf("look %+v", got)
	}
}

func TestSetProfileErrors(t *testing.T) {
	r := newRig(t, 1)
	token, _ := r.join(t, "Ann")
	bad := goodLook
	bad.Eyes = "none"
	long := "123456789012345678901234567890"
	for name, c := range map[string]struct {
		token, name string
		look        app.Look
		want        error
	}{
		"empty name":    {token, "   ", goodLook, app.ErrBadName},
		"long name":     {token, long, goodLook, app.ErrBadName},
		"bad look":      {token, "Zed", bad, app.ErrBadLook},
		"unknown token": {"nope", "Zed", goodLook, app.ErrUnauthorized},
		"empty token":   {"", "Zed", goodLook, app.ErrUnauthorized}, // house bots have no token, so none can be addressed
	} {
		if err := r.a.SetProfile(c.token, c.name, c.look); !errors.Is(err, c.want) {
			t.Errorf("%s: %v", name, err)
		}
	}
	if v, _ := r.a.GetBot(token); v.Name != "Ann" {
		t.Errorf("a failed call changed the bot: %+v", v)
	}
}

func TestSetProfileBeforeTheJoinLandsAndLookSurvivesARestart(t *testing.T) {
	r := newRig(t, 0)
	token, id := r.join(t, "Ann")
	if err := r.a.SetProfile(token, "Zed", goodLook); err != nil {
		t.Fatal(err)
	}
	r.tick(t, 2)
	u := r.feed.Last()
	if got := boardLine(u.Board, id); got != "Zed" || u.Roster[0].Look != goodLook {
		t.Errorf("%+v", u)
	}
	r.a.Close()
	r.start(t)
	v, err := r.a.GetBot(token)
	if want := (app.BotView{ID: id, Name: "Zed", Source: v.Source, Look: goodLook, InArena: true, StayLeft: -1, RebootLeft: -1}); v != want || err != nil {
		t.Errorf("after restart: %+v %v", v, err)
	}
}

func TestSavedBotsWithoutAValidLookGetTheDefault(t *testing.T) {
	r := newUnstartedRig(0, room)
	bad := app.Look{Color: "#zzzzzz"}
	r.store.Bots = []app.SavedBot{
		{Token: "t1", ID: "old", Name: "Old", Source: waiter},
		{Token: "t2", ID: "odd", Name: "Odd", Source: waiter, Look: bad},
	}
	r.start(t)
	for _, tok := range []string{"t1", "t2"} {
		v, err := r.a.GetBot(tok)
		if err != nil || v.Look != app.DefaultLook(v.ID) {
			t.Errorf("%s: %+v %v", tok, v, err)
		}
	}
}

func TestProfileChangesDoNotTouchTheReplayHash(t *testing.T) {
	r := newRig(t, 3)
	var live []event.Logged
	token, _ := r.join(t, "Ann")
	for i := range 30 {
		if i%10 == 5 {
			if err := r.a.SetProfile(token, "Ann"+string(rune('a'+i)), goodLook); err != nil {
				t.Fatal(err)
			}
		}
		r.tick(t, 1)
		live = append(live, r.feed.Last().Events...)
	}
	replayed, err := app.Replay(r.a.Record())
	if err != nil {
		t.Fatal(err)
	}
	want, _ := app.HashLog(live)
	if got, _ := app.HashLog(replayed); got != want {
		t.Errorf("hash %s, live %s", got, want)
	}
}
