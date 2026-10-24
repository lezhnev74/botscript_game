# PAINTLINE - a territory-control bot arena

## 1. Pitch

Bots fight to **paint the arena floor in their team's color**; the map itself is the scoreboard, so at any instant - even mid-firefight - the whole room can see who is winning by how much floor each color owns.

Why it fits the constraints: humans read colored area preattentively (no numbers needed, perfect for one shared TV); wins come from **positioning** (lanes, zones, frontier pressure), not twitch aiming, so 250 ms ticks feel deliberate; combat exists only to take ground, which gives every fight a visible "why"; and team coordination is literal - your paint is your spawn territory, so covering a teammate's lane is a real, readable commitment.

## 2. World model

- Grid arena **32 x 18 = 576 tiles** (16:9, fills the TV). 2-4 teams, 4-8 bots total.
- One tick = **250 ms**; each bot performs exactly one card action per tick.
- Movement: 1 tile/tick (4 tiles/s; crossing the map takes ~8 s). Bullets: 4 tiles/tick (16 tiles/s).
- Tile states: neutral (gray) or a team color. Repainting converts instantly; last painter owns the tile.
- **Zones A, B, C**: three fixed 5 x 5 areas (25 tiles each) marked with big letters. Zone tiles count **double** in the territory bar, so the map has magnetic spots worth fighting over.
- **Paint dries**: a tile fades back to neutral after **60 s** unless refreshed (any friendly repaint, SPRAY, HOLD, or a teammate standing on it). No one-time land-grab decides the round; the frontier stays alive.
- Respawn: on death, reappear after **4 s** on a random friendly-painted tile within 6 tiles of your team's home edge (fallback: the home edge itself). Deep paint = safe spawns = map control matters beyond the bar.

## 3. Program model

Same DNA as the baseline: an ordered list of **IF condition -> THEN action** cards, first match wins, evaluated top to bottom every tick. **Order is the strategy.**

- Budget: **6 rule slots** at start, up to **8** earned across a season. No variables or loops; built-in memory only (last scan per direction, last hit direction, HP).
- Non-programmers build programs by tapping/reordering cards on a phone. Programmers (or their AI) may write a small text DSL that **compiles to the exact same card list** - nothing outside the card space is expressible.

**Sensors (conditions)**
- `enemy signal [dir/last] > X%`, `bullet signal [any/dir] > X%` (wedge scan, as baseline)
- `I was hit` (remembers direction)
- `tile [dir] is mine / enemy / neutral / wall`
- `I stand on my / enemy / neutral paint`
- `frontier distance > X tiles` (nearest color boundary)
- `zone [A/B/C] is ours / theirs / contested`
- `our map % < X` / `> X`
- `HP <= X`, `allies within 3 tiles >= X`, `time left < X s`
- `OTHERWISE`

**Actions**
- `MOVE [N/E/S/W / toward frontier / toward zone X / toward strongest signal / toward friendly paint / sidestep from threat]` - the tile you enter becomes yours.
- `SPRAY [dir / toward most enemy paint nearby]` - paints the **2 x 3 block** ahead; never harms teammates.
- `FIRE [dir / last strong signal / where hit from]` - bullet, 1 damage, first bot hit (friendly fire ON).
- `SCAN [dir / next clockwise / toward where hit]` - flashlight wedge sweep.
- `HOLD` - repaints your tile + 8 neighbors (refreshes decay; the "zone anchor" verb).

Dynamic directions (`toward frontier`, `toward zone A`) are what make 3-rule beginner bots look smart - they wander to where the game lives.

**Example 1 - Frontier Painter (beginner, 4 rules)**
```
1. IF bullet signal any > 50%       -> MOVE sideways from it
2. IF enemy signal last scan > 70%  -> FIRE that direction
3. IF standing on enemy paint       -> SPRAY toward most enemy paint
4. OTHERWISE                        -> MOVE toward frontier
```

**Example 2 - Zone Anchor (beginner, 5 rules)**
```
1. IF HP <= 1                           -> MOVE toward friendly paint
2. IF enemy signal toward zone A > 60%  -> FIRE that direction
3. IF zone A not ours                   -> MOVE toward zone A
4. IF standing in zone A                -> HOLD
5. OTHERWISE                            -> SCAN toward zone A
```

**Example 3 - Lane Controller (AI-written, 8 rules)**
```
1. IF bullet signal any > 40%             -> MOVE sideways from it
2. IF our map % < 40 AND time left < 60 s -> MOVE toward biggest enemy zone
3. IF HP <= 1 AND allies within 3 = 0     -> MOVE toward friendly paint
4. IF enemy signal > 50%                  -> FIRE that direction
5. IF zone A ours AND zone A contested    -> MOVE toward zone A
6. IF frontier distance > 4               -> MOVE toward frontier
7. IF tile ahead is enemy or neutral      -> SPRAY ahead
8. OTHERWISE                              -> SCAN next direction clockwise
```

## 4. Combat & scoring rules

- Bots have **3 HP**; bullets deal 1; sidestep clears a line, as in the baseline.
- **Kill = splatter**: the victim explodes into a radius-3 diamond (25 tiles) of the *killer's* color. Combat literally converts ground - the territorial payoff of a duel is visible one second later.
- No per-kill points. **Round winner = most tiles owned at 0:00** (zone tiles double). Tie-breakers: zones held, then kills.
- **Underdog boost**: a team more than 20% behind on the bar paints 50% larger sprays (3 x 3). Comebacks are visible and self-correcting.
- **Sudden death**: in the last 30 s, decay runs at 4x - abandoned ground cracks away and the frontier is forced to move.

## 5. Spectator layer

- The **map is the scoreboard**: full-screen color field, thin white-hot glow along the frontier, top bar with each team's % and a match clock. Readable from 10 meters in one glance.
- Zone rings pulse when contested; a zone flip triggers a full-screen letter banner + sound sting.
- Kills get a 1 s slow-mo splatter cam, then snap back - the paint blast shows why the kill mattered.
- Drying paint visibly cracks, so the room feels time pressure in the last 30 s.
- Between rounds: per-team **heatmap** (where bots actually spent time) and a ticker line of facts ("Blue hasn't touched zone C for 40 s", "Red respawns are 11 tiles from the frontier").

## 6. Session format

- 2-4 teams of 1-3 players; everyone owns one bot. Match = **best of 3 rounds, 3 min each**, with **90 s edit windows** between rounds (territory programs benefit from a calm look at the heatmap). A match fits ~15 min - one coffee break.
- Weekly ritual: season ladder, **rotating zone layouts** published each Monday, plus 3 built-in house bots (Painter / Anchor / Hunter) to challenge.
- Editing between rounds only; a mid-round save costs a 3 s "reboot" freeze, as in the baseline.

## 7. AI-written programs

- Explicitly welcome. AI users write the text DSL; the server compiles it to the card IR and **rejects anything that doesn't fit** (max 8 rules, listed sensors/actions only). This caps complexity by construction, not by trust.
- Programs are private during a match and **published after it**, so opponents can counter-program next week - the AI meta becomes a shared office conversation.
- The sim is deterministic (seeded RNG, fixed rule-evaluation order), so there is no timing exploit or simulation advantage to grind; strategy depth comes from the game, not the harness.
- Weekly layout rotation plus decay tuning means a pasted "solved" program from last week rots naturally.

## 8. Risks / degenerate strategies

| Strategy | Counter |
|---|---|
| Turtle in a corner, SPRAY in circles | Decay erases unused paint; zones (double value) sit elsewhere; underdog boost punishes letting the enemy farm |
| Kill-only duelist who never paints | Kills convert only 25 tiles each; the bar and the round win are territorial - a duelist loses to a mediocre painter |
| 8 bots stacked on one zone | Three zones force splits; contested zone resolution means a 2v2 elsewhere flips ground faster |
| Runaway leader snowballs | Underdog spray boost + respawns anchored to (shrinking) friendly paint keep the losing team in the game |
| Everyone pastes the same AI program | Post-match publishing + weekly layout rotation shift the meta; identical mirror programs tie by construction (determinism) |
| Two turtling teams stall to a tie | 4x decay in the last 30 s guarantees a moving frontier and a decision |
| Friendly fire griefing by ally | Bullets hurt allies (positioning talk!), but SPRAY is ally-safe, so painters never accidentally erase friends |

## 9. Implementation sketch

- TypeScript monorepo: **Node deterministic sim** (integer ticks, seeded PRNG, fixed card-evaluation order), **Canvas/PixiJS renderer** with 60 fps interpolation between ticks, phone editor as a web app joined by QR code over WebSocket.
- Programs are JSON card lists validated server-side - **no arbitrary code execution**, so sandboxing is trivial by design; the text DSL is parsed, never eval'd.
- Replays = seed + program lists; instant playback for post-round analysis.
- Prototype scope: one map, two teams, three house bots, the 90 s editor - roughly 1-2 weeks of part-time work before the first office session.
