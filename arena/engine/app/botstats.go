package app

import (
	"github.com/lezhnev/arena/engine/domain/event"
	"github.com/lezhnev/arena/engine/domain/geo"
)

// dodgeWindow is how many beats after a level-3 bullet reading a hit from the
// same side still counts as that bullet (a bullet crosses a 15 cell room in
// five beats at speed 3).
const dodgeWindow = 5

// BotStats are the tuning metrics of one bot (or one program, summed over
// matches). Joins and leaves are ignored: cmd/sim matches have none.
type BotStats struct {
	Kills, Deaths, Coins     int
	Shots, Hits, Clicks      int // Hits = own bullets that landed
	AliveBeats, IdleBeats    int // idle = alive and no action event (wait, wall no-op, frozen)
	Seen3, Dodged3           int // bullets reported at level 3, and those that did not hit from that side
	FirstLineSum, FirstLineN int // beat of the first scan with an enemy at level 3, over matches that had one
}

// PerBot reads the event log of a match of the given beats; ids are the bots
// on the map at beat 0.
func PerBot(log []event.Logged, beats int, ids []string) map[string]BotStats {
	t := &botTally{stats: map[string]*BotStats{}, acted: map[string]int{}, diedAt: map[string]int{},
		deadBeats: map[string]int{}, lastSeen: map[sighting]int{}}
	for _, id := range ids {
		t.stats[id] = &BotStats{}
	}
	for _, l := range log {
		t.count(l)
	}
	return t.finish(beats)
}

type sighting struct {
	bot string
	dir geo.Dir
}

type seenBullet struct {
	sighting
	beat int
}

type botTally struct {
	stats     map[string]*BotStats
	acted     map[string]int // bot -> action events
	diedAt    map[string]int // bot -> beat of its death, while dead
	deadBeats map[string]int
	lastSeen  map[sighting]int // last counted level-3 bullet reading
	seen      []seenBullet
	hits      []seenBullet // landed bullets: who was hit from which side
}

func (t *botTally) bot(id string) *BotStats {
	s, ok := t.stats[id]
	if !ok {
		s = &BotStats{}
		t.stats[id] = s
	}
	return s
}

func (t *botTally) count(l event.Logged) {
	switch e := l.Event.(type) {
	case event.Clicked:
		t.act(e.Bot)
		t.bot(e.Bot).Clicks++
	case event.Fired:
		t.act(e.Bot)
		t.bot(e.Bot).Shots++
	case event.Scanned:
		t.act(e.Bot)
		t.scan(l.Beat, e)
	case event.Hit:
		t.hit(l.Beat, e)
	default:
		t.countOther(l)
	}
}

func (t *botTally) countOther(l event.Logged) {
	switch e := l.Event.(type) {
	case event.Moved:
		t.act(e.Bot)
	case event.Bumped:
		t.act(e.Bot)
	case event.Killed:
		t.kill(l.Beat, e)
	case event.Respawned:
		t.respawn(l.Beat, e.Bot)
	case event.CoinTaken:
		t.bot(e.Bot).Coins++
	}
}

func (t *botTally) hit(beat int, e event.Hit) {
	if e.Shooter != "" {
		t.bot(e.Shooter).Hits++
	}
	t.hits = append(t.hits, seenBullet{sighting{e.Bot, e.From}, beat})
}

func (t *botTally) act(id string) { t.acted[id]++; t.bot(id) }

func (t *botTally) scan(beat int, e event.Scanned) {
	s := t.bot(e.Bot)
	if e.Enemy == 3 && s.FirstLineN == 0 {
		s.FirstLineN, s.FirstLineSum = 1, beat
	}
	if e.Bullet != 3 {
		return
	}
	k := sighting{e.Bot, e.Dir}
	if last, ok := t.lastSeen[k]; ok && beat-last <= dodgeWindow {
		return
	}
	t.lastSeen[k] = beat
	t.seen = append(t.seen, seenBullet{k, beat})
}

func (t *botTally) kill(beat int, e event.Killed) {
	t.bot(e.Victim).Deaths++
	t.diedAt[e.Victim] = beat
	if e.Killer != "" && e.Killer != e.Victim {
		t.bot(e.Killer).Kills++
	}
}

func (t *botTally) respawn(beat int, id string) {
	if died, ok := t.diedAt[id]; ok {
		t.deadBeats[id] += beat - died
		delete(t.diedAt, id)
	}
}

func (t *botTally) finish(beats int) map[string]BotStats {
	for id, died := range t.diedAt {
		t.deadBeats[id] += beats - died
	}
	for _, s := range t.seen {
		t.bot(s.bot).Seen3++
		if !t.hitWithin(s) {
			t.bot(s.bot).Dodged3++
		}
	}
	out := make(map[string]BotStats, len(t.stats))
	for id, s := range t.stats {
		s.AliveBeats = beats - t.deadBeats[id]
		s.IdleBeats = s.AliveBeats - t.acted[id]
		out[id] = *s
	}
	return out
}

func (t *botTally) hitWithin(s seenBullet) bool {
	for _, h := range t.hits {
		if h.sighting == s.sighting && h.beat >= s.beat && h.beat <= s.beat+dodgeWindow {
			return true
		}
	}
	return false
}

// Add sums two bots' stats.
func (s BotStats) Add(o BotStats) BotStats {
	return BotStats{
		Kills: s.Kills + o.Kills, Deaths: s.Deaths + o.Deaths, Coins: s.Coins + o.Coins,
		Shots: s.Shots + o.Shots, Hits: s.Hits + o.Hits, Clicks: s.Clicks + o.Clicks,
		AliveBeats: s.AliveBeats + o.AliveBeats, IdleBeats: s.IdleBeats + o.IdleBeats,
		Seen3: s.Seen3 + o.Seen3, Dodged3: s.Dodged3 + o.Dodged3,
		FirstLineSum: s.FirstLineSum + o.FirstLineSum, FirstLineN: s.FirstLineN + o.FirstLineN,
	}
}

// Accuracy is landed shots per shot fired.
func (s BotStats) Accuracy() float64 { return ratio(s.Hits, s.Shots) }

// IdleShare is the share of alive beats without an action.
func (s BotStats) IdleShare() float64 { return ratio(s.IdleBeats, s.AliveBeats) }

// DodgeRate is the share of level-3 bullet readings that did not end in a hit.
func (s BotStats) DodgeRate() float64 { return ratio(s.Dodged3, s.Seen3) }

// AvgLife is alive beats per death (a bot that never died: its alive beats).
func (s BotStats) AvgLife() float64 { return ratio(s.AliveBeats, max(s.Deaths, 1)) }

// AvgFirstLine is the mean beat of the first level-3 enemy reading.
func (s BotStats) AvgFirstLine() float64 { return ratio(s.FirstLineSum, s.FirstLineN) }
