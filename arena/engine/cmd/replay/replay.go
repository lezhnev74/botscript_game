package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/lezhnev/arena/engine/adapters/protocol"
	"github.com/lezhnev/arena/engine/app"
	"github.com/lezhnev/arena/engine/domain/event"
	"github.com/lezhnev/arena/engine/scenario"
)

type config struct {
	file, hash, export, name string
	coinPts                  int
}

func parseConfig(args []string, stderr io.Writer) (config, error) {
	var c config
	fl := flag.NewFlagSet("replay", flag.ContinueOnError)
	fl.SetOutput(stderr)
	fl.StringVar(&c.hash, "hash", "", "expected hash of the event log; a mismatch is an error")
	fl.StringVar(&c.export, "export", "", "directory to write the WebSocket fixture <name>.json into")
	fl.StringVar(&c.name, "name", "", "fixture name (default: the input file name without extension)")
	fl.IntVar(&c.coinPts, "coin-points", app.DefaultPoints().Coin, "leaderboard points per coin, as the served run had (not part of the event log)")
	if err := fl.Parse(args); err != nil {
		return c, err
	}
	if c.coinPts < 0 {
		return c, fmt.Errorf("-coin-points must not be negative")
	}
	if fl.NArg() != 1 {
		return c, fmt.Errorf("want exactly one input file (.scn or record .json), got %d", fl.NArg())
	}
	c.file = fl.Arg(0)
	if c.name == "" {
		c.name = fixtureName(c.file)
	}
	return c, nil
}

// fixtureName is the file name without its directory and extensions
// ("duel_5x5.record.json" -> "duel_5x5").
func fixtureName(file string) string {
	base, _, _ := strings.Cut(filepath.Base(file), ".")
	return base
}

func run(args []string, stdout, stderr io.Writer) error {
	c, err := parseConfig(args, stderr)
	if err != nil {
		return err
	}
	rec, err := load(c.file)
	if err != nil {
		return err
	}
	msgs, log, err := play(rec, c.coinPts)
	if err != nil {
		return err
	}
	if err := report(c, log, stdout); err != nil {
		return err
	}
	if c.export == "" {
		return nil
	}
	return writeFixture(filepath.Join(c.export, c.name+".json"), msgs)
}

// report prints the hash of the log and checks it against -hash.
func report(c config, log []event.Logged, stdout io.Writer) error {
	hash, err := app.HashLog(log)
	if err != nil {
		return err
	}
	if c.hash != "" && c.hash != hash {
		return fmt.Errorf("hash mismatch: want %s, got %s", c.hash, hash)
	}
	fmt.Fprintln(stdout, "hash", hash)
	return nil
}

// load reads a scenario (.scn) as its Record, or a Record document.
func load(file string) (app.Record, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return app.Record{}, err
	}
	if filepath.Ext(file) != ".scn" {
		return app.DecodeRecord(data)
	}
	s, err := scenario.ParseDir(string(data), filepath.Dir(file))
	if err != nil {
		return app.Record{}, fmt.Errorf("%s: %w", file, err)
	}
	return s.Record(), nil
}

// play reruns the record and returns the WebSocket messages (the beat 0
// snapshot, then one beat message per beat) and the whole event log.
func play(rec app.Record, coinPts int) ([]protocol.Message, []event.Logged, error) {
	m, err := rec.NewMatch()
	if err != nil {
		return nil, nil, err
	}
	defer m.Close()
	pts := app.DefaultPoints()
	pts.Coin = coinPts
	board := app.NewBoard(nil).WithPoints(pts)
	u := update(m, nil, board, rec)
	msgs := []protocol.Message{protocol.FromUpdate(u, protocol.TypeSnapshot)}
	var log []event.Logged
	for beat := 1; beat <= rec.Beats; beat++ {
		evs := m.Step()
		if err := m.Err(); err != nil {
			return nil, nil, fmt.Errorf("beat %d: %w", beat, err)
		}
		log = append(log, evs...)
		msgs = append(msgs, protocol.FromUpdate(update(m, evs, board, rec), protocol.TypeBeat))
	}
	return msgs, log, nil
}

// update describes the match after a beat through app.NewUpdate, the same
// code the Arena publishes with. A replay has no player names, so a bot's name
// is its id.
func update(m *app.Match, evs []event.Logged, board *app.Board, rec app.Record) app.Update {
	roster := rosterOf(m)
	board.Apply(evs)
	for _, p := range roster {
		board.SetName(p.ID, p.Name)
	}
	return app.NewUpdate(m, evs, roster, editedBy(rec, m.World().Beat), board.Top())
}

func rosterOf(m *app.Match) []app.Player {
	w := m.World()
	var out []app.Player
	for _, b := range w.Bots {
		out = append(out, app.Player{ID: b.ID, Name: b.ID, Look: app.DefaultLook(b.ID)})
	}
	for _, d := range w.Fallen {
		out = append(out, app.Player{ID: d.Bot.ID, Name: d.Bot.ID, Look: app.DefaultLook(d.Bot.ID)})
	}
	for _, r := range w.Rebooting {
		out = append(out, app.Player{ID: r.Bot.ID, Name: r.Bot.ID, Look: app.DefaultLook(r.Bot.ID)})
	}
	return out
}

// editedBy maps a bot to the beat of its latest hot-swap up to beat.
func editedBy(rec app.Record, beat int) map[string]int {
	out := map[string]int{}
	for _, e := range rec.EditLog {
		if e.Beat <= beat {
			out[e.Bot] = e.Beat
		}
	}
	return out
}

// writeFixture writes a JSON array with one message per line.
func writeFixture(path string, msgs []protocol.Message) error {
	var buf bytes.Buffer
	buf.WriteString("[\n")
	for i, m := range msgs {
		b, err := json.Marshal(m)
		if err != nil {
			return err
		}
		buf.Write(b)
		if i < len(msgs)-1 {
			buf.WriteByte(',')
		}
		buf.WriteByte('\n')
	}
	buf.WriteString("]\n")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, buf.Bytes(), 0o644)
}
