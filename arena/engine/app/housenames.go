package app

import "strconv"

// houseNames are the display names of house bots, handed out in order by the
// house counter. Ids stay house-N; only the name is funny.
var houseNames = [...]string{
	"WobbleBot", "SirBeepsBot", "NoodleBot", "BiscuitBot", "PicklesBot", "GizmoBot", "WaffleBot", "ZigZagBot",
	"MuffinBot", "BoopBot", "TurnipBot", "SprocketBot", "PancakeBot", "DoodleBot", "BananaBot", "ClunkBot",
	"FizzBot", "NachoBot", "PotatoBot", "SnazzyBot", "BumbleBot", "CrumpetBot", "WidgetBot", "KazooBot",
	"MangoBot", "GrumbleBot", "TofuBot", "PogoBot", "SquishBot", "DumplingBot", "BlinkyBot", "ToastBot",
	"OtterBot", "PixelBot", "JellyBot", "TwirlBot", "GadgetBot", "PretzelBot", "CosmoBot", "ZippyBot",
}

// houseName is the k-th candidate name (k from 0): the list in order, then
// again with a round number suffix (WobbleBot2, WobbleBot3, ...).
func houseName(k int) string {
	name := houseNames[k%len(houseNames)]
	if round := k / len(houseNames); round > 0 {
		name += strconv.Itoa(round + 1)
	}
	return name
}

// nextHouseName picks the name for house bot number n (1-based): the n-th
// candidate, or the next one after it if a bot in the arena already wears it.
// Pure in (n, players), so replays name bots the same way.
func (a *Arena) nextHouseName(n int) string {
	taken := make(map[string]bool, len(a.players))
	for _, p := range a.players {
		taken[p.name] = true
	}
	for k := n - 1; ; k++ {
		if name := houseName(k); !taken[name] {
			return name
		}
	}
}
