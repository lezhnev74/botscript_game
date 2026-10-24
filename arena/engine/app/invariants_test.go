package app

import (
	"strings"
	"testing"

	"github.com/lezhnev/arena/engine/domain/event"
	"github.com/lezhnev/arena/engine/domain/geo"
	"github.com/lezhnev/arena/engine/domain/sim"
)

func freshWorld(t *testing.T) (sim.World, sim.Rules) {
	t.Helper()
	r := sim.DefaultRules()
	return sim.NewWorld(mustMap(t, "#####\n#A.B#\n#...#\n#####\n"), r, 1), r
}

func TestInvariantsHoldOnAFreshWorld(t *testing.T) {
	w, r := freshWorld(t)
	if err := CheckInvariants(w, r, nil); err != nil {
		t.Error(err)
	}
}

func TestInvariantsCatchBrokenWorlds(t *testing.T) {
	cases := map[string]struct {
		mutate func(*sim.World)
		log    []event.Logged
		want   string
	}{
		"shared cell":       {mutate: func(w *sim.World) { w.Bots[1].Pos = w.Bots[0].Pos }, want: "share"},
		"hp too high":       {mutate: func(w *sim.World) { w.Bots[0].HP = sim.DefaultRules().HP + 1 }, want: "hp"},
		"hp zero":           {mutate: func(w *sim.World) { w.Bots[0].HP = 0 }, want: "hp"},
		"bonus over max":    {mutate: func(w *sim.World) { w.Bots[0].Bonus = 3 }, want: "bonus"},
		"bot in wall":       {mutate: func(w *sim.World) { w.Bots[0].Pos = geo.Pos{} }, want: "wall"},
		"bullet in wall":    {mutate: func(w *sim.World) { w.Bullets = []sim.Bullet{{Pos: geo.Pos{}}} }, want: "bullet"},
		"bullet outside":    {mutate: func(w *sim.World) { w.Bullets = []sim.Bullet{{Pos: geo.Pos{X: 9, Y: 9}}} }, want: "bullet"},
		"fallen on grid":    {mutate: func(w *sim.World) { w.Fallen = []sim.Fallen{{Bot: w.Bots[0]}} }, want: "fallen"},
		"rebooting on grid": {mutate: func(w *sim.World) { w.Rebooting = []sim.Rebooting{{Bot: w.Bots[0]}} }, want: "rebooting"},
		"unpaid death": {
			mutate: func(*sim.World) {},
			log:    []event.Logged{{Beat: 1, Event: event.Killed{Killer: "A", Victim: "B"}}},
			want:   "deaths",
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			w, r := freshWorld(t)
			c.mutate(&w)
			err := CheckInvariants(w, r, c.log)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Errorf("got %v, want a %q violation", err, c.want)
			}
		})
	}
}

func TestInvariantsCountDeathsAsRespawnedOrWaiting(t *testing.T) {
	w, r := freshWorld(t)
	dead := w.Bots[1]
	dead.HP = 0
	w.Bots = w.Bots[:1]
	w.Fallen = []sim.Fallen{{Bot: dead, Wait: 3}}
	log := []event.Logged{{Beat: 1, Event: event.Killed{Killer: "A", Victim: "B"}}}
	if err := CheckInvariants(w, r, log); err != nil {
		t.Error(err)
	}
}

func TestInvariantsCountADeadRebootingBotAsWaiting(t *testing.T) {
	w, r := freshWorld(t)
	dead := w.Bots[1]
	w.Bots = w.Bots[:1]
	w.Rebooting = []sim.Rebooting{{Bot: dead, Left: 3, Total: 3, Dead: true}}
	log := []event.Logged{{Beat: 1, Event: event.Killed{Killer: "A", Victim: "B"}}}
	if err := CheckInvariants(w, r, log); err != nil {
		t.Error(err)
	}
}
