package app

import (
	"fmt"
	"math/rand/v2"
	"strings"

	"github.com/lezhnev/arena/engine/domain/brain"
	"github.com/lezhnev/arena/engine/domain/geo"
	"github.com/lezhnev/arena/engine/domain/sim"
	"github.com/lezhnev/arena/engine/lang"
)

// RandomParams sizes a generated match. Out-of-range values are clamped: maps
// are 5x5..9x9, 2..4 bots, at least 0 beats.
type RandomParams struct {
	W, H, Bots, Beats int
}

const (
	minSide, maxSide = 5, 9
	minBots, maxBots = 2, 4
	wallOneIn        = 7
)

func clamp(v, lo, hi int) int { return max(lo, min(v, hi)) }

func newRand(seed uint64) *rand.Rand { return rand.New(rand.NewPCG(seed, seed^0x9e3779b97f4a7c15)) }

// RandomRecord builds a reproducible random match: a walled map with a few
// inner walls, bots with random scripted action lists, and lightly varied
// rules. Each bot is a random action script or (half the time) a house
// program, and a program bot may get a hot-swap to another house program; the
// same seed always gives the same record.
func RandomRecord(seed uint64, p RandomParams) Record {
	r := newRand(seed)
	w, h := clamp(p.W, minSide, maxSide), clamp(p.H, minSide, maxSide)
	n := clamp(p.Bots, minBots, maxBots)
	rules := randomRules(r)
	// The generated map always parses, so the error cannot happen.
	rec, _ := RandomRecordOn(seed, rules, randomMap(r, w, h, n), p.Beats)
	addPrograms(newRand(seed^0x5bd1e995), &rec)
	addMembership(newRand(seed^0x2545f491), &rec)
	return rec
}

// addPrograms turns about half of the scripted bots into house programs and
// gives about a third of those one edit to a random house program.
func addPrograms(r *rand.Rand, rec *Record) {
	names := lang.HouseBotNames()
	for i := range rec.Bots {
		if r.IntN(2) == 0 {
			continue
		}
		rec.Bots[i].Brain = houseSpec(names[r.IntN(len(names))])
		if rec.Beats > 0 && r.IntN(3) == 0 {
			edit := Edit{Beat: 1 + r.IntN(rec.Beats), Bot: rec.Bots[i].ID, Brain: houseSpec(names[r.IntN(len(names))])}
			rec.EditLog = append(rec.EditLog, edit)
		}
	}
}

// maxJoins is how many bots may join a random match mid-way, if the room is
// big enough to always leave a free cell for them.
const maxJoins = 2

// addMembership gives a big enough room up to maxJoins joining bots (a house
// program or a random script, joining at a random beat) and lets about a
// quarter of all bots leave, always after their last edit and their join.
func addMembership(r *rand.Rand, rec *Record) {
	m, _ := geo.ParseMap(rec.Map) // RandomRecordOn parsed it
	if rec.Beats < 4 || floorCells(m.Grid) < 4*(len(rec.Bots)+maxJoins)+rec.Rules.CoinMax {
		return
	}
	joinBeat := map[string]int{}
	for i := range r.IntN(maxJoins + 1) {
		id := fmt.Sprintf("j%d", i+1)
		joinBeat[id] = 1 + r.IntN(rec.Beats)
		rec.Joins = append(rec.Joins, Join{Beat: joinBeat[id], Bot: id, Brain: joinBrain(r, rec.Beats)})
	}
	for _, id := range botIDs(rec) {
		earliest := max(joinBeat[id], lastEdit(rec.EditLog, id)) + 1
		if r.IntN(4) == 0 && earliest <= rec.Beats {
			rec.Leaves = append(rec.Leaves, Leave{Beat: earliest + r.IntN(rec.Beats-earliest+1), Bot: id})
		}
	}
}

func joinBrain(r *rand.Rand, beats int) BrainSpec {
	if r.IntN(2) == 0 {
		names := lang.HouseBotNames()
		return houseSpec(names[r.IntN(len(names))])
	}
	return BrainSpec{Kind: KindScript, Src: brain.FormatScript(randomActions(r, beats))}
}

func botIDs(rec *Record) []string {
	var ids []string
	for _, b := range rec.Bots {
		ids = append(ids, b.ID)
	}
	for _, j := range rec.Joins {
		ids = append(ids, j.Bot)
	}
	return ids
}

func lastEdit(log []Edit, id string) int {
	last := 0
	for _, e := range log {
		if e.Bot == id {
			last = max(last, e.Beat)
		}
	}
	return last
}

func houseSpec(name string) BrainSpec {
	src, _ := lang.HouseBot(name) // names come from HouseBotNames
	return BrainSpec{Kind: KindProgram, Src: src}
}

// RandomMap is a reproducible w x h room (clamped to 5..9) with n bots (2..4).
func RandomMap(seed uint64, w, h, n int) string {
	return randomMap(newRand(seed), clamp(w, minSide, maxSide), clamp(h, minSide, maxSide), clamp(n, minBots, maxBots))
}

// RandomRecordOn gives every bot on the given map a random script of the
// given length (beats, at least 0), drawn from the seed.
func RandomRecordOn(seed uint64, rules sim.Rules, mapText string, beats int) (Record, error) {
	m, err := geo.ParseMap(mapText)
	if err != nil {
		return Record{}, fmt.Errorf("map: %w", err)
	}
	r := newRand(seed ^ 0xa5a5a5a5)
	beats = max(beats, 0)
	rec := Record{Version: RecordVersion, Rules: rules, Map: mapText, Seed: seed, Beats: beats}
	for _, b := range m.Bots {
		src := brain.FormatScript(randomActions(r, beats))
		rec.Bots = append(rec.Bots, BotSpec{ID: string(b.Letter), Brain: BrainSpec{Kind: KindScript, Src: src}})
	}
	return rec, nil
}

func randomRules(r *rand.Rand) sim.Rules {
	rules := sim.DefaultRules()
	rules.HP = 1 + r.IntN(3)
	rules.BulletSpeed = 1 + r.IntN(4)
	rules.Reload = 1 + r.IntN(4)
	rules.RespawnBeats = 1 + r.IntN(6)
	rules.CoinEvery = 2 + r.IntN(8)
	rules.BonusMax = 1 + r.IntN(3)
	rules.ShieldBeats = r.IntN(4)
	rules.CoinMax = 1 + r.IntN(4)
	return rules
}

// randomMap is a w x h room: border walls, random inner walls, n bots A.. on
// distinct inner floor cells.
func randomMap(r *rand.Rand, w, h, n int) string {
	rows := make([][]byte, h)
	var inner []geo.Pos
	for y := range rows {
		rows[y] = []byte(strings.Repeat(".", w))
		for x := 0; x < w; x++ {
			if isBorder(x, y, w, h) {
				rows[y][x] = '#'
			} else {
				inner = append(inner, geo.Pos{X: x, Y: y})
			}
		}
	}
	for i, k := range r.Perm(len(inner)) {
		c := inner[k]
		rows[c.Y][c.X] = innerCell(r, i, n)
	}
	lines := make([]string, h)
	for y, row := range rows {
		lines[y] = string(row)
	}
	return strings.Join(lines, "\n") + "\n"
}

func isBorder(x, y, w, h int) bool { return x == 0 || y == 0 || x == w-1 || y == h-1 }

// innerCell is the i-th shuffled inner cell: one of the n bots, else floor or
// (one time in wallOneIn) a wall.
func innerCell(r *rand.Rand, i, n int) byte {
	switch {
	case i < n:
		return byte('A' + i)
	case r.IntN(wallOneIn) == 0:
		return '#'
	}
	return '.'
}

func randomActions(r *rand.Rand, n int) []brain.Action {
	acts := make([]brain.Action, n)
	for i := range acts {
		acts[i] = brain.Action{Kind: brain.Kind(r.IntN(4)), Dir: geo.Dir(r.IntN(4))}
		if acts[i].Kind == brain.Wait {
			acts[i].Dir = geo.N
		}
	}
	return acts
}

// PlaceBots puts n bots (letters A..) on random free floor cells of a bot-free
// map, reproducibly from the seed. A map that already has bots is returned as
// it is.
func PlaceBots(mapText string, seed uint64, n int) (string, error) {
	m, err := geo.ParseMap(mapText)
	if err != nil {
		return "", fmt.Errorf("map: %w", err)
	}
	if len(m.Bots) > 0 {
		return mapText, nil
	}
	rows := strings.Split(strings.TrimRight(mapText, "\n"), "\n")
	free := floorCellsOf(rows)
	if n < 1 || n > min(26, len(free)) {
		return "", fmt.Errorf("cannot place %d bots on %d free cells (1..26)", n, len(free))
	}
	var spots []geo.Pos
	for _, k := range newRand(seed).Perm(len(free))[:n] {
		spots = append(spots, free[k])
	}
	return stampBots(rows, spots), nil
}

// stampBots writes the letters A.. onto the given cells of the map rows.
func stampBots(rows []string, spots []geo.Pos) string {
	grid := make([][]byte, len(rows))
	for y, row := range rows {
		grid[y] = []byte(row)
	}
	for i, p := range spots {
		grid[p.Y][p.X] = byte('A' + i)
	}
	lines := make([]string, len(grid))
	for y, row := range grid {
		lines[y] = string(row)
	}
	return strings.Join(lines, "\n") + "\n"
}

// floorCellsOf lists the '.' cells of map rows, row by row.
func floorCellsOf(rows []string) []geo.Pos {
	var free []geo.Pos
	for y, row := range rows {
		for x, c := range row {
			if c == '.' {
				free = append(free, geo.Pos{X: x, Y: y})
			}
		}
	}
	return free
}
