package sim

import (
	"fmt"
	"maps"
	"slices"
)

// Rules is the tunable game configuration; no magic numbers live in phases.
type Rules struct {
	BulletSpeed  int `json:"bullet_speed"` // cells per beat
	Reload       int `json:"reload"`       // beats between loaded shots
	HP           int `json:"hp"`
	BonusMax     int `json:"bonus_max"` // max stored bonus shots
	RespawnBeats int `json:"respawn_beats"`
	ShieldBeats  int `json:"shield_beats"`
	CoinEvery    int `json:"coin_every"` // a coin spawns every this many beats while below CoinMax; 0 = never
	CoinMax      int `json:"coin_max"`   // most coins on the map at once
	CoinHeal     int `json:"coin_heal"`  // HP a coin restores, capped at HP
	ConeSlope    int `json:"cone_slope"` // scan cone: most perpendicular offset per cell along, in percent (100 = 90 degree wedge)
	HearRange    int `json:"hear_range"` // a shot is heard by bots within this Manhattan distance; 0 = deaf world
}

// DefaultRules are the VISION defaults.
func DefaultRules() Rules {
	return Rules{BulletSpeed: 3, Reload: 3, HP: 4, BonusMax: 2, RespawnBeats: 5, ShieldBeats: 2, CoinEvery: 50, CoinMax: 5, CoinHeal: 1, ConeSlope: 100, HearRange: 6}
}

// ruleField locates a rule in Rules and gives its smallest sensible value.
type ruleField struct {
	ptr func(*Rules) *int
	min int
}

var ruleFields = map[string]ruleField{
	"bullet_speed":  {func(r *Rules) *int { return &r.BulletSpeed }, 1},
	"reload":        {func(r *Rules) *int { return &r.Reload }, 1},
	"hp":            {func(r *Rules) *int { return &r.HP }, 1},
	"bonus_max":     {func(r *Rules) *int { return &r.BonusMax }, 0},
	"respawn_beats": {func(r *Rules) *int { return &r.RespawnBeats }, 1},
	"shield_beats":  {func(r *Rules) *int { return &r.ShieldBeats }, 0},
	"coin_every":    {func(r *Rules) *int { return &r.CoinEvery }, 0},
	"coin_max":      {func(r *Rules) *int { return &r.CoinMax }, 0},
	"coin_heal":     {func(r *Rules) *int { return &r.CoinHeal }, 0},
	"cone_slope":    {func(r *Rules) *int { return &r.ConeSlope }, 0},
	"hear_range":    {func(r *Rules) *int { return &r.HearRange }, 0},
}

// Set overrides one rule by its scenario key (e.g. "bullet_speed"). A value
// below the rule's minimum (a bullet that never moves, no HP, a negative
// count) is an error and changes nothing.
func (r *Rules) Set(key string, v int) error {
	f, ok := ruleFields[key]
	if !ok {
		return fmt.Errorf("unknown rule %q", key)
	}
	if v < f.min {
		return fmt.Errorf("rule %s=%d: must be at least %d", key, v, f.min)
	}
	*f.ptr(r) = v
	return nil
}

// RuleKeys lists the rule names Set accepts, sorted. They are also the JSON
// keys of Rules.
func RuleKeys() []string { return slices.Sorted(maps.Keys(ruleFields)) }
