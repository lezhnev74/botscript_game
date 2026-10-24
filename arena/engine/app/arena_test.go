package app_test

import (
	"context"
	"errors"
	"maps"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/lezhnev/arena/engine/app"
	"github.com/lezhnev/arena/engine/app/apptest"
	"github.com/lezhnev/arena/engine/domain/event"
	"github.com/lezhnev/arena/engine/domain/sim"
	"github.com/lezhnev/arena/engine/lang"
)

const room = "#########\n#.......#\n#.......#\n#.......#\n#.......#\n#.......#\n#########\n"

type rig struct {
	errs  []error // reported through Deps.OnError
	a     *app.Arena
	store *apptest.MemStore
	port  app.Store // what the arena is given; the store unless useStore replaces it
	feed  *apptest.Feed
	ids   *apptest.Seq
	cfg   app.Config
}

func newRig(t *testing.T, minBots int) *rig {
	t.Helper()
	r := newUnstartedRig(minBots, room)
	r.start(t)
	return r
}

func newUnstartedRig(minBots int, roomMap string) *rig {
	r := &rig{store: &apptest.MemStore{}, feed: &apptest.Feed{}, ids: &apptest.Seq{}}
	r.cfg = app.Config{Rules: sim.DefaultRules(), Map: roomMap, Seed: 11, MinBots: minBots}
	return r
}

const closet = "###\n#.#\n###\n" // one floor cell

func (r *rig) start(t *testing.T) {
	t.Helper()
	a, err := app.NewArena(r.cfg, app.Deps{Store: r.storePort(), Broadcaster: r.feed, IDs: r.ids, OnError: r.onError})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(a.Close)
	r.a = a
}

func (r *rig) useStore(s app.Store) { r.port = s }

func (r *rig) storePort() app.Store {
	if r.port != nil {
		return r.port
	}
	return r.store
}

func (r *rig) onError(err error) { r.errs = append(r.errs, err) }

func (r *rig) tick(t *testing.T, n int) {
	t.Helper()
	for range n {
		if err := r.a.Tick(); err != nil {
			t.Fatal(err)
		}
	}
}

func (r *rig) join(t *testing.T, name string) (token, id string) {
	t.Helper()
	token, id, err := r.a.Join(name)
	if err != nil {
		t.Fatal(err)
	}
	return token, id
}

func (r *rig) roster() string {
	var out []string
	for _, p := range r.feed.Last().Roster {
		out = append(out, p.ID)
	}
	return strings.Join(out, ",")
}

const waiter = "function loop() { wait(); }"

func TestJoinGivesATokenAndAnIDAndNoBotYet(t *testing.T) {
	r := newRig(t, 0)
	token, id := r.join(t, "Ann")
	if token == "" || id != "p1" || len(r.feed.Last().Roster) != 0 {
		t.Fatalf("token %q id %q roster %v", token, id, r.feed.Last().Roster)
	}
}

func TestTheJoinedBotAppearsAtTheNextBeat(t *testing.T) {
	r := newRig(t, 0)
	r.join(t, "Ann")
	r.tick(t, 1)
	u := r.feed.Last()
	if u.Beat != 1 || len(u.World.Bots) != 1 || u.World.Bots[0].ID != "p1" || u.Roster[0].Name != "Ann" {
		t.Errorf("update %+v", u)
	}
}

func TestJoinRejectsBadNames(t *testing.T) {
	r := newRig(t, 0)
	for _, name := range []string{"", "   ", strings.Repeat("x", 25)} {
		if _, _, err := r.a.Join(name); !errors.Is(err, app.ErrBadName) {
			t.Errorf("name %q: err %v", name, err)
		}
	}
}

func TestGetBotReturnsTheRookieTemplate(t *testing.T) {
	r := newRig(t, 0)
	token, _ := r.join(t, "Ann")
	v, err := r.a.GetBot(token)
	rookie, _ := lang.HouseBot("rookie")
	if err != nil || v.ID != "p1" || v.Name != "Ann" || v.Source != rookie {
		t.Errorf("view %+v err %v", v, err)
	}
}

func TestTokenIsTheOnlyAuth(t *testing.T) {
	r := newRig(t, 0)
	token, id := r.join(t, "Ann")
	if err := r.a.Kick(id); err != nil {
		t.Fatal(err)
	}
	for _, tok := range []string{"nope", token} {
		_, e1 := r.a.GetBot(tok)
		e2 := r.a.PutBot(tok, waiter)
		e3 := r.a.Leave(tok)
		for _, err := range []error{e1, e2, e3} {
			if !errors.Is(err, app.ErrUnauthorized) {
				t.Errorf("token %q: err %v", tok, err)
			}
		}
	}
}

func TestPutBotReturnsValidationErrorsWithPositions(t *testing.T) {
	r := newRig(t, 0)
	token, _ := r.join(t, "Ann")
	err := r.a.PutBot(token, "function loop() {\n  var x = 1;\n}")
	var ve *app.ValidationError
	if !errors.As(err, &ve) || len(ve.Errs) == 0 || ve.Errs[0].Line != 2 || ve.Errs[0].Hint == "" {
		t.Fatalf("err = %v", err)
	}
	if v, _ := r.a.GetBot(token); !strings.Contains(v.Source, "scan") {
		t.Error("an invalid put changed the bot")
	}
	if err := r.a.PutBot(token, waiter); err != nil {
		t.Errorf("an invalid put used up the rate limit: %v", err)
	}
}

func TestPutBotAppliesAtTheNextBeatAndIsLogged(t *testing.T) {
	r := newRig(t, 0)
	token, _ := r.join(t, "Ann")
	r.tick(t, 2)
	if err := r.a.PutBot(token, waiter); err != nil {
		t.Fatal(err)
	}
	if got := r.a.Record().EditLog; len(got) != 0 {
		t.Fatalf("edit logged before its beat: %v", got)
	}
	r.tick(t, 1)
	want := []app.Edit{{Beat: 3, Bot: "p1", Brain: app.BrainSpec{Kind: app.KindProgram, Src: waiter}}}
	if got := r.a.Record().EditLog; !slices.Equal(got, want) {
		t.Errorf("edit log %v, want %v", got, want)
	}
	if v, _ := r.a.GetBot(token); v.Source != waiter {
		t.Error("GetBot does not show the new source")
	}
}

func TestPutBotIsRateLimitedToOneSwapPerThreeBeats(t *testing.T) {
	r := newRig(t, 0)
	token, _ := r.join(t, "Ann")
	r.tick(t, 1)
	if err := r.a.PutBot(token, waiter); err != nil { // lands on beat 2
		t.Fatal(err)
	}
	err := r.a.PutBot(token, waiter)
	var rl *app.RateLimitError
	if !errors.Is(err, app.ErrRateLimited) || !errors.As(err, &rl) || rl.RetryBeats != 3 {
		t.Fatalf("err = %v", err)
	}
	r.tick(t, 2) // the next swap could land on beat 5
	if err := r.a.PutBot(token, waiter); !errors.Is(err, app.ErrRateLimited) {
		t.Errorf("a swap landing on beat 4: err %v", err)
	}
	r.tick(t, 1)
	if err := r.a.PutBot(token, waiter); err != nil {
		t.Errorf("three beats later: %v", err)
	}
}

func TestLeaveTakesEffectAtTheNextBeat(t *testing.T) {
	r := newRig(t, 0)
	token, _ := r.join(t, "Ann")
	r.tick(t, 2)
	if err := r.a.Leave(token); err != nil {
		t.Fatal(err)
	}
	if r.roster() != "p1" {
		t.Errorf("left before the beat boundary: %s", r.roster())
	}
	r.tick(t, 1)
	rec := r.a.Record()
	if r.roster() != "" || len(rec.Leaves) != 1 || rec.Leaves[0] != (app.Leave{Beat: 3, Bot: "p1"}) {
		t.Errorf("roster %q leaves %v", r.roster(), rec.Leaves)
	}
}

func TestLeavingBeforeTheJoinBeatCancelsTheJoin(t *testing.T) {
	r := newRig(t, 0)
	token, _ := r.join(t, "Ann")
	_ = r.a.Leave(token)
	r.tick(t, 1)
	if rec := r.a.Record(); len(rec.Joins)+len(rec.Leaves) != 0 || r.roster() != "" {
		t.Errorf("record %+v roster %q", rec, r.roster())
	}
}

func TestEditThenLeaveOnTheSameBeatDropsTheEdit(t *testing.T) {
	r := newRig(t, 0)
	token, _ := r.join(t, "Ann")
	r.tick(t, 1)
	_ = r.a.PutBot(token, waiter)
	_ = r.a.Leave(token)
	r.tick(t, 1)
	if rec := r.a.Record(); len(rec.EditLog) != 0 || len(rec.Leaves) != 1 {
		t.Errorf("record %+v", rec)
	}
}

func TestHouseBotsFillTheArenaUpToMinBots(t *testing.T) {
	r := newRig(t, 3)
	r.tick(t, 1)
	if r.roster() != "house-1,house-2,house-3" {
		t.Fatalf("roster %q", r.roster())
	}
	for _, p := range r.feed.Last().Roster {
		if !p.House {
			t.Errorf("%+v should be a house bot", p)
		}
	}
}

func TestHumansReplaceHouseBotsOldestFirstAndHouseBotsReturn(t *testing.T) {
	r := newRig(t, 3)
	r.tick(t, 1)
	t1, _ := r.join(t, "Ann")
	r.tick(t, 1)
	if r.roster() != "house-2,house-3,p1" {
		t.Fatalf("one human: %q", r.roster())
	}
	r.join(t, "Bob")
	r.join(t, "Cy")
	r.join(t, "Di")
	r.tick(t, 1)
	if r.roster() != "p1,p2,p3,p4" {
		t.Fatalf("four humans: %q", r.roster())
	}
	_ = r.a.Leave(t1)
	r.tick(t, 1)
	if r.roster() != "p2,p3,p4" {
		t.Fatalf("three humans, no filler needed: %q", r.roster())
	}
}

func TestFiveHouseBotsAtStartEachJoinReplacesOneAndALeaveRefills(t *testing.T) {
	r := newRig(t, 5)
	r.tick(t, 1)
	if r.roster() != "house-1,house-2,house-3,house-4,house-5" {
		t.Fatalf("start: %q", r.roster())
	}
	t1, _ := r.join(t, "Ann")
	r.join(t, "Bob")
	r.tick(t, 1)
	if r.roster() != "house-3,house-4,house-5,p1,p2" {
		t.Fatalf("2 joins: %q", r.roster())
	}
	_ = r.a.Leave(t1)
	r.tick(t, 1)
	if r.roster() != "house-3,house-4,house-5,house-6,p2" {
		t.Errorf("1 leave: %q", r.roster())
	}
}

func TestHouseFillerRefillsWhenHumansLeave(t *testing.T) {
	r := newRig(t, 3)
	t1, _ := r.join(t, "Ann")
	r.tick(t, 1)
	if r.roster() != "house-1,house-2,p1" {
		t.Fatalf("roster %q", r.roster())
	}
	_ = r.a.Leave(t1)
	r.tick(t, 1)
	if r.roster() != "house-1,house-2,house-3" {
		t.Errorf("roster %q", r.roster())
	}
}

func TestLeaderboardIsPersistedAndEqualsTheEventLogProjection(t *testing.T) {
	r := newRig(t, 4)
	var log []event.Logged
	for range 400 {
		r.tick(t, 1)
		log = append(log, r.feed.Last().Events...)
	}
	want := oracle(log, 1)
	if totalDeaths(want) == 0 {
		t.Fatal("no deaths in 400 beats: the test proves nothing")
	}
	if got := stats(r.a.Leaderboard()); !maps.Equal(got, want) {
		t.Errorf("leaderboard %v, event log %v", got, want)
	}
	if got := stats(r.store.Board); !maps.Equal(got, want) {
		t.Errorf("stored %v, event log %v", got, want)
	}
}

func TestConfiguredCoinPointsReachTheLeaderboard(t *testing.T) {
	r := newUnstartedRig(4, room)
	r.cfg.Points = &app.Points{Kill: 3, Repeat: 1, Coin: 3, Death: -1}
	r.start(t)
	var log []event.Logged
	for range 400 {
		r.tick(t, 1)
		log = append(log, r.feed.Last().Events...)
	}
	want := oracle(log, 3)
	coins := 0
	for _, v := range want {
		coins += v[3]
	}
	if coins == 0 {
		t.Fatal("no coin taken in 400 beats: the test proves nothing")
	}
	if got := stats(r.a.Leaderboard()); !maps.Equal(got, want) {
		t.Errorf("leaderboard %v, event log at 3 per coin %v", got, want)
	}
	board := app.NewBoard(r.a.Leaderboard()).WithPoints(*r.cfg.Points)
	if err := app.CheckBoard(board, log); err != nil {
		t.Error(err)
	}
}

func totalDeaths(m map[string][4]int) int {
	n := 0
	for _, v := range m {
		n += v[2]
	}
	return n
}

// stats keys entries by bot, leaving out joined bots that scored nothing.
func stats(entries []app.Entry) map[string][4]int {
	out := map[string][4]int{}
	for _, e := range entries {
		if v := [4]int{e.Score, e.Kills, e.Deaths, e.Coins}; v != [4]int{} {
			out[e.Bot] = v
		}
	}
	return out
}

// oracle scores straight from the events' text form: score, kills, deaths, coins.
func oracle(log []event.Logged, coin int) map[string][4]int {
	out := map[string][4]int{}
	seen := map[string]bool{} // "killer>victim" pairs already killed
	for _, l := range log {
		a := l.Event.Args()
		switch l.Event.Name() {
		case "Killed":
			out[a[1]] = shift(out[a[1]], -1, 0, 1, 0)
			creditKiller(out, seen, a[0], a[1])
		case "CoinTaken":
			out[a[0]] = shift(out[a[0]], coin, 0, 0, 1)
		}
	}
	return out
}

func creditKiller(out map[string][4]int, seen map[string]bool, killer, victim string) {
	if killer == "" || killer == victim {
		return
	}
	pts := 3
	if seen[killer+">"+victim] {
		pts = 1
	}
	seen[killer+">"+victim] = true
	out[killer] = shift(out[killer], pts, 1, 0, 0)
}

func shift(v [4]int, score, kills, deaths, coins int) [4]int {
	return [4]int{max(v[0]+score, 0), v[1] + kills, v[2] + deaths, v[3] + coins}
}

func TestRestartLoadsTheBoardAndKeepsPlayersTokens(t *testing.T) {
	r := newRig(t, 0)
	token, id := r.join(t, "Ann")
	r.tick(t, 1)
	r.a.Close()
	r.store.Board = []app.Entry{{Bot: id, Name: "Ann", Kills: 7, Deaths: 2}}
	r.start(t)
	if top := r.a.Leaderboard(); len(top) != 1 || top[0].Kills != 7 {
		t.Fatalf("board after restart: %+v", top)
	}
	v, err := r.a.GetBot(token)
	if err != nil || v.ID != id || v.Name != "Ann" {
		t.Fatalf("player after restart: %+v %v", v, err)
	}
	r.tick(t, 1)
	if r.roster() != id {
		t.Errorf("roster %q", r.roster())
	}
}

func TestStoreErrorsSurfaceFromTickAfterThePlayedBeat(t *testing.T) {
	r := newRig(t, 0)
	r.join(t, "Ann")
	r.store.Err = errors.New("disk full")
	if err := r.a.Tick(); err == nil || !strings.Contains(err.Error(), "disk full") {
		t.Errorf("err = %v", err)
	}
	if r.feed.Last().Beat != 1 {
		t.Error("the beat should still have been played")
	}
}

func TestNewArenaRejectsBadConfig(t *testing.T) {
	deps := app.Deps{Store: &apptest.MemStore{}, Broadcaster: &apptest.Feed{}, IDs: &apptest.Seq{}}
	for name, m := range map[string]string{"bots on the map": "####\n#A.#\n####\n", "bad map": ""} {
		cfg := app.Config{Rules: sim.DefaultRules(), Map: m}
		if _, err := app.NewArena(cfg, deps); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	cfg := app.Config{Rules: sim.DefaultRules(), Map: room, Houses: []string{"nobody"}}
	if _, err := app.NewArena(cfg, deps); err == nil {
		t.Error("unknown house bot accepted")
	}
}

func TestTickAfterCloseFails(t *testing.T) {
	r := newRig(t, 0)
	r.a.Close()
	r.a.Close()
	if err := r.a.Tick(); !errors.Is(err, app.ErrClosed) {
		t.Errorf("err = %v", err)
	}
}

func TestRunPlaysOneBeatPerClockTick(t *testing.T) {
	r := newRig(t, 2)
	clock := apptest.NewManualClock()
	done := make(chan error)
	go func() { done <- r.a.Run(context.Background(), clock) }()
	clock.Advance(5)
	clock.Stop()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if r.feed.Last().Beat != 5 {
		t.Errorf("beat %d, want 5", r.feed.Last().Beat)
	}
}

func TestRunStopsOnContextAndWhenTheArenaIsClosed(t *testing.T) {
	r := newRig(t, 0)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := r.a.Run(ctx, apptest.NewManualClock()); err != nil {
		t.Errorf("cancelled run: %v", err)
	}
	r.a.Close()
	clock := apptest.NewManualClock()
	done := make(chan error)
	go func() { done <- r.a.Run(context.Background(), clock) }()
	clock.Advance(1)
	if err := <-done; !errors.Is(err, app.ErrClosed) {
		t.Errorf("run after close: %v", err)
	}
}

func (r *rig) racer(t *testing.T, leave bool) {
	token, _, err := r.a.Join("racer")
	if err != nil {
		t.Error(err)
		return
	}
	for range 10 {
		_ = r.a.PutBot(token, waiter)
		_, _ = r.a.GetBot(token)
		_ = r.a.Leaderboard()
	}
	if leave {
		_ = r.a.Leave(token)
	}
}

func TestCommandsAndBeatsMayRaceSafely(t *testing.T) {
	r := newRig(t, 2)
	var wg sync.WaitGroup
	for i := range 4 {
		wg.Go(func() { r.racer(t, i%2 == 0) })
	}
	r.tick(t, 30)
	wg.Wait()
	r.tick(t, 2)
}

func TestJoinIntoAFullArenaFailsAtOnceWithoutIssuingATokenOrID(t *testing.T) {
	r := newUnstartedRig(0, closet)
	r.start(t)
	r.join(t, "Ann")
	if token, id, err := r.a.Join("Bob"); !errors.Is(err, app.ErrFull) || token != "" || id != "" {
		t.Fatalf("token %q id %q err %v", token, id, err)
	}
}

func TestJoinStopsAtMaxPlayersAndHouseBotsDoNotCount(t *testing.T) {
	r := newUnstartedRig(3, room)
	r.cfg.MaxPlayers = 2
	r.start(t)
	r.join(t, "Ann")
	r.join(t, "Bob")
	if _, _, err := r.a.Join("Cy"); !errors.Is(err, app.ErrFull) {
		t.Fatalf("third player: %v", err)
	}
}

func TestMaxPlayersAboveTheFloorLeavesTheFloorAsTheCap(t *testing.T) {
	r := newUnstartedRig(0, closet)
	r.cfg.MaxPlayers = 50
	r.start(t)
	r.join(t, "Ann")
	if _, _, err := r.a.Join("Bob"); !errors.Is(err, app.ErrFull) {
		t.Fatalf("second player in a one-cell room: %v", err)
	}
}

func savedBots() []app.SavedBot {
	return []app.SavedBot{
		{Token: "t-a", ID: "a1", Name: "Ann", Source: waiter},
		{Token: "t-b", ID: "b1", Name: "Bob", Source: waiter},
	}
}

func TestRestoredPlayersWaitForRoomAndAreNeverDropped(t *testing.T) {
	r := newUnstartedRig(0, closet)
	r.store.Bots = savedBots()
	r.start(t)
	r.tick(t, 3)
	if r.roster() != "a1" {
		t.Fatalf("roster %q, want only a1 (one cell)", r.roster())
	}
	if _, err := r.a.GetBot("t-b"); err != nil || len(r.store.Bots) != 2 {
		t.Errorf("the waiting player lost the token (%v) or the store (%d bots)", err, len(r.store.Bots))
	}
	if hasLine(r.a.Leaderboard(), "b1") {
		t.Error("a waiting player already has a leaderboard line")
	}
}

func hasLine(entries []app.Entry, bot string) bool {
	return slices.ContainsFunc(entries, func(e app.Entry) bool { return e.Bot == bot })
}

func TestAWaitingPlayerTakesTheCellFreedByALeaver(t *testing.T) {
	r := newUnstartedRig(0, closet)
	r.store.Bots = savedBots()
	r.start(t)
	r.tick(t, 3)
	if err := r.a.Leave("t-a"); err != nil {
		t.Fatal(err)
	}
	r.tick(t, 1)
	rec := r.a.Record()
	if r.roster() != "b1" || len(rec.Joins) != 2 || rec.Joins[1].Beat != 4 {
		t.Errorf("roster %q joins %+v: only successful joins are recorded, at the beat they happened", r.roster(), rec.Joins)
	}
}

func TestMinBotsAboveTheFloorIsRejected(t *testing.T) {
	r := newUnstartedRig(2, closet)
	_, err := app.NewArena(r.cfg, app.Deps{Store: r.store, Broadcaster: r.feed, IDs: r.ids})
	if err == nil {
		t.Error("MinBots 2 in a one-cell arena was accepted")
	}
}

func TestHouseFillerNeverMakesPhantomBoardLines(t *testing.T) {
	r := newUnstartedRig(1, closet)
	r.store.Bots = savedBots()
	r.start(t)
	r.tick(t, 20)
	for _, e := range r.a.Leaderboard() {
		if strings.HasPrefix(e.Bot, "house") || e.Bot == "b1" {
			t.Errorf("phantom line %+v", e)
		}
	}
	if len(r.a.Record().Joins) != 1 {
		t.Errorf("joins %+v", r.a.Record().Joins)
	}
}

func TestRestartedIDGeneratorCannotCollideWithKnownBotsOrTokens(t *testing.T) {
	r := newRig(t, 0)
	_, first := r.join(t, "Ann")
	left, _ := r.join(t, "Bob")
	r.tick(t, 1)
	_ = r.a.Leave(left)
	r.tick(t, 1) // p2 left but keeps its leaderboard line
	r.a.Close()
	r.ids = &apptest.Seq{} // a restart: p1 and tok-1 are drawn again
	r.start(t)
	token, id := r.join(t, "Cy")
	if id == first || id == "p2" || id != "p3" || token != "tok-3" {
		t.Errorf("new player got id %q token %q", id, token)
	}
}

type stuck struct{}

func (stuck) BotID() string { return "same" }
func (stuck) Token() string { return "same-token" }

func TestStuckIDGeneratorFailsInsteadOfCorruptingState(t *testing.T) {
	r := newUnstartedRig(0, room)
	r.start(t)
	a, err := app.NewArena(r.cfg, app.Deps{Store: &apptest.MemStore{}, Broadcaster: r.feed, IDs: stuck{}})
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	if _, _, err := a.Join("Ann"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := a.Join("Bob"); !errors.Is(err, app.ErrIDCollision) {
		t.Errorf("err = %v", err)
	}
}

func TestStoreErrorsAreReportedAndDoNotStopRun(t *testing.T) {
	r := newRig(t, 0)
	r.join(t, "Ann")
	r.store.Err = errors.New("disk full")
	clock := apptest.NewManualClock()
	done := make(chan error)
	go func() { done <- r.a.Run(context.Background(), clock) }()
	clock.Advance(3)
	clock.Stop()
	if err := <-done; err != nil {
		t.Fatalf("Run stopped: %v", err)
	}
	if r.feed.Last().Beat != 3 || len(r.errs) != 3 || !strings.Contains(r.errs[0].Error(), "disk full") {
		t.Errorf("beat %d, errors %v", r.feed.Last().Beat, r.errs)
	}
	r.store.Err = nil
	r.tick(t, 1)
	if len(r.store.Bots) != 1 || len(r.store.Board) != 1 {
		t.Errorf("the next beat should have retried the saves: bots %v board %v", r.store.Bots, r.store.Board)
	}
}

func TestRunWithoutAnErrorHandlerKeepsGoing(t *testing.T) {
	r := newUnstartedRig(0, room)
	a, err := app.NewArena(r.cfg, app.Deps{Store: r.store, Broadcaster: r.feed, IDs: r.ids})
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	_, _, _ = a.Join("Ann")
	r.store.Err = errors.New("disk full")
	clock := apptest.NewManualClock()
	done := make(chan error)
	go func() { done <- a.Run(context.Background(), clock) }()
	clock.Advance(2)
	clock.Stop()
	if err := <-done; err != nil || r.feed.Last().Beat != 2 {
		t.Errorf("err %v beat %d", err, r.feed.Last().Beat)
	}
}

// gate is a Broadcaster whose Publish blocks until released.
type gate struct {
	entered chan int
	release chan struct{}
	order   []int
	mu      sync.Mutex
}

func (g *gate) Publish(u app.Update) {
	g.entered <- u.Beat
	<-g.release
	g.mu.Lock()
	g.order = append(g.order, u.Beat)
	g.mu.Unlock()
}

func TestSlowBroadcasterDoesNotBlockCommandsButBeatsStayOrdered(t *testing.T) {
	r := newUnstartedRig(0, room)
	g := &gate{entered: make(chan int), release: make(chan struct{})}
	a, err := app.NewArena(r.cfg, app.Deps{Store: r.store, Broadcaster: g, IDs: r.ids})
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	token, _, _ := a.Join("Ann")
	var wg sync.WaitGroup
	wg.Go(func() { _ = a.Tick() })
	<-g.entered // beat 1 is being published, slowly
	if _, err := a.GetBot(token); err != nil {
		t.Fatal(err) // a command must not wait for the broadcast
	}
	_, _, _ = a.Join("Bob")
	wg.Go(func() { _ = a.Tick() }) // beat 2 is computed but must wait its turn to publish
	g.release <- struct{}{}
	if beat := <-g.entered; beat != 2 {
		t.Fatalf("published beat %d after 1", beat)
	}
	g.release <- struct{}{}
	wg.Wait()
	if !slices.Equal(g.order, []int{1, 2}) {
		t.Errorf("publish order %v", g.order)
	}
}

func TestConcurrentTicksPublishInBeatOrder(t *testing.T) {
	r := newRig(t, 2)
	var wg sync.WaitGroup
	for range 4 {
		wg.Go(func() {
			for range 15 {
				_ = r.a.Tick()
			}
		})
	}
	wg.Wait()
	beats := r.feed.Updates
	if len(beats) != 60 {
		t.Fatalf("%d updates", len(beats))
	}
	for i, u := range beats {
		if u.Beat != i+1 {
			t.Fatalf("update %d carries beat %d", i, u.Beat)
		}
	}
}
