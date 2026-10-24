package app

import (
	"encoding/json"
	"strings"
	"testing"
)

// rebootRecord is editRecord with the edit as a 3-beat reboot at beat 3.
func rebootRecord() Record {
	rec := editRecord()
	rec.Beats = 8
	rec.EditLog[0].Beat = 3
	rec.EditLog[0].Reboot = 3
	return rec
}

func (m *Match) wantNum(t *testing.T, bot, name string, want int) {
	t.Helper()
	if got := m.Probes()[bot+"."+name]; got != want {
		t.Errorf("%s.%s = %d, want %d", bot, name, got, want)
	}
}

func TestRebootTakesTheBotOffTheMapForItsBeats(t *testing.T) {
	m, err := rebootRecord().NewMatch()
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	stepN(m, 2)
	m.wantNum(t, "A", "num_0", 2)
	stepN(m, 1) // beat 3: the reboot begins, the bot is off the map
	if !m.World().IsRebooting("A") || len(m.World().Bots) != 1 {
		t.Fatalf("rebooting %v, bots %d", m.World().IsRebooting("A"), len(m.World().Bots))
	}
	if got := moved(stepN(m, 2), "A"); len(got) != 0 || m.World().IsRebooting("A") {
		t.Errorf("beats 4-5: A moved %v, rebooting %v (want back on the map, not acted yet)", got, m.World().IsRebooting("A"))
	}
}

func TestARebootedBotRunsTheNewProgramAndKeepsMemory(t *testing.T) {
	m, err := rebootRecord().NewMatch()
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	stepN(m, 5)
	got := moved(stepN(m, 1), "A") // beat 6: it acts again, running the mover
	if len(got) != 1 || got[0] != 6 {
		t.Errorf("A moved on %v, want beat 6", got)
	}
	m.wantNum(t, "A", "num_0", 2)
}

func TestASecondEditDuringARebootReplacesTheProgramWithoutRestartingTheCountdown(t *testing.T) {
	rec := rebootRecord()
	rec.EditLog = append(rec.EditLog, Edit{Beat: 4, Bot: "A", Reboot: 3, Brain: BrainSpec{Kind: KindProgram, Src: "function loop() { num[1] = 9; wait(); }"}})
	m, err := rec.NewMatch()
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	stepN(m, 5)
	if m.World().IsRebooting("A") {
		t.Fatal("A should be back on beat 5, the countdown is not restarted")
	}
	stepN(m, 1)
	if got := m.Probes()["A.num_1"]; got != 9 {
		t.Errorf("num[1] = %d, want 9: the later program should run", got)
	}
}

func TestRebootEditsReplayAndSurviveJSON(t *testing.T) {
	data, err := json.Marshal(rebootRecord())
	if err != nil || !strings.Contains(string(data), `"reboot":3`) {
		t.Fatalf("%v %s", err, data)
	}
	back, err := DecodeRecord(data)
	if err != nil {
		t.Fatal(err)
	}
	plain := rebootRecord()
	plain.EditLog[0].Reboot = 0
	if a, b := hashOf(t, rebootRecord()), hashOf(t, back); a != b {
		t.Errorf("round trip changed the replay: %s vs %s", a, b)
	}
	if hashOf(t, plain) == hashOf(t, rebootRecord()) {
		t.Error("the reboot changed nothing against a hot-swap")
	}
}

func TestOlderRecordsWithoutRebootsStillDecode(t *testing.T) {
	data, err := json.Marshal(editRecord())
	if err != nil {
		t.Fatal(err)
	}
	old := strings.Replace(string(data), `"version":5`, `"version":4`, 1)
	rec, err := DecodeRecord([]byte(old))
	if err != nil || rec.Version != RecordVersion || rec.EditLog[0].Reboot != 0 {
		t.Errorf("version %d reboot %d err %v", rec.Version, rec.EditLog[0].Reboot, err)
	}
}

func TestANegativeRebootIsRejected(t *testing.T) {
	rec := rebootRecord()
	rec.EditLog[0].Reboot = -1
	if _, err := Replay(rec); err == nil || !strings.Contains(err.Error(), "edit_log[0]") {
		t.Errorf("err = %v", err)
	}
}

func TestRebootOfAnUnknownBotFails(t *testing.T) {
	m, err := rebootRecord().NewMatch()
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	if err := m.Reboot("Z", nil, 3); err == nil {
		t.Error("rebooted a bot that is not in the arena")
	}
}

func TestLeavingDuringARebootCancelsIt(t *testing.T) {
	rec := rebootRecord()
	rec.Leaves = []Leave{{Beat: 4, Bot: "A"}}
	m, err := rec.NewMatch()
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	stepN(m, 4)
	if m.World().IsRebooting("A") || len(m.World().Bots) != 1 {
		t.Errorf("A still rebooting after it left: %+v", m.World().Rebooting)
	}
}
