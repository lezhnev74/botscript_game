package httpapi_test

import (
	"net/http/httptest"

	"github.com/lezhnev/arena/engine/app"
	"github.com/lezhnev/arena/engine/app/apptest"
	"github.com/lezhnev/arena/engine/domain/sim"
	"strings"
	"testing"
	"time"

	"github.com/lezhnev/arena/engine/adapters/httpapi"
	"github.com/lezhnev/arena/engine/adapters/protocol"
)

// botBody reads GET /api/bot.
func (r *rig) botBody(t *testing.T, token string) protocol.BotBody {
	t.Helper()
	_, body, _ := r.call(t, "GET", "/api/bot", token, "")
	return decode[protocol.BotBody](t, body)
}

func (r *rig) post(t *testing.T, path, token string, want int) string {
	t.Helper()
	code, body, _ := r.call(t, "POST", path, token, "")
	if code != want {
		t.Fatalf("POST %s: %d %s, want %d", path, code, body, want)
	}
	return body
}

func TestGetBotReportsTheArenaStateAndBeatLength(t *testing.T) {
	r := newRig(t, httpapi.Options{Beat: 1500 * time.Millisecond})
	j := r.join(t, "Ann")
	r.tick(t, 5)
	if b := r.botBody(t, j.Token); b.Arena != (protocol.BotArena{In: true, StayLeftBeats: -1}) || b.BeatMs != 1500 {
		t.Errorf("in: %+v", b)
	}
	r.post(t, "/api/bot/leave", j.Token, 204)
	r.tick(t, 1)
	b := r.botBody(t, j.Token)
	if b.Arena != (protocol.BotArena{LeftBy: "player", StayLeftBeats: -1}) || !strings.Contains(b.Source, "loop") {
		t.Errorf("detached: %+v", b)
	}
}

func TestReturnRouteIsRateLimitedThenBringsTheBotBack(t *testing.T) {
	r := newRig(t, httpapi.Options{})
	j := r.join(t, "Ann")
	r.tick(t, 5)
	r.post(t, "/api/bot/leave", j.Token, 204)
	r.tick(t, 1)
	if e := decode[protocol.ErrorBody](t, r.post(t, "/api/bot/return", j.Token, 429)); e.RetryBeats < 1 {
		t.Errorf("retryBeats %+v", e)
	}
	r.tick(t, 3)
	r.post(t, "/api/bot/return", j.Token, 204)
	r.tick(t, 1)
	if !r.botBody(t, j.Token).Arena.In {
		t.Error("not back")
	}
}

func TestLeaveAndReturnNeedAValidTokenAndPost(t *testing.T) {
	r := newRig(t, httpapi.Options{})
	for _, path := range []string{"/api/bot/leave", "/api/bot/return"} {
		if code, _, _ := r.call(t, "POST", path, "", ""); code != 401 {
			t.Errorf("%s no token: %d", path, code)
		}
		if code, _, _ := r.call(t, "POST", path, "nope", ""); code != 401 {
			t.Errorf("%s bad token: %d", path, code)
		}
		code, _, h := r.call(t, "GET", path, "", "")
		if code != 405 || h.Get("Allow") != "POST" {
			t.Errorf("%s GET: %d %q", path, code, h.Get("Allow"))
		}
	}
}

func TestReturnToAFullArenaIs409(t *testing.T) {
	r := newMaxPlayersRig(t, 1)
	a := r.join(t, "Ann")
	r.tick(t, 1)
	r.call(t, "POST", "/api/bot/leave", a.Token, "")
	r.tick(t, 1)
	r.join(t, "Bob")
	r.tick(t, 4)
	code, body, _ := r.call(t, "POST", "/api/bot/return", a.Token, "")
	if code != 409 || !strings.Contains(body, "full") {
		t.Errorf("%d %s", code, body)
	}
}

func TestAdminStatusMarksDetachedBots(t *testing.T) {
	r := newAdminRig(t, adminPass)
	s := r.login(t)
	j := r.join(t, "Ann")
	r.tick(t, 2)
	r.call(t, "POST", "/api/bot/leave", j.Token, "")
	r.tick(t, 1)
	found := false
	for _, b := range r.status(t, s).Bots {
		if b.ID == j.BotID {
			found = b.Detached
		} else if b.Detached {
			t.Errorf("house bot detached: %+v", b)
		}
	}
	if !found {
		t.Errorf("%+v", r.status(t, s).Bots)
	}
	if code, _, _ := r.do(t, "POST", "/api/admin/kick", s, `{"id":"`+j.BotID+`"}`); code != 204 {
		t.Errorf("kick: %d", code)
	}
	if code, _, _ := r.call(t, "GET", "/api/bot", j.Token, ""); code != 401 {
		t.Errorf("token after kick: %d", code)
	}
}

func newMaxPlayersRig(t *testing.T, n int) *rig {
	t.Helper()
	a, err := app.NewArena(
		app.Config{Rules: sim.DefaultRules(), Map: room, Seed: 3, MaxPlayers: n},
		app.Deps{Store: &apptest.MemStore{}, Broadcaster: &apptest.Feed{}, IDs: &apptest.Seq{}},
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(a.Close)
	srv := httptest.NewServer(httpapi.New(a, httpapi.Options{}))
	t.Cleanup(srv.Close)
	return &rig{a: a, srv: srv}
}

func (r *rig) wantRebootLeft(t *testing.T, token, when string, want int) {
	t.Helper()
	if b := r.botBody(t, token); b.RebootLeftBeats != want || !b.Arena.In {
		t.Errorf("%s: %+v, want %d beats left", when, b, want)
	}
}

func TestPutBotRebootsTheBotAndGetBotReportsTheCountdown(t *testing.T) {
	r := newRigWith(t, httpapi.Options{Beat: time.Second}, app.Config{RebootBeats: 5})
	j := r.join(t, "Ann")
	r.tick(t, 3)
	r.wantRebootLeft(t, j.Token, "before a save", -1)
	for range 2 { // no 429 for a second save: the reboot is the limit
		if code, body, _ := r.call(t, "PUT", "/api/bot", j.Token, validBot); code != 204 {
			t.Fatalf("PUT: %d %s", code, body)
		}
	}
	r.wantRebootLeft(t, j.Token, "queued", 5)
	r.tick(t, 1)
	r.wantRebootLeft(t, j.Token, "rebooting", 4)
	r.tick(t, 4)
	r.wantRebootLeft(t, j.Token, "back", -1)
	if b := r.botBody(t, j.Token); b.RebootTotalBeats != 5 {
		t.Errorf("total %d", b.RebootTotalBeats)
	}
}

func TestReturnTooSoonAfterOwnLeaveGives429WhenSavesReboot(t *testing.T) {
	r := newRigWith(t, httpapi.Options{Beat: time.Second}, app.Config{RebootBeats: 5})
	j := r.join(t, "Ann")
	r.tick(t, 3)
	if code, body, _ := r.call(t, "POST", "/api/bot/leave", j.Token, ""); code != 204 {
		t.Fatalf("leave: %d %s", code, body)
	}
	r.tick(t, 4) // past the swap limit, inside the reboot time
	if code, body, _ := r.call(t, "POST", "/api/bot/return", j.Token, ""); code != 429 {
		t.Errorf("return: %d %s", code, body)
	}
}
