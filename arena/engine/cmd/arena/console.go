package main

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/lezhnev/arena/engine/adapters/httpapi"
	"github.com/lezhnev/arena/engine/app"
)

// admin is what the console needs from the arena (app.Arena implements it).
type admin interface {
	Bots() []app.BotInfo
	Kick(id string) error
	ResetBoard() error
	SetMinBots(n int) error
	SetTimer(beats int) error
	StartGame() error
}

const consoleHelp = `commands:
  list               every bot: id, name, kind, score
  kick <id|name>     remove a human bot and its leaderboard line
  reset-board        wipe the leaderboard (bots stay)
  min-bots <n>       fill the arena with house bots up to n bots
  timer <minutes>    end the game after this many minutes (0 = no timer)
  start              start the next game after one has ended
  help               this text`

// console reads admin commands, one per line, from in and answers on out. It
// returns at EOF (or a read error); the server keeps running without it.
func console(in io.Reader, out io.Writer, a admin, beat time.Duration) {
	sc := bufio.NewScanner(in)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line != "" {
			fmt.Fprintln(out, execute(line, a, beat))
		}
	}
}

// commands maps a console word to its handler (arg is the rest of the line).
var commands = map[string]func(arg string, a admin, beat time.Duration) string{
	"list":        func(_ string, a admin, _ time.Duration) string { return list(a.Bots()) },
	"kick":        func(arg string, a admin, _ time.Duration) string { return kick(arg, a) },
	"reset-board": func(_ string, a admin, _ time.Duration) string { return resetBoard(a) },
	"min-bots":    func(arg string, a admin, _ time.Duration) string { return minBots(arg, a) },
	"timer":       timer,
	"start":       func(_ string, a admin, _ time.Duration) string { return startGame(a) },
	"help":        func(string, admin, time.Duration) string { return consoleHelp },
}

// execute runs one command line and returns its output.
func execute(line string, a admin, beat time.Duration) string {
	cmd, arg, _ := strings.Cut(line, " ")
	if run, ok := commands[cmd]; ok {
		return run(strings.TrimSpace(arg), a, beat)
	}
	return fmt.Sprintf("unknown command %q (try: help)", cmd)
}

func resetBoard(a admin) string {
	if err := a.ResetBoard(); err != nil {
		return "error: " + err.Error()
	}
	return "leaderboard reset"
}

func timer(arg string, a admin, beat time.Duration) string {
	m, err := strconv.ParseFloat(arg, 64)
	if err != nil || !(m >= 0) || beat <= 0 {
		return "usage: timer <minutes>"
	}
	if err := a.SetTimer(httpapi.MinutesToBeats(m, beat)); err != nil {
		return "error: " + err.Error()
	}
	if m == 0 {
		return "timer off"
	}
	return fmt.Sprintf("the game ends in %g minutes", m)
}

func startGame(a admin) string {
	if err := a.StartGame(); err != nil {
		return "error: " + err.Error()
	}
	return "new game started"
}

func minBots(arg string, a admin) string {
	n, err := strconv.Atoi(arg)
	if err != nil {
		return "usage: min-bots <n>"
	}
	if err := a.SetMinBots(n); err != nil {
		return "error: " + err.Error()
	}
	return fmt.Sprintf("min bots is now %d", n)
}

func list(bots []app.BotInfo) string {
	lines := make([]string, len(bots))
	for i, b := range bots {
		kind := "human"
		if b.House {
			kind = "house"
		}
		lines[i] = fmt.Sprintf("%-10s %-24s %-5s %6d", b.ID, b.Name, kind, b.Score)
		if b.Detached {
			lines[i] += "  detached"
		}
	}
	return strings.Join(lines, "\n")
}

// kick resolves an id or an exact name to one bot and kicks it.
func kick(target string, a admin) string {
	if target == "" {
		return "usage: kick <id|name>"
	}
	matches := findBots(a.Bots(), target)
	switch len(matches) {
	case 0:
		return fmt.Sprintf("no bot with id or name %q", target)
	case 1:
		b := matches[0]
		if err := a.Kick(b.ID); err != nil {
			return "error: " + err.Error()
		}
		return fmt.Sprintf("kicked %s (%s)", b.ID, b.Name)
	}
	return fmt.Sprintf("%d matches for %q, kick by id:\n%s", len(matches), target, list(matches))
}

// findBots returns the bot with this id, else every bot with this name.
func findBots(bots []app.BotInfo, target string) []app.BotInfo {
	var byName []app.BotInfo
	for _, b := range bots {
		if b.ID == target {
			return []app.BotInfo{b}
		}
		if b.Name == target {
			byName = append(byName, b)
		}
	}
	return byName
}
