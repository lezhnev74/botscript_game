# Bot Arena: Refined Baseline ("Gambit Grid")

## 1. Pitch
Grid bots run ordered IF->THEN cards; you write the cards, the TV shows which card each bot is running. Same core as the baseline, with four fixes: a slower beat the room can follow, sensing that creates mind games instead of spin-scanning, cards that render 1:1 from text an AI can write, and an opt-in **Tier 2** (states, memory, timers) so programmers and AI users get depth without changing how the game looks or reads.

**Baseline problems this fixes**
- 250 ms ticks x 8+ bots = noise on a TV. Nobody can see why anything happened.
- Optimal play collapses to "scan clockwise, fire at strong signal". Scanning has no risk, and percentages are unreadable.
- Dodging needs a scan, so it competes with everything else, and bots are either deaf or paralyzed.
- No state means AI-written programs hit the ceiling at once, and coin-unlocked slots snowball.

## 2. World model
- **Arena**: 21x13 grid (fits 16:9), 4-8 cover blocks (mirrored, seeded), cells ~70 px on a 1080p TV.
- **Beat**: 1 tick = **400 ms** (2.5 actions/s), tweened animation. Round = 3 min = 450 ticks.
- **Bots**: up to 12, in 2-4 team colors. 3 HP. Fire reload 3 ticks.
- **Bullets**: 2 cells/tick, 1 dmg, pass through coins, stopped by cover and by the first bot (friendly fire on).
- **Tick order (published)**: 1) all bots evaluate cards on the same snapshot, 2) moves resolve (contested cell: both bounce), 3) scans resolve, 4) fires spawn, 5) bullets advance and hit, 6) coins and score. Point-blank (<=2 cells) always lands. 3-4 cells can be dodged only if dodge is your top card. Long shots are for herding.
- **TV shows**: the arena, a bot token with name and a **card badge** ("#2 FIRE N") above every bot each beat, scan wedges as flashlight sweeps, tracers, and a sidebar scoreboard.

## 3. Program model
Players build cards on their phone. Every card list has an exact text form (BotCard), so pasted AI text renders as cards and cards export as text.

### Tier 1 (default, everyone)
- Up to **6 cards**, 1 condition each, evaluated top-down. The first match acts. `OTHERWISE` is the last card.
- Signals are **3 levels**: `faint / near / locked`, not percentages. They read as 1-3 bars on the TV.

**Sensors.** Passive sensors are free every tick. Scan sensors come from your last scan and go stale after 4 ticks.
| Sensor | Kind | Meaning |
|---|---|---|
| `BULLET [dir/any]` | passive | bullet approaching on my row/col within 4 cells |
| `HIT [dir/any]` | passive | took damage in last 5 ticks, from dir |
| `HEARD [dir/any]` | passive | a shot was fired within 6 cells (direction only) |
| `SCANNED [dir/any]` | passive | an enemy scanned me this tick, from dir |
| `BLOCKED [dir]` | passive | wall/edge/cover adjacent |
| `HP <= n`, `RELOADED`, `ALLY NEAR` | passive | self/team state |
| `ENEMY [dir/any] >= level` | scan | from last SWEEP |
| `COIN [dir/any] >= level` | scan | from last SWEEP |
| `LOCKED` | scan | last PING hit an enemy (distance known) |

**Actions** (each costs the whole tick):
- `SWEEP dir` - quarter wedge, fuzzy levels for enemy and coin. Quiet.
- `PING dir` - a 1-cell-wide line to the first object, exact type and distance. **Loud**: the target gets `SCANNED`.
- `FIRE dir`
- `MOVE dir`
- `SIDESTEP` - perpendicular to the latest threat, away from walls.
- `WAIT` - hold still.

**Direction words**: `N E S W`, `CW` (clockwise from last scan), `TARGET` (strongest enemy), `COIN`, `HIT` (where I was hit from), `HEARD`, `AWAY` (opposite of the threat).

### Tier 2 (opt-in, same cards plus 4 card types)
- **States**: up to 4 named card lists plus one `ALWAYS` list (max 3 cards), checked first in every state. `GOTO state` is a card action.
- **Memory**: registers `A B C` (int 0-99) and `D` (direction). `SET`, `ADD`, `SUB`, `SET D = TARGET`.
- **Timers**: `T1 T2`. `START T1 6` counts down 1 per tick. Test with `T1 DONE` / `T1 RUNNING`.
- **Logic**: up to 2 conditions per card with `AND`, and `NOT`. Signal and distance compare as numbers (`ENEMY N >= 60`, `PING_DIST <= 3`).
- **Team**: `SAY dir` posts a direction to teammates. `ALLY SAID [dir]` reads it.
- **Budget**: per tick, at most **6 instant ops** (`SET/ADD/START/GOTO/SAY`, which fall through to the next card) plus exactly 1 action. Over budget = **overheat**: the bot WAITs and the TV shows a steam puff. The whole program is capped at **24 cards**.
- **Handicap**: Tier 1 bots spawn with a 1-hit shield. This keeps rookies alive in open rounds.

### Examples
**Tier 1 "Lighthouse" (sniper)**
```
1. BULLET any      -> SIDESTEP
2. LOCKED          -> FIRE TARGET
3. ENEMY N >= near -> PING N
4. OTHERWISE       -> SWEEP N
```
**Tier 1 "Possum" (ambusher)**: plays dead until someone gives themselves away.
```
1. BULLET any -> SIDESTEP
2. SCANNED any -> FIRE SCANNED
3. HEARD any   -> FIRE HEARD
4. OTHERWISE   -> WAIT
```
**Tier 2 "Wolf" (typical AI output): bait, flank, strike.**
```
bot "Wolf" tier 2
always:
  BULLET any                  -> SIDESTEP
  HP <= 1 AND NOT ALLY NEAR   -> GOTO retreat
state search:
  ENEMY any >= near           -> SET D = TARGET ; START T1 4 ; GOTO bait
  OTHERWISE                   -> SWEEP CW
state bait:
  T1 DONE                     -> GOTO flank
  RELOADED                    -> FIRE D          # wide miss on purpose, forces a sidestep
  OTHERWISE                   -> WAIT
state flank:
  LOCKED AND RELOADED         -> FIRE TARGET ; GOTO search
  A >= 3                      -> SET A = 0 ; GOTO search
  OTHERWISE                   -> PING D ; ADD A 1
state retreat:
  COIN any >= faint           -> MOVE COIN
  HP >= 2                     -> GOTO search
  OTHERWISE                   -> MOVE AWAY
```
(`;` chains instant ops before or after the action. It counts against the 6-op budget.)

## 4. Combat and scoring
| Event | Points |
|---|---|
| Coin | +1 (golden coin +5, one per round) |
| Kill | +3, plus a **bounty** of +2 on any bot with a 3+ kill streak (crown on the TV) |
| Assist (hit in last 10 ticks before an ally kills) | +1 |
| Death | -1, respawn in 8 ticks at the edge cell farthest from enemies, 6 ticks spawn shield (can't fire) |
| Team kill | -3 |
- Coins: 6 on the field, respawning seeded-random, never within 3 cells of a bot.
- Last 30 s: **storm ring** shrinks 1 cell every 15 ticks and deals 1 dmg per 5 ticks outside.
- Team score = sum of its bots. Tiebreak: fewer deaths.

## 5. Spectator layer
- **Card badges**: every bot shows its firing card number and verb. The room learns programs by watching. "Bo is stuck on #4 WAIT!"
- **Lookahead director**: the sim is deterministic, so the TV renders 1.5 s behind the sim. It knows hits and near-misses before they happen, so it zooms 1.3x and slows to 0.5x for kills, last-moment sidesteps and bounty fights. Max one slow-mo per 10 s.
- **Loud scans**: PING draws a hard red laser and the target flashes "SPOTTED". SWEEP is a faint cone. You can see who is hiding and who is hunting.
- **Ticker commentary** (template-based): "Possum (Ana) ambushed Wolf (Bo) - card #2", "Lighthouse overheated".
- **Kill-cam**: the last 3 s before each kill replays in a corner PiP with both bots' card lists and the firing card highlighted.
- **Between rounds**: a 10 s highlight reel (top 3 moments by director score), then the round MVP's program shown full-screen as cards.

## 6. Session format
- **Join**: QR on the TV, then a phone PWA. Pick a team, then a house template or a blank deck.
- **Break session (~15 min)**: 3 rounds x 3 min, 90 s edit windows. Programs of the top 3 bots are **revealed** during each edit window, which drives counter-programming.
- **Weekly ritual (~40 min)**: Rookie Heat (Tier 1 only, 2 rounds), Open Heat (mixed tiers), Team Final (best 2 teams). Season ladder by Elo per player, plus a "Card of the Week" award voted by the room.
- **Live edit** is allowed mid-round: the bot freezes for 8 ticks ("REBOOTING", visible) and loses its stale scan memory.
- **Offline practice**: the phone and web runner simulate your bot vs house bots or last week's revealed bots, at 8x speed.

## 7. AI-written programs
- **Encouraged**: a public "Prompt Pack" (1-page BotCard grammar, tick order, numbers, 6 example bots, last week's winners) is designed to be pasted into any chatbot.
- **Paste then cards**: the validator parses the text and renders cards. Errors show as red cards with a plain-language fix. Nothing is hidden: an AI program reads like anyone else's on the TV.
- **Fairness through limits, not bans**: the same 24-card / 6-op / 4-state caps for everyone. Depth comes from the design (baiting, timing, team signals), not from code size. Tier 1 shield and Rookie Heat protect non-coders.
- **Iteration parity**: the offline runner is available to everyone, so the advantage is ideas, not compute. Submissions lock at the end of each edit window.

## 8. Risks and counters
| Degenerate strategy | Counter |
|---|---|
| Spin-scan-fire loop | SWEEP only gives levels. A kill-confirm needs a loud PING, which warns the target. Reload 3 ticks. |
| Camping (WAIT + fire HEARD) | Possum is a valid archetype. It is countered by long-range PING + fire and by the storm. A bot idle for 20+ ticks becomes "restless" and visible to all SWEEPs. |
| Perma-dodge (top card SIDESTEP) | Dodging costs the tick. Herding shots push dodgers into a second shooter's line. Point-blank can't be dodged. |
| Bullet wall down a column | Reload 3, cover blocks, team-kill penalty. |
| Spawn camping | Farthest-from-enemy spawn plus shield. |
| Tier 2 overwhelms rookies | Rookie Heat, Tier 1 shield, and revealed programs let rookies copy winners. |
| Snowball | No coin-unlocked slots (baseline idea dropped). Bounty rewards killing the leader. |
| Stalemate mirrors | Seeded coin spawns break symmetry, and the storm forces contact. |
| Infinite GOTO loops | 6 instant ops per tick, then overheat. |

## 9. Implementation sketch
- **TypeScript monorepo**. `sim` is a pure, integer-only `step(state, programs, rng) -> state` with no floats, so replays are bit-exact.
- **Server**: Node with WebSockets, authoritative sim, room codes. The **TV client** uses PixiJS, interpolates between ticks and runs the lookahead director over the sim's future frames. The **phone** client is a PWA card editor with the offline sim (same package) for practice.
- **Programs are data, not code**: the BotCard text parses to a JSON AST, and a bounded interpreter runs it (max 24 cards, 6 ops). No sandbox needed, and no eval.
- **Persistence**: SQLite holds programs, replays (seed + programs + edit log) and Elo.
- **Prototype order**: sim + Tier 1 + 3 house bots on the TV, then the phone editor, then the director, then Tier 2.
