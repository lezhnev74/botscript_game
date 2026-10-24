package sim

import (
	"slices"
	"testing"

	"github.com/lezhnev/arena/engine/domain/brain"
	"github.com/lezhnev/arena/engine/domain/event"
	"github.com/lezhnev/arena/engine/domain/geo"
)

const corridor = "#########\n#A......#\n#########\n"

func corridorWorld(t *testing.T) World {
	t.Helper()
	return NewWorld(mustMap(t, corridor), DefaultRules(), 1)
}

func bot(w World, id string) Bot {
	b, _ := w.bot(id)
	return b
}

func resolveAll(w World, acts map[string]brain.Action) (World, []event.Event) {
	return Resolve(w, acts, DefaultRules())
}

func pos(x, y int) geo.Pos { return geo.Pos{X: x, Y: y} }

func TestBulletTravelsSpeedCellsStartingAtNeighbour(t *testing.T) {
	next, _ := resolveA(corridorWorld(t), fr(geo.E))
	want := []Bullet{{Pos: pos(4, 1), Dir: geo.E, Owner: "A"}}
	if !slices.Equal(next.Bullets, want) {
		t.Errorf("bullets %+v, want %+v (1 + 3 cells)", next.Bullets, want)
	}
}

func TestBulletSpeedIsARule(t *testing.T) {
	r := DefaultRules()
	r.BulletSpeed = 1
	next, _ := Resolve(corridorWorld(t), map[string]brain.Action{"A": fr(geo.E)}, r)
	if next.Bullets[0].Pos != pos(2, 1) {
		t.Errorf("bullet at %v, want (2,1)", next.Bullets[0].Pos)
	}
}

func TestBulletKeepsFlyingNextBeat(t *testing.T) {
	w, _ := resolveA(corridorWorld(t), fr(geo.E))
	next, evs := resolveAll(w, nil)
	if len(evs) != 0 || len(next.Bullets) != 1 || next.Bullets[0].Pos != pos(7, 1) {
		t.Errorf("events %v bullets %+v, want one bullet at (7,1)", evs, next.Bullets)
	}
}

func TestBulletVanishesAtWall(t *testing.T) {
	w, _ := resolveA(corridorWorld(t), fr(geo.E))
	w, _ = resolveAll(w, nil) // at (7,1)
	next, evs := resolveAll(w, nil)
	if len(next.Bullets) != 0 || len(evs) != 0 {
		t.Errorf("bullets %+v events %v, want the bullet gone silently", next.Bullets, evs)
	}
}

func TestBulletNeverHitsItsOwnerAtSpawn(t *testing.T) {
	next, evs := resolveA(corridorWorld(t), fr(geo.W)) // wall right behind: gone at once
	if len(next.Bullets) != 0 || !equal(evs, []string{"Fired A W"}) || botA(next).HP != DefaultRules().HP {
		t.Errorf("events %v bullets %v hp %d", evs, next.Bullets, botA(next).HP)
	}
}

func duel(t *testing.T) World {
	t.Helper()
	return NewWorld(mustMap(t, "#########\n#A..B...#\n#########\n"), DefaultRules(), 1)
}

func TestBulletHitsFirstBotOnItsPath(t *testing.T) {
	w := duel(t)
	w.Bots = append(w.Bots, Bot{ID: "C", Pos: pos(6, 1), HP: 3})
	next, evs := resolveAll(w, map[string]brain.Action{"A": fr(geo.E)})
	if !equal(texts(evs), []string{"Fired A E", "Hit B W"}) {
		t.Fatalf("events %v", texts(evs))
	}
	if bot(next, "B").HP != 3 || bot(next, "C").HP != 3 || len(next.Bullets) != 0 {
		t.Errorf("hp B=%d C=%d bullets %v", bot(next, "B").HP, bot(next, "C").HP, next.Bullets)
	}
}

func TestHitNamesTheShooterInItsPayload(t *testing.T) {
	_, evs := resolveAll(duel(t), map[string]brain.Action{"A": fr(geo.E)})
	h := evs[1].(event.Hit)
	line, err := event.JSONLine(event.Logged{Beat: 1, Event: h})
	if err != nil || h.Shooter != "A" || string(line) != `{"beat":1,"type":"Hit","args":["B","W"],"data":{"by":"A"}}` {
		t.Errorf("hit %+v json %s (%v)", h, line, err)
	}
}

func TestHitFromDirectionIsWhereTheBulletCameFrom(t *testing.T) {
	const m = "#####\n#.N.#\n#W.E#\n#.S.#\n#####\n"
	cases := []struct {
		shooter string
		fire    geo.Dir
		victim  string
		from    string
	}{
		{"W", geo.E, "E", "W"},
		{"E", geo.W, "W", "E"},
		{"N", geo.S, "S", "N"},
		{"S", geo.N, "N", "S"},
	}
	for _, c := range cases {
		w := NewWorld(mustMap(t, m), DefaultRules(), 1)
		_, evs := resolveAll(w, map[string]brain.Action{c.shooter: fr(c.fire)})
		if !slices.Contains(texts(evs), "Hit "+c.victim+" "+c.from) {
			t.Errorf("fire %v: events %v, want Hit %s %s", c.fire, texts(evs), c.victim, c.from)
		}
	}
}

func TestTargetThatMovesOnTheBeatYouFireWins(t *testing.T) {
	w := NewWorld(mustMap(t, "#########\n#A..B...#\n#.......#\n#########\n"), DefaultRules(), 1)
	next, evs := resolveAll(w, map[string]brain.Action{"A": fr(geo.E), "B": mv(geo.S)})
	if slices.Contains(texts(evs), "Hit B W") || bot(next, "B").HP != 4 {
		t.Errorf("events %v hp %d", texts(evs), bot(next, "B").HP)
	}
	if len(next.Bullets) != 1 || next.Bullets[0].Pos != pos(4, 1) {
		t.Errorf("the bullet must fly on through the empty cell: %+v", next.Bullets)
	}
}

func TestBotWalkingIntoABulletCellIsHit(t *testing.T) {
	// A bullet rests at (3,1) flying E; B steps west into its cell.
	w := NewWorld(mustMap(t, "#########\n#A..B...#\n#########\n"), DefaultRules(), 1)
	w.Bullets = []Bullet{{Pos: pos(3, 1), Dir: geo.E, Owner: "A"}}
	w.Bots[1].Pos = pos(4, 1)
	w.Bots[0].Pos = pos(1, 1)
	next, evs := resolveAll(w, map[string]brain.Action{"B": mv(geo.W)})
	// B must not slip past the bullet.
	if !slices.Contains(texts(evs), "Hit B W") || bot(next, "B").HP != 3 {
		t.Errorf("events %v hp %d", texts(evs), bot(next, "B").HP)
	}
}

func TestBulletsPassThroughEachOther(t *testing.T) {
	w := NewWorld(mustMap(t, "#########\n#A.....B#\n#########\n"), DefaultRules(), 1)
	next, _ := resolveAll(w, map[string]brain.Action{"A": fr(geo.E), "B": fr(geo.W)})
	want := []Bullet{{Pos: pos(4, 1), Dir: geo.E, Owner: "A"}, {Pos: pos(4, 1), Dir: geo.W, Owner: "B"}}
	if !slices.Equal(next.Bullets, want) {
		t.Errorf("bullets %+v, want %+v", next.Bullets, want)
	}
}

func TestTwoBulletsOnOneBotBothHit(t *testing.T) {
	w := NewWorld(mustMap(t, "#######\n#A.B.C#\n#######\n"), DefaultRules(), 1)
	next, evs := resolveAll(w, map[string]brain.Action{"A": fr(geo.E), "C": fr(geo.W)})
	if !equal(texts(evs), []string{"Fired A E", "Fired C W", "Hit B W", "Hit B E"}) {
		t.Fatalf("events %v", texts(evs))
	}
	b := bot(next, "B")
	if b.HP != 2 || !b.Last.Hit || b.Last.HitDir != geo.W {
		t.Errorf("B %+v, want hp 2, hit from the first bullet (W)", b)
	}
}

func TestHpNeverGoesBelowZero(t *testing.T) {
	w := duel(t)
	w.Bots[1].HP = 0
	next, _ := resolveAll(w, map[string]brain.Action{"A": fr(geo.E)})
	if bot(next, "B").HP != 0 {
		t.Errorf("hp %d", bot(next, "B").HP)
	}
}

func TestHitIsShownToTheVictimsNextDecide(t *testing.T) {
	next, _ := resolveAll(duel(t), map[string]brain.Action{"A": fr(geo.E)})
	p := next.Perception("B", 2)
	if !p.Last.Hit || p.Last.HitDir != geo.W || p.HP != 3 {
		t.Errorf("perception %+v", p)
	}
	if q := next.Perception("A", 2); q.Last.Hit {
		t.Errorf("shooter must not be marked hit: %+v", q.Last)
	}
}

func TestHitAndBumpOnTheSameBeatAreBothReported(t *testing.T) {
	w := NewWorld(mustMap(t, "#########\n#A..BC..#\n#########\n"), DefaultRules(), 1)
	next, _ := resolveAll(w, map[string]brain.Action{"A": fr(geo.E), "B": mv(geo.E)})
	l := bot(next, "B").Last
	if !l.Hit || !l.Bumped || l.HitDir != geo.W || l.BumpDir != geo.E {
		t.Errorf("last %+v", l)
	}
}

func TestHitFlagClearsOnTheNextBeat(t *testing.T) {
	next, _ := resolveAll(duel(t), map[string]brain.Action{"A": fr(geo.E)})
	after, _ := resolveAll(next, nil)
	if bot(after, "B").Last.Hit {
		t.Error("hit must only show for the beat after the hit")
	}
}

func TestBulletsLeavingTheGridAreRemoved(t *testing.T) {
	w := corridorWorld(t)
	w.Bullets = []Bullet{{Pos: pos(8, 1), Dir: geo.E}}
	next, _ := resolveAll(w, nil)
	if len(next.Bullets) != 0 {
		t.Errorf("bullets %+v", next.Bullets)
	}
}

func TestRenderDrawsBullets(t *testing.T) {
	w, _ := resolveA(corridorWorld(t), fr(geo.E))
	want := "#########\n#A..*...#\n#########\n"
	if got := w.Render(); got != want {
		t.Errorf("render\n%s\nwant\n%s", got, want)
	}
}
