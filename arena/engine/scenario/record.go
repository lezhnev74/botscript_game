package scenario

import (
	"github.com/lezhnev/arena/engine/app"
	"github.com/lezhnev/arena/engine/domain/brain"
)

// Record is the scenario as replayable data: Replay(s.Record()) yields the
// same event log as Run(s).
func (s Scenario) Record() app.Record {
	rec := app.Record{Version: app.RecordVersion, Rules: s.Rules, Map: s.Map.Render(), Seed: s.Seed, Beats: s.Beats}
	for _, b := range s.Bots {
		rec.Bots = append(rec.Bots, app.BotSpec{ID: b.ID, Brain: b.brainSpec()})
	}
	for _, e := range s.Edits {
		rec.EditLog = append(rec.EditLog, app.Edit{Beat: e.Beat, Bot: e.Bot, Brain: app.BrainSpec{Kind: app.KindProgram, Src: e.Program}, Reboot: e.Reboot})
	}
	for _, st := range s.Start {
		for _, f := range st.Fields {
			rec.Start = append(rec.Start, app.StartSpec{Bot: st.ID, Field: f.Key, Value: f.Value})
		}
	}
	for _, b := range s.Bullets {
		rec.Bullets = append(rec.Bullets, app.BulletSpec{X: b.Pos.X, Y: b.Pos.Y, Dir: b.Dir.String(), Owner: b.Owner})
	}
	return rec
}

// brainSpec is the bot's brain as record data: the program source itself, so
// a Record never refers to a file.
func (b BotSpec) brainSpec() app.BrainSpec {
	if b.Program != "" {
		return app.BrainSpec{Kind: app.KindProgram, Src: b.Program}
	}
	return app.BrainSpec{Kind: app.KindScript, Src: brain.FormatScript(b.Script)}
}
