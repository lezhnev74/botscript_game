package lang

import (
	"github.com/lezhnev/arena/engine/domain/brain"
	"github.com/lezhnev/arena/engine/domain/geo"
)

// scriptEnv is the Env of a ScriptBrain: the latest Perception and trail.
// The script goroutine reads it only while the driver is blocked.
type scriptEnv struct{ b *ScriptBrain }

var readingGetters = map[string]func(brain.Perception) int{
	"enemy":  func(p brain.Perception) int { return p.Read.Enemy },
	"coin":   func(p brain.Perception) int { return p.Read.Coin },
	"bullet": func(p brain.Perception) int { return p.Read.Bullet },
	"last":   func(p brain.Perception) int { return int(p.Read.Dir) },
	"hp":     func(p brain.Perception) int { return p.HP },
	"loaded": func(p brain.Perception) int { return b2i(p.Loaded) },
	"bonus":  func(p brain.Perception) int { return p.Bonus },
	"x":      func(p brain.Perception) int { return p.Pos.X },
	"y":      func(p brain.Perception) int { return p.Pos.Y },
	"WIDTH":  func(p brain.Perception) int { return p.Width },
	"HEIGHT": func(p brain.Perception) int { return p.Height },
}

func (e scriptEnv) Reading(name string) int { return readingGetters[name](e.b.p) }

func (e scriptEnv) TrailLen() int { return len(e.b.trail) }

func (e scriptEnv) TrailField(i int, field string) int {
	return trailGetters[field](e.b.trail[i])
}

func (e scriptEnv) Open(d int) bool { return e.b.p.Open[wrap(d)] }

func (e scriptEnv) Steps(x, y int) int { return e.b.navigator().Steps(e.b.p.Pos, geo.Pos{X: x, Y: y}) }

func (e scriptEnv) Path(x, y int) int { return e.b.navigator().Path(e.b.p.Pos, geo.Pos{X: x, Y: y}) }

func (e scriptEnv) Heard() Place {
	f := e.b.p.Heard
	return Place{X: f.Pos.X, Y: f.Pos.Y, Set: f.Set}
}

func (e scriptEnv) Wall(x, y int) bool { return e.b.p.Grid.Blocked(geo.Pos{X: x, Y: y}) }
