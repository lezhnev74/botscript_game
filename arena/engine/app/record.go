package app

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/lezhnev/arena/engine/domain/brain"
	"github.com/lezhnev/arena/engine/domain/event"
	"github.com/lezhnev/arena/engine/domain/geo"
	"github.com/lezhnev/arena/engine/domain/sim"
	"github.com/lezhnev/arena/engine/lang"
)

// RecordVersion is the Record format version this code writes. DecodeRecord
// also reads older versions: a rule added after a document's version may be
// missing and reads as the value that version always used (addedRules); the
// record comes back as the current version.
//
// Version 4 (not yet released) also carries the language v2 rewrite and negative
// directions as waits, and scans after bullet flight (a scan sees only bullets
// still in the air after the beat's flight): none of them has a rule key, and a record always holds
// every rule, so a changed Rules default (coin_every 50) never touches an
// existing record. Records of versions 1-3 have no hear_range (read as 0,
// deaf). Version 5 adds Edit.Reboot (a program saved while the bot is in a game takes it off the map);
// older records have none (read as 0, a plain hot-swap), so they need no rewrite. A "ping" action never shipped in a released record (it is gone again).
const RecordVersion = 5

// addedRule is a rule key that older Record versions lack: the first version
// that has it, and the value documents before that meant.
type addedRule struct{ since, old int }

var addedRules = map[string]addedRule{
	"cone_slope": {since: 2, old: 100}, // version 1 always used the 90 degree wedge
	"coin_heal":  {since: 3, old: 0},   // coins did not heal before version 3
	"hear_range": {since: 4, old: 0},   // gunfire was silent before version 4
}

// Brain kinds a Record can name: a fixed action list, or a BOTSCRIPT program.
const (
	KindScript  = "script"
	KindProgram = "program"
)

// BrainSpec describes a brain as data so a Record can rebuild it: Kind picks
// the implementation, Src is its source ("fire E, move S" for a script, the
// program text for BOTSCRIPT).
type BrainSpec struct {
	Kind string `json:"kind"`
	Src  string `json:"src"`
}

// BotSpec binds a bot id from the map to its brain. A map bot without a spec
// waits forever.
type BotSpec struct {
	ID    string    `json:"id"`
	Brain BrainSpec `json:"brain"`
}

// StartSpec overrides a bot's initial state (hp, bonus, cooldown, shield).
type StartSpec struct {
	Bot   string `json:"bot"`
	Field string `json:"field"`
	Value int    `json:"value"`
}

// BulletSpec is a bullet in flight before beat 1; Dir is N, E, S or W.
type BulletSpec struct {
	X     int    `json:"x"`
	Y     int    `json:"y"`
	Dir   string `json:"dir"`
	Owner string `json:"owner,omitempty"`
}

// Edit, Join and Leave are the live-session inputs (hot-swap, players coming
// and going). All three take effect at the start of their Beat, in the order
// leaves, joins, edits (each list in record order). Edits are program bots
// only: the brain keeps its memory, trail and the bot its HP and reload. A join
// stands on the respawn spot, drawn from the world's seeded RNG, so the spot
// is reproduced by the replay and needs no recording. Join and Leave ids are
// not map letters.
type (
	// Edit swaps a bot's program at the start of Beat. With Reboot > 0 the bot
	// also leaves the map for that many beats (see Match.Reboot); an edit of a
	// bot that is already rebooting only replaces the program.
	Edit struct {
		Beat   int       `json:"beat"`
		Bot    string    `json:"bot"`
		Brain  BrainSpec `json:"brain"`
		Reboot int       `json:"reboot,omitempty"`
	}
	// Join adds a bot at the start of Beat.
	Join struct {
		Beat  int       `json:"beat"`
		Bot   string    `json:"bot"`
		Brain BrainSpec `json:"brain"`
	}
	// Leave removes a bot at the start of Beat.
	Leave struct {
		Beat int    `json:"beat"`
		Bot  string `json:"bot"`
		// Kicked marks an admin removal: the match logs Kicked after Left, and
		// the leaderboard drops the bot's line. Absent in every older record.
		Kicked bool `json:"kicked,omitempty"`
	}
)

// Record is everything needed to rerun a match from scratch: same record,
// same event log, same hash. Map is the ASCII map text (bots as letters).
type Record struct {
	Version int          `json:"version"`
	Rules   sim.Rules    `json:"rules"`
	Map     string       `json:"map"`
	Seed    uint64       `json:"seed,string"` // a string: JS numbers lose precision above 2^53
	Beats   int          `json:"beats"`
	Bots    []BotSpec    `json:"bots"`
	Start   []StartSpec  `json:"start,omitempty"`
	Bullets []BulletSpec `json:"bullets,omitempty"`
	EditLog []Edit       `json:"edit_log,omitempty"`
	Joins   []Join       `json:"joins,omitempty"`
	Leaves  []Leave      `json:"leaves,omitempty"`
}

// DecodeRecord reads a Record document strictly: unknown keys, a missing or
// unsupported version and a missing rules key are all errors, so a typo cannot
// silently become a zero rule. An older document may lack the rules added
// after its version and is upgraded (see RecordVersion).
func DecodeRecord(data []byte) (Record, error) {
	var probe struct {
		Version int                        `json:"version"`
		Rules   map[string]json.RawMessage `json:"rules"`
	}
	if err := json.Unmarshal(data, &probe); err != nil {
		return Record{}, fmt.Errorf("record: %w", err)
	}
	if err := checkRuleKeys(probe.Version, probe.Rules); err != nil {
		return Record{}, err
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var rec Record
	if err := dec.Decode(&rec); err != nil {
		return Record{}, fmt.Errorf("record: %w", err)
	}
	rec = upgrade(rec, probe.Rules)
	return rec, rec.checkVersion()
}

// checkRuleKeys wants every rule present, except rules added after version.
func checkRuleKeys(version int, rules map[string]json.RawMessage) error {
	for _, k := range sim.RuleKeys() {
		if _, ok := rules[k]; !ok && !addedSince(k, version) {
			return fmt.Errorf("record: rules.%s is missing", k)
		}
	}
	return nil
}

// addedSince reports whether rule k is newer than a version document.
func addedSince(k string, version int) bool {
	a, ok := addedRules[k]
	return ok && version < a.since
}

// upgrade fills the rules an older document lacks with what they meant then
// and marks it current. A current or unknown version is returned as is.
func upgrade(rec Record, present map[string]json.RawMessage) Record {
	if rec.Version < 1 || rec.Version >= RecordVersion {
		return rec
	}
	for k, a := range addedRules {
		if _, ok := present[k]; !ok && rec.Version < a.since {
			_ = rec.Rules.Set(k, a.old) // old values are valid by construction (TestAddedRulesAreValid)
		}
	}
	rec.Version = RecordVersion
	return rec
}

func (rec Record) checkVersion() error {
	if rec.Version != RecordVersion {
		return fmt.Errorf("record: version %d is not supported (want %d)", rec.Version, RecordVersion)
	}
	return nil
}

// SpecError is a bad start override or bullet; Index is its position in
// Record.Start or Record.Bullets.
type SpecError struct {
	Section string // "start" or "bullet"
	Index   int
	Err     error
}

func (e *SpecError) Error() string { return fmt.Sprintf("record %s %d: %v", e.Section, e.Index, e.Err) }

// Unwrap exposes the cause.
func (e *SpecError) Unwrap() error { return e.Err }

// Replay reruns the record from scratch and returns its event log.
func Replay(rec Record) ([]event.Logged, error) {
	m, err := rec.NewMatch()
	if err != nil {
		return nil, err
	}
	defer m.Close()
	var log []event.Logged
	for i := 0; i < rec.Beats; i++ {
		log = append(log, m.Step()...)
		if err := m.Err(); err != nil {
			return log, fmt.Errorf("beat %d: %w", i+1, err)
		}
	}
	return log, nil
}

// NewMatch builds the match at beat 0 described by the record.
func (rec Record) NewMatch() (*Match, error) {
	if err := rec.checkVersion(); err != nil {
		return nil, err
	}
	m, err := geo.ParseMap(rec.Map)
	if err != nil {
		return nil, fmt.Errorf("record map: %w", err)
	}
	w, err := rec.world(m)
	if err != nil {
		return nil, err
	}
	brains, err := rec.brains(m)
	if err != nil {
		return nil, err
	}
	match := NewMatch(w, rec.Rules, brains)
	if err := rec.schedule(match, m); err != nil {
		match.Close()
		return nil, err
	}
	return match, nil
}

func (rec Record) world(m geo.Map) (sim.World, error) {
	w := sim.NewWorld(m, rec.Rules, rec.Seed)
	for i, s := range rec.Start {
		if err := rec.applyStart(w, s); err != nil {
			return w, &SpecError{Section: "start", Index: i, Err: err}
		}
	}
	for i, b := range rec.Bullets {
		bullet, err := rec.bullet(m, b)
		if err != nil {
			return w, &SpecError{Section: "bullet", Index: i, Err: err}
		}
		w.Bullets = append(w.Bullets, bullet)
	}
	return w, nil
}

func (rec Record) applyStart(w sim.World, s StartSpec) error {
	if s.Field == "bonus" && s.Value > rec.Rules.BonusMax {
		return fmt.Errorf("start %s: bonus %d exceeds bonus_max %d", s.Bot, s.Value, rec.Rules.BonusMax)
	}
	if err := w.SetBot(s.Bot, s.Field, s.Value); err != nil {
		return fmt.Errorf("start %s: %w", s.Bot, err)
	}
	return nil
}

func (rec Record) bullet(m geo.Map, b BulletSpec) (sim.Bullet, error) {
	d, ok := geo.ParseDir(b.Dir)
	if !ok {
		return sim.Bullet{}, fmt.Errorf("bullet: bad direction %q", b.Dir)
	}
	p := geo.Pos{X: b.X, Y: b.Y}
	if m.Grid.Blocked(p) {
		return sim.Bullet{}, fmt.Errorf("bullet at %d,%d is not on a floor cell", b.X, b.Y)
	}
	if b.Owner != "" && !hasBot(m, b.Owner) {
		return sim.Bullet{}, fmt.Errorf("bullet owner: no bot %q on the map", b.Owner)
	}
	if botStartsAt(m, p) {
		return sim.Bullet{}, fmt.Errorf("bullet at %d,%d starts on a bot", b.X, b.Y)
	}
	return sim.Bullet{Pos: p, Dir: d, Owner: b.Owner}, nil
}

func botStartsAt(m geo.Map, p geo.Pos) bool {
	for _, b := range m.Bots {
		if b.Pos == p {
			return true
		}
	}
	return false
}

func hasBot(m geo.Map, id string) bool {
	if len(id) != 1 {
		return false
	}
	_, ok := m.BotPos(id[0])
	return ok
}

func (rec Record) brains(m geo.Map) (map[string]brain.Brain, error) {
	brains := make(map[string]brain.Brain, len(m.Bots))
	for _, b := range m.Bots {
		brains[string(b.Letter)] = brain.NewScripted(nil)
	}
	listed := make(map[string]bool, len(rec.Bots))
	for _, spec := range rec.Bots {
		if !hasBot(m, spec.ID) {
			return nil, fmt.Errorf("record bot %q: not on the map", spec.ID)
		}
		if listed[spec.ID] {
			return nil, fmt.Errorf("record bot %q: listed twice", spec.ID)
		}
		listed[spec.ID] = true
		br, err := NewBrain(spec.Brain, rec.Seed, spec.ID)
		if err != nil {
			return nil, fmt.Errorf("record bot %s: %w", spec.ID, err)
		}
		brains[spec.ID] = br
	}
	return brains, nil
}

// NewBrain builds the brain a BrainSpec describes. A program's dice are
// seeded with lang.SeedFor(matchSeed, botID); scripts ignore both. Brains
// that hold a goroutine have a Close method (Match.Close calls it).
func NewBrain(s BrainSpec, matchSeed uint64, botID string) (brain.Brain, error) {
	switch s.Kind {
	case KindScript:
		return newScripted(s.Src)
	case KindProgram:
		return newProgram(s.Src, lang.SeedFor(matchSeed, botID))
	}
	return nil, fmt.Errorf("unknown brain kind %q (want %q or %q)", s.Kind, KindScript, KindProgram)
}

func newScripted(src string) (brain.Brain, error) {
	acts, err := brain.ParseScript(src)
	if err != nil {
		return nil, fmt.Errorf("script: %w", err)
	}
	return brain.NewScripted(acts), nil
}

func newProgram(src string, seed uint64) (brain.Brain, error) {
	p, err := compile(src)
	if err != nil {
		return nil, err
	}
	return lang.NewScriptBrain(p, seed), nil
}

// compile validates BOTSCRIPT source; the error lists every problem.
func compile(src string) (*lang.Program, error) {
	p, errs := lang.Validate(src)
	if len(errs) > 0 {
		return nil, fmt.Errorf("program: %w", errors.Join(errsOf(errs)...))
	}
	return p, nil
}

func errsOf(errs []lang.Error) []error {
	out := make([]error, len(errs))
	for i, e := range errs {
		out[i] = e
	}
	return out
}
