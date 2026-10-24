package protocol

import (
	"github.com/lezhnev/arena/engine/app"
	"github.com/lezhnev/arena/engine/domain/event"
	"github.com/lezhnev/arena/engine/domain/geo"
	"github.com/lezhnev/arena/engine/domain/sim"
	"github.com/lezhnev/arena/engine/lang"
	"strconv"
	"strings"
)

// FromUpdate maps a published beat to a WebSocket message of the given type.
// A TypeSnapshot message carries no events.
func FromUpdate(u app.Update, typ string) Message {
	m := Message{Type: typ, Frame: frame(u), Events: []Event{}, Board: board(u.Board), Remaining: u.Remaining, GameOver: u.GameOver}
	if typ == TypeSnapshot {
		return m
	}
	for _, l := range u.Events {
		m.Events = append(m.Events, convertEvent(l))
	}
	return m
}

func frame(u app.Update) Frame {
	w := u.World
	names := make(map[string]string, len(u.Roster))
	houses := make(map[string]bool, len(u.Roster))
	looks := make(map[string]Look, len(u.Roster))
	for _, p := range u.Roster {
		names[p.ID], houses[p.ID], looks[p.ID] = p.Name, p.House, FromLook(p.Look)
	}
	f := Frame{
		Beat: u.Beat, Width: w.Grid.Width(), Height: w.Grid.Height(), Grid: gridRows(w.Grid),
		Bots: []Bot{}, Fallen: []FallenBot{}, Rebooting: []RebootingBot{}, Bullets: []Bullet{}, Coins: []Cell{}, MaxHP: w.MaxHP, Reload: w.Reload,
	}
	for _, b := range w.Bots {
		f.Bots = append(f.Bots, bot(b, names[b.ID], houses[b.ID], looks[b.ID], u.Probes))
	}
	for _, d := range w.Fallen {
		f.Fallen = append(f.Fallen, FallenBot{ID: d.Bot.ID, Name: names[d.Bot.ID], Look: looks[d.Bot.ID], X: d.Bot.Pos.X, Y: d.Bot.Pos.Y, Wait: d.Wait})
	}
	for _, r := range w.Rebooting {
		f.Rebooting = append(f.Rebooting, RebootingBot{Bot: r.Bot.ID, Name: names[r.Bot.ID], Look: looks[r.Bot.ID], Cell: Cell{X: r.Bot.Pos.X, Y: r.Bot.Pos.Y}, HP: rebootHP(r, w.MaxHP), BeatsLeft: r.Left, Total: r.Total})
	}
	for _, b := range w.Bullets {
		f.Bullets = append(f.Bullets, Bullet{X: b.Pos.X, Y: b.Pos.Y, Dir: b.Dir.String(), Owner: b.Owner})
	}
	for _, c := range w.Coins {
		f.Coins = append(f.Coins, Cell{X: c.X, Y: c.Y})
	}
	return f
}

func gridRows(g geo.Grid) []string {
	return strings.Split(strings.TrimRight(g.Render(nil), "\n"), "\n")
}

func bot(b sim.Bot, name string, house bool, look Look, probes map[string]int) Bot {
	act, ok := probes[b.ID+".act"]
	if !ok {
		act = -1
	}
	return Bot{
		ID: b.ID, Name: name, House: house, Look: look, X: b.Pos.X, Y: b.Pos.Y, HP: b.HP,
		Loaded: b.Loaded(), Bonus: b.Bonus, Shield: b.Shield, Cooldown: b.Cooldown, Act: act,
		Line: probes[b.ID+".line"], EditedBeat: probes[b.ID+".edited"],
		Num: numProbes(b.ID, probes),
	}
}

// numProbes is the program memory num[0..7] from the probes, zero for house bots.
func numProbes(id string, probes map[string]int) [8]int {
	var n [8]int
	for i := range n {
		n[i] = probes[id+".num_"+strconv.Itoa(i)]
	}
	return n
}

func convertEvent(l event.Logged) Event {
	e := Event{Beat: l.Beat, Type: l.Event.Name(), Args: l.Event.Args()}
	switch ev := l.Event.(type) {
	case event.Scanned:
		e.Cells, e.Seen = cells(ev.Cells), sightings(ev.Seen)
	case event.Hit:
		e.By = ev.Shooter
	}
	return e
}

func cells(in []event.Cell) []Cell {
	out := make([]Cell, len(in))
	for i, c := range in {
		out[i] = Cell{X: c.X, Y: c.Y}
	}
	return out
}

func sightings(in []event.Sighting) []Sighting {
	out := make([]Sighting, len(in))
	for i, s := range in {
		out[i] = Sighting{Kind: s.Kind, X: s.X, Y: s.Y, Level: s.Level}
	}
	return out
}

func board(in []app.Entry) []LeaderboardEntry {
	out := make([]LeaderboardEntry, len(in))
	for i, e := range in {
		out[i] = LeaderboardEntry{Bot: e.Bot, Name: e.Name, Score: e.Score, Kills: e.Kills, Deaths: e.Deaths, Coins: e.Coins}
	}
	return out
}

// FromBoard maps the leaderboard for GET /api/leaderboard.
func FromBoard(in []app.Entry) []LeaderboardEntry { return board(in) }

// FromBotState maps a bot's live state.
func FromBotState(id string, s app.BotState) BotState {
	out := BotState{ID: id, InArena: s.InArena, Num: s.Num, Pin: pinCells(s.Pin), Act: s.Act, Trail: []TrailEntry{}}
	for _, t := range s.Trail {
		out.Trail = append(out.Trail, TrailEntry{X: t.X, Y: t.Y, Act: int(t.Act), Dir: t.Dir, OK: t.OK, Hit: t.Hit, Bump: t.Bump})
	}
	return out
}

func pinCells(in [lang.PinSize]lang.Place) [8]*Cell {
	var out [8]*Cell
	for i, p := range in {
		if p.Set {
			out[i] = &Cell{X: p.X, Y: p.Y}
		}
	}
	return out
}

// FromValidation lists every problem of a rejected program.
func FromValidation(e *app.ValidationError) ValidationBody {
	out := ValidationBody{Errors: []SourceError{}}
	for _, x := range e.Errs {
		out.Errors = append(out.Errors, SourceError{Line: x.Line, Col: x.Col, Msg: x.Msg, Hint: x.Hint})
	}
	return out
}

// FromLook maps a look to the wire.
func FromLook(l app.Look) Look {
	return Look{Color: l.Color, Body: l.Body, Eyes: l.Eyes, Mouth: l.Mouth, Hat: l.Hat}
}

// ToLook maps a wire look back; the app validates it.
func ToLook(l Look) app.Look {
	return app.Look{Color: l.Color, Body: l.Body, Eyes: l.Eyes, Mouth: l.Mouth, Hat: l.Hat}
}

// FromCheck maps the result of app.Check to the wire.
func FromCheck(r app.CheckResult) CheckResponse {
	if r.Trial == nil {
		out := CheckResponse{Errors: []CheckError{}}
		for _, e := range r.Errors {
			out.Errors = append(out.Errors, CheckError{Line: e.Line, Col: e.Col, Message: e.Msg, Hint: e.Hint})
		}
		return out
	}
	t := r.Trial
	return CheckResponse{Ok: true, Trial: &CheckTrial{
		Beats: t.Beats, DeadBeats: t.DeadBeats,
		Actions: CheckActions{Scan: t.Actions.Scan, Move: t.Actions.Move, Fire: t.Actions.Fire, Wait: t.Actions.Wait},
		Idle:    t.Idle, NoAction: t.NoAction, Clicks: t.Clicks, HitsTaken: t.HitsTaken, Hits: t.Hits,
		Kills: t.Kills, Deaths: t.Deaths, Coins: t.Coins, Warnings: t.Warnings,
	}}
}

// rebootHP is the HP a rebooting bot comes back with: the HP it kept, or full
// for a bot that was dead when the reboot began.
func rebootHP(r sim.Rebooting, maxHP int) int {
	if r.Dead {
		return maxHP
	}
	return r.Bot.HP
}
