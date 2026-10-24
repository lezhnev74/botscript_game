package sim

import (
	"testing"

	"github.com/lezhnev/arena/engine/domain/geo"
)

func rebootWorld(t *testing.T) World {
	t.Helper()
	w := NewWorld(mustMap(t, "#######\n#A...B#\n#######\n"), DefaultRules(), 1)
	w.Bots[0].HP, w.Bots[0].Cooldown, w.Bots[0].Bonus = 2, 1, 1
	return w
}

func TestRebootTakesTheBotOffTheGrid(t *testing.T) {
	w := rebootWorld(t)
	next, ok := w.Reboot("A", 3)
	if !ok || len(next.Bots) != 1 || next.Bots[0].ID != "B" || len(w.Bots) != 2 {
		t.Fatalf("ok %v bots %+v (input bots %d)", ok, next.Bots, len(w.Bots))
	}
	if r := next.Rebooting[0]; r != (Rebooting{Bot: w.Bots[0], Left: 3, Total: 3}) {
		t.Errorf("rebooting %+v", r)
	}
}

func TestRebootedBotCannotBeSeenHitOrBumped(t *testing.T) {
	w, _ := rebootWorld(t).Reboot("A", 5)
	free := NewWorld(mustMap(t, "#######\n#A...B#\n#######\n"), DefaultRules(), 1)
	w.Bullets = []Bullet{{Pos: free.Bots[0].Pos, Dir: 1, Owner: "B"}}
	next, evs := resolveAll(w, nil)
	if len(evs) != 0 || len(next.Bots) != 1 {
		t.Errorf("events %v, bots %+v", evs, next.Bots)
	}
	if free2, _ := resolveAll(next, nil); free2.Rebooting[0].Left != 3 {
		t.Errorf("left %d, want 3 after two beats", free2.Rebooting[0].Left)
	}
}

func TestRebootEndsAfterNBeatsKeepingHPGunAndBonus(t *testing.T) {
	w, _ := rebootWorld(t).Reboot("A", 2)
	w, _ = resolveAll(w, nil)
	if alive(w, "A") || w.Rebooting[0].Left != 1 {
		t.Fatalf("after one beat: alive %v left %d", alive(w, "A"), w.Rebooting[0].Left)
	}
	w, _ = resolveAll(w, nil)
	b, ok := w.bot("A")
	if !ok || len(w.Rebooting) != 0 {
		t.Fatalf("after two beats: A back %v, rebooting %v", ok, w.Rebooting)
	}
	want := Bot{ID: "A", Pos: b.Pos, HP: 2, Cooldown: 1, Bonus: 1, Shield: DefaultRules().ShieldBeats, Life: 1, Last: b.Last}
	if b != want || !b.Last.OK {
		t.Errorf("returned bot %+v, want %+v", b, want)
	}
}

func TestRebootOfOneBeatIsOneBeatOffTheMap(t *testing.T) {
	w, _ := rebootWorld(t).Reboot("A", 1)
	if w, _ = resolveAll(w, nil); !alive(w, "A") {
		t.Error("A should be back after one beat")
	}
}

func TestRebootDoesNotRestart(t *testing.T) {
	w, _ := rebootWorld(t).Reboot("A", 4)
	w, _ = resolveAll(w, nil)
	next, ok := w.Reboot("A", 4)
	if ok || next.Rebooting[0].Left != 3 {
		t.Errorf("ok %v left %d", ok, next.Rebooting[0].Left)
	}
}

func deadWorld(t *testing.T, wait int) World {
	t.Helper()
	w := rebootWorld(t)
	w.Fallen = []Fallen{{Bot: Bot{ID: "C", Pos: w.Bots[0].Pos}, Wait: wait}}
	return w
}

func TestRebootOfADeadBotWaitsForTheLaterOfRespawnAndReboot(t *testing.T) {
	long, ok := deadWorld(t, 5).Reboot("C", 3)
	if !ok || len(long.Fallen) != 0 || long.Rebooting[0].Left != 6 || !long.Rebooting[0].Dead {
		t.Errorf("long death: %v %+v", ok, long.Rebooting)
	}
	short, _ := deadWorld(t, 0).Reboot("C", 3)
	if short.Rebooting[0].Left != 3 {
		t.Errorf("long reboot: left %d", short.Rebooting[0].Left)
	}
}

func TestADeadBotComesBackFromItsRebootAsAFullHPRespawn(t *testing.T) {
	w, _ := deadWorld(t, 0).Reboot("C", 3)
	for range 3 {
		w, _ = resolveAll(w, nil)
	}
	b, ok := w.bot("C")
	if !ok || b.HP != DefaultRules().HP {
		t.Errorf("dead bot back %v with %+v, want a full-HP respawn", ok, b)
	}
}

func TestRebootNeedsALiveOrFallenBot(t *testing.T) {
	if _, ok := rebootWorld(t).Reboot("Z", 3); ok {
		t.Error("rebooted a bot that is not there")
	}
}

func TestRebootingBotWithNoFreeCellStaysOffTheMap(t *testing.T) {
	w := NewWorld(mustMap(t, "####\n#AB#\n####\n"), DefaultRules(), 1)
	w.Coins = []geo.Pos{w.Bots[0].Pos} // the only free cell holds a coin
	w, _ = w.Reboot("A", 1)
	w, _ = resolveAll(w, nil)
	if alive(w, "A") || len(w.Rebooting) != 1 || w.Rebooting[0].Left != 0 {
		t.Errorf("A back %v rebooting %+v", alive(w, "A"), w.Rebooting)
	}
}

func TestQueryReadsARebootingBot(t *testing.T) {
	w, _ := rebootWorld(t).Reboot("A", 3)
	for spec, want := range map[string]int{"A.hp": 2, "A.alive": 0, "A.rebooting": 3, "A.x": 1, "B.rebooting": 0} {
		if got, err := w.Query(spec); err != nil || got != want {
			t.Errorf("%s = %d, %v; want %d", spec, got, err, want)
		}
	}
}

func TestRemoveDropsARebootingBot(t *testing.T) {
	w, _ := rebootWorld(t).Reboot("A", 3)
	if next := w.Remove("A"); len(next.Rebooting) != 0 || w.IsRebooting("A") == false {
		t.Errorf("after remove %+v (input %+v)", next.Rebooting, w.Rebooting)
	}
}
