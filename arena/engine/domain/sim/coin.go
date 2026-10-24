package sim

import (
	"slices"

	"github.com/lezhnev/arena/engine/domain/event"
)

// coinPhase runs after hits: bots standing on a coin take it, then the
// spawn timer may drop a new one.
func coinPhase(c *beatCtx) {
	c.pickUpCoins()
	c.spawnCoin()
}

// pickUpCoins gives every bot on a coin one bonus shot (capped by BonusMax)
// and CoinHeal HP (capped by HP); the coin is consumed either way (it also earns
// leaderboard points, see app.Points). Emits CoinTaken.
func (c *beatCtx) pickUpCoins() {
	for i := range c.w.Bots {
		b := &c.w.Bots[i]
		at := slices.Index(c.w.Coins, b.Pos)
		if at < 0 {
			continue
		}
		c.w.Coins = slices.Delete(c.w.Coins, at, at+1)
		b.AddBonus(c.rules.BonusMax)
		b.HP = min(b.HP+c.rules.CoinHeal, c.rules.HP)
		c.emit(event.CoinTaken{Bot: b.ID, X: b.Pos.X, Y: b.Pos.Y})
	}
}

// spawnCoin drops one coin on a random free cell every CoinEvery beats while
// fewer than CoinMax coins lie on the map.
func (c *beatCtx) spawnCoin() {
	if !c.coinDue() {
		return
	}
	if p, ok := c.pickSpot(c.freeCells()); ok {
		c.w.Coins = append(c.w.Coins, p)
		c.emit(event.CoinSpawned{X: p.X, Y: p.Y})
	}
}

func (c *beatCtx) coinDue() bool {
	r := c.rules
	return r.CoinEvery > 0 && c.w.Beat%r.CoinEvery == 0 && len(c.w.Coins) < r.CoinMax
}
