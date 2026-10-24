package app

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/lezhnev/arena/engine/domain/sim"
)

func duel() Record {
	return Record{
		Version: RecordVersion,
		Rules:   sim.DefaultRules(),
		Map:     "#######\n#A...B#\n#.....#\n#######\n",
		Seed:    7,
		Beats:   6,
		Bots: []BotSpec{
			{ID: "A", Brain: BrainSpec{Kind: KindScript, Src: "fire E, move S, fire E"}},
			{ID: "B", Brain: BrainSpec{Kind: KindScript, Src: "move S, fire W"}},
		},
	}
}

func TestReplayPlaysTheRecord(t *testing.T) {
	log, err := Replay(duel())
	if err != nil {
		t.Fatal(err)
	}
	if len(log) == 0 || log[len(log)-1].Beat > 6 {
		t.Fatalf("log %v", log)
	}
	again, _ := Replay(duel())
	h1, _ := HashLog(log)
	h2, _ := HashLog(again)
	if h1 != h2 {
		t.Errorf("replay not repeatable: %s vs %s", h1, h2)
	}
}

func TestReplaySurvivesAJSONRoundTrip(t *testing.T) {
	rec := duel()
	rec.Start = []StartSpec{{Bot: "B", Field: "hp", Value: 1}}
	rec.Bullets = []BulletSpec{{X: 3, Y: 2, Dir: "E", Owner: "A"}}
	data, err := json.Marshal(rec)
	if err != nil {
		t.Fatal(err)
	}
	back, err := DecodeRecord(data)
	if err != nil {
		t.Fatal(err)
	}
	want, _ := Replay(rec)
	got, _ := Replay(back)
	h1, _ := HashLog(want)
	h2, _ := HashLog(got)
	if h1 != h2 || !strings.Contains(string(data), `"kind":"script"`) {
		t.Errorf("round trip changed the replay (%s) or format: %s", h1, data)
	}
}

func TestRecordAppliesStartAndBullets(t *testing.T) {
	rec := duel()
	rec.Start = []StartSpec{{Bot: "B", Field: "hp", Value: 1}, {Bot: "A", Field: "bonus", Value: 2}}
	rec.Bullets = []BulletSpec{{X: 3, Y: 2, Dir: "E", Owner: "A"}}
	m, err := rec.NewMatch()
	if err != nil {
		t.Fatal(err)
	}
	w := m.World()
	if hp, _ := w.Query("B.hp"); hp != 1 {
		t.Errorf("B.hp %d", hp)
	}
	if b, _ := w.Query("A.bonus"); b != 2 {
		t.Errorf("A.bonus %d", b)
	}
	if len(w.Bullets) != 1 || w.Bullets[0].Pos.X != 3 || w.Bullets[0].Owner != "A" {
		t.Errorf("bullets %+v", w.Bullets)
	}
}

func TestRecordBotsWithoutASpecWait(t *testing.T) {
	rec := duel()
	rec.Bots = rec.Bots[:1]
	log, err := Replay(rec)
	if err != nil {
		t.Fatal(err)
	}
	for _, l := range log {
		if strings.HasSuffix(l.Event.Name(), "Moved") && l.Event.Args()[0] == "B" {
			t.Errorf("B moved: %v", l)
		}
	}
}

func TestRecordErrors(t *testing.T) {
	cases := map[string]func(*Record){
		"bad map":        func(r *Record) { r.Map = "#A\n#" },
		"unknown kind":   func(r *Record) { r.Bots[0].Brain.Kind = "bogus" },
		"bad script":     func(r *Record) { r.Bots[0].Brain.Src = "jump N" },
		"unknown bot":    func(r *Record) { r.Bots = append(r.Bots, BotSpec{ID: "Z"}) },
		"bad start bot":  func(r *Record) { r.Start = []StartSpec{{Bot: "Z", Field: "hp", Value: 1}} },
		"bad start key":  func(r *Record) { r.Start = []StartSpec{{Bot: "A", Field: "luck", Value: 1}} },
		"bonus over max": func(r *Record) { r.Start = []StartSpec{{Bot: "A", Field: "bonus", Value: 9}} },
		"bullet in wall": func(r *Record) { r.Bullets = []BulletSpec{{X: 0, Y: 0, Dir: "E"}} },
		"bullet bad dir": func(r *Record) { r.Bullets = []BulletSpec{{X: 2, Y: 1, Dir: "Q"}} },
		"bullet on bot":  func(r *Record) { r.Bullets = []BulletSpec{{X: 1, Y: 1, Dir: "E"}} },
		"bullet owner":   func(r *Record) { r.Bullets = []BulletSpec{{X: 2, Y: 1, Dir: "E", Owner: "Z"}} },
		"dup bot id":     func(r *Record) { r.Bots = append(r.Bots, r.Bots[0]) },
		"version":        func(r *Record) { r.Version = 1 },
		"edit log":       func(r *Record) { r.EditLog = []Edit{{Beat: 2, Bot: "A"}} },
		"joins":          func(r *Record) { r.Joins = []Join{{Beat: 2, Bot: "C", Brain: BrainSpec{Kind: "bogus"}}} },
		"leaves":         func(r *Record) { r.Leaves = []Leave{{Beat: 2, Bot: "Z"}} },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			rec := duel()
			mutate(&rec)
			if _, err := Replay(rec); err == nil {
				t.Error("want an error")
			}
		})
	}
}

func encoded(t *testing.T, rec Record) map[string]any {
	t.Helper()
	data, err := json.Marshal(rec)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func decodeMap(m map[string]any) (Record, error) {
	data, _ := json.Marshal(m)
	return DecodeRecord(data)
}

func TestDecodeRecordRejectsBadDocuments(t *testing.T) {
	cases := map[string]func(map[string]any){
		"unknown top key":  func(m map[string]any) { m["bogus"] = 1 },
		"unknown rule key": func(m map[string]any) { m["rules"].(map[string]any)["bulet_speed"] = 3 },
		"missing rule key": func(m map[string]any) { delete(m["rules"].(map[string]any), "reload") },
		"v2 needs cone":    func(m map[string]any) { delete(m["rules"].(map[string]any), "cone_slope") },
		"missing rules":    func(m map[string]any) { delete(m, "rules") },
		"no version":       func(m map[string]any) { delete(m, "version") },
		"future version":   func(m map[string]any) { m["version"] = 99 },
		"seed as number":   func(m map[string]any) { m["seed"] = 7 },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			m := encoded(t, duel())
			mutate(m)
			if _, err := decodeMap(m); err == nil {
				t.Error("want an error")
			}
		})
	}
	if _, err := DecodeRecord([]byte("{")); err == nil {
		t.Error("want a syntax error")
	}
}

func TestSeedIsAJSONString(t *testing.T) {
	rec := duel()
	rec.Seed = 1<<63 + 12345
	m := encoded(t, rec)
	if m["seed"] != "9223372036854788153" {
		t.Errorf("seed encoded as %#v", m["seed"])
	}
	back, err := decodeMap(m)
	if err != nil || back.Seed != rec.Seed {
		t.Errorf("seed round trip: %d, %v", back.Seed, err)
	}
}

func TestRulesJSONKeysMatchTheRuleNames(t *testing.T) {
	rules := encoded(t, duel())["rules"].(map[string]any)
	keys := sim.RuleKeys()
	if len(rules) != len(keys) {
		t.Errorf("json has %d rule keys, sim has %d", len(rules), len(keys))
	}
	for _, k := range keys {
		var r sim.Rules
		if _, ok := rules[k]; !ok || r.Set(k, 1) != nil {
			t.Errorf("rule key %q missing from the JSON or from Rules.Set", k)
		}
	}
}

func TestStartAndBulletErrorsKnowTheirIndex(t *testing.T) {
	rec := duel()
	rec.Start = []StartSpec{{Bot: "A", Field: "hp", Value: 2}, {Bot: "Z", Field: "hp", Value: 1}}
	_, err := rec.NewMatch()
	var se *SpecError
	if !errors.As(err, &se) || se.Section != "start" || se.Index != 1 {
		t.Errorf("start error %v", err)
	}
	rec.Start = nil
	rec.Bullets = []BulletSpec{{X: 3, Y: 1, Dir: "E"}, {X: 0, Y: 0, Dir: "E"}}
	_, err = rec.NewMatch()
	if !errors.As(err, &se) || se.Section != "bullet" || se.Index != 1 {
		t.Errorf("bullet error %v", err)
	}
}

func TestDecodeRecordReadsVersion1WithoutConeSlopeAsAWedge(t *testing.T) {
	m := encoded(t, duel())
	m["version"] = 1
	delete(m["rules"].(map[string]any), "cone_slope")
	rec, err := decodeMap(m)
	if err != nil {
		t.Fatal(err)
	}
	if rec.Rules.ConeSlope != 100 || rec.Version != RecordVersion {
		t.Errorf("v1 record read as version %d cone_slope %d", rec.Version, rec.Rules.ConeSlope)
	}
	want, _ := Replay(duel())
	got, err := Replay(rec)
	if err != nil || len(got) != len(want) {
		t.Errorf("an upgraded v1 record replays like the original: %v", err)
	}
}

func TestDecodeRecordReadsVersion2WithoutCoinHealAsNoHealing(t *testing.T) {
	m := encoded(t, duel())
	m["version"] = 2
	delete(m["rules"].(map[string]any), "coin_heal")
	rec, err := decodeMap(m)
	if err != nil {
		t.Fatal(err)
	}
	if rec.Rules.CoinHeal != 0 || rec.Version != RecordVersion {
		t.Errorf("v2 record read as version %d coin_heal %d", rec.Version, rec.Rules.CoinHeal)
	}
}

func TestDecodeRecordVersion2NeedsConeSlope(t *testing.T) {
	m := encoded(t, duel())
	m["version"] = 2
	delete(m["rules"].(map[string]any), "cone_slope")
	if _, err := decodeMap(m); err == nil {
		t.Error("want an error")
	}
}

func TestDecodeRecordReadsVersion3WithoutHearRangeAsADeafWorld(t *testing.T) {
	m := encoded(t, duel())
	m["version"] = 3
	delete(m["rules"].(map[string]any), "hear_range")
	rec, err := decodeMap(m)
	if err != nil {
		t.Fatal(err)
	}
	if rec.Rules.HearRange != 0 || rec.Version != RecordVersion {
		t.Errorf("v3 record read as version %d hear_range %d", rec.Version, rec.Rules.HearRange)
	}
}

func TestDecodeRecordVersion4NeedsHearRange(t *testing.T) {
	m := encoded(t, duel())
	delete(m["rules"].(map[string]any), "hear_range")
	if _, err := decodeMap(m); err == nil {
		t.Error("want an error")
	}
}

func TestAddedRulesAreValid(t *testing.T) {
	for k, a := range addedRules {
		r := sim.DefaultRules()
		if err := r.Set(k, a.old); err != nil || a.since < 2 || a.since > RecordVersion {
			t.Errorf("%s %+v: %v", k, a, err)
		}
	}
}

func TestDecodeRecordVersion1StillRejectsOtherMissingRules(t *testing.T) {
	m := encoded(t, duel())
	m["version"] = 1
	delete(m["rules"].(map[string]any), "reload")
	if _, err := decodeMap(m); err == nil {
		t.Error("want an error")
	}
}
