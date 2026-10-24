# BOT ARENA - Game Vision

Synthesis of 8 competing designs (`designs/01-08`, written by Opus, Sonnet, Grok and GLM) and 2 independent judges (`judging/judge-opus.md`, `judging/judge-grok.md`).

**Owner decisions (locked):**
1. No energy economy. Firing uses a **reload cooldown**, and **coins give a bonus shot**.
2. **Kills are the main mode.** Territory (PAINTLINE) is a later mode.
3. **Scans are drawn as an x-ray** of exactly the area each bot scanned, so the room sees what every bot is looking at and what it learned.
4. **Programming = taps.** **Editing is allowed any time** in v1 (hot-swap at the next beat, rate-limited, pencil badge on TV).
5. **Scan-n-act: the language is SCANLOOP** (`LANGUAGE.md`, supersedes sections 3 action table, 4 and the sensor/scan
   parts of 2, 6.1, 9). An imperative loop: `scan dir` (1 beat) returns `enemy/coin/bullet` levels 0-3 (= how lined up,
   no side, no distance), free `if`/`chance` logic, then `move/fire dir`. Only interrupts: `on hit`, `on bump` (the bumped
   bot notices nothing). No facing. Walls: moving into one is a no-op, `rand` skips walls. Scans cover the cone to the
   walls, see through everything, are silent. Dice (`rand(...)`, `chance:` switch) are first-class. No teams (FFA only).
   Where sections below mention SWEEP/PING, HEARD, SCANNED, cover shadows, teams or cards, SCANLOOP wins.

## Verdict at a glance

| # | Design | Opus judge | Grok judge | Role in the final game |
|---|---|---|---|---|
| 01 | Gambit Grid (refined baseline) | 48 | 46 | **Backbone**: cards, badges, quiet/loud scan, tiers |
| 03 | Same Beat (simultaneous + energy) | 41 | 49 | **Core timing**: blind simultaneous lock, STEP_IN/LEAD. The energy part was dropped (decision 1) |
| 08 | SIGNAL (layered language, noise) | 47 | 41 | **Depth layer**: states/timers, decoy, noise, brain strip |
| 04 | PAINTLINE (territory) | 44 | 44 | **Later mode** (decision 2) |
| 02 | CODEBOTS (JS) | 42 | 39 | Steal `say`/mood bubbles, victim-POV replay. Drop JS |
| 07 | Bot Hour (spectator meta) | 37 | 36 | Steal the ritual for season 2 |
| 06 | BEARING (continuous) | 39 | 29 | Reject: unreadable mush, aim math excludes non-coders |
| 05 | Squad Radio (roles) | 30 | 30 | Reject: a balancing and readability trap |

Both judges agreed on: grid with 4 directions, a slow beat, the firing card painted on every bot, programs as data (cards with a text form, no user code in v1), equal caps for everyone, no coin-bought slots, no percentages on screen, programs revealed between rounds, and deterministic replays.

---

## 1. Pitch

Up to 10 bots on one TV, all locking an action at the same moment, every 600 ms. You program a bot with 6-8 IF->THEN cards. The TV shows which card each bot is running and x-rays every area it scans, so the room reads the programs by watching them play. Between rounds, the winners' programs are revealed and everyone gets 90 s to counter-program, alone or with an AI.

## 2. World

- **Grid** 19x11 (16:9, cells ~90 px at 1080p), 4 directions, 4-6 mirrored cover blocks, seeded per round.
- **Beat**: 600 ms by default (tunable 400-800 on a real TV), tweened animation. A round is 3 min, 300 beats.
- **Blind simultaneous lock** (03): all bots decide on the same snapshot, then the beat resolves in a published order: moves -> scans -> new shots -> bullets travel -> hits -> coins. A collision on the same cell bounces both bots. Hits use the post-move cell, so **a sidestep always beats a shot you predicted**. That is the mind game.
- **Bots**: 3 HP. Bullets travel 3 cells per beat, are stopped by cover, and hit the first bot, ally or not.
- **Cover blocks scans too.** Cells behind cover sit in a scan shadow, which gives bots somewhere to hide.
- **God view on TV**: the room sees everything, while bots see only what their sensors report. That gap is the comedy ("everyone saw the ambush except Bo's bot").

## 3. Weapons: reload + coin bonus shots

- **Reload**: after FIRE, the gun needs **3 beats** to reload. On the TV, a ring around the bot fills up, and the bot shows a bright **"loaded" muzzle** when it is ready. The tell survives: the room sees who is armed.
- **Bonus shots from coins**: each coin gives +1 score and **+1 bonus shot** (max 2 stored, shown as gold pips). A bonus shot fires **even while reloading**, so a coin-holder can double-tap. That gives coins a combat purpose, so collectors become dangerous and are worth hunting.
- **Point-blank** (<= 2 cells) always lands. Longer shots can be sidestepped if the target predicts them.
- One action per beat, nothing else to manage. The reload is the only pacing rule, which keeps the help screen to a single line.

Action table below is historical; the v1 actions are `scan / move / fire / wait` (see `LANGUAGE.md`).

| Action | Notes |
|---|---|
| MOVE dir | One cell |
| SIDESTEP | Perpendicular off the nearest threat line, toward open cells |
| SWEEP dir | Quiet, fuzzy wedge: enemy/coin as `faint / near / locked` |
| PING dir | **Loud**: an exact line to the first object, with distance. The target gets `SCANNED` |
| FIRE dir | Uses the loaded gun first, then a bonus shot |
| LEAD | FIRE at the cell a STEP_IN enemy is about to enter. If none exists, it becomes a free WAIT |
| WAIT | Still for 3+ beats = **dark** (sweeps read it one level lower) |
| DECOY dir | Footsteps beacon 4 cells away, 2 per life (v1.1) |

## 4. Program model (historical - superseded by SCANLOOP in `LANGUAGE.md`)

Cards, text and (later) script all compile to **one deterministic interpreter**. Nobody gets powers others lack.

**Tier 1 (everyone, default):** up to 8 cards, first match wins, `OTHERWISE` last. 1 condition per card, or 2 joined by AND/NOT.

**Sensors.** Passive ones are free every beat:
- `BULLET dir`, `HIT dir` (where damage came from), `HEARD dir` (moves within 4 cells, shots heard arena-wide), `SCANNED dir` (someone pinged me)
- `ON_LINE dir` (clear line to an enemy), `STEP_IN dir` (their last move would put them on my line), `ARMED dir` (that enemy shows a loaded muzzle)
- `LOADED`, `BONUS >= n`, `HP <= n`, `BLOCKED dir`, `ALLY NEAR`, `FRIEND dir` (an ally is on my fire line), `RAND < n`
- From the last SWEEP/PING: `ENEMY dir >= faint|near|locked`, `COIN dir`, `LOCKED` (with distance)

**Direction words:** `N E S W`, `CW`, `TARGET`, `TOWARD`, `AWAY`, `HIT`, `HEARD`, `COIN`.

**Tier 2 (opt-in, same cards plus 4 card types, from 01/08):**
- **States**: up to 4 named card tabs plus an `ALWAYS` list (Hunt / Bait / Flee).
- **Timers** `T1 T2` ("for N beats" chip).
- **Registers** `A B C` and a direction register `D`.
- **Team**: `SAY dir` / `ALLY SAID dir`.
- **Caps**: 24 cards, 6 instant ops per beat. Over the cap = **overheat** (the bot waits, steam puff on TV).

**Text form.** It is exact and 1:1 with the cards, so pasted AI text renders as cards and cards export as text. Errors come back as red cards with a plain-language fix and a line number.

```
bot "Lamp"                       # default template, the sniper
1. BULLET any                -> SIDESTEP
2. ON_LINE any AND LOADED    -> FIRE TOWARD
3. ENEMY any >= near         -> PING TARGET
4. OTHERWISE                 -> SWEEP CW

bot "Magpie"                     # collector that turns coins into a double-tap
1. BULLET any                -> SIDESTEP
2. ON_LINE any AND BONUS >= 1 -> FIRE TOWARD
3. COIN any                  -> MOVE COIN
4. OTHERWISE                 -> SWEEP CW

bot "Possum"                     # ambusher: stays dark until someone gives themselves away
1. BULLET any                -> SIDESTEP
2. SCANNED any AND LOADED    -> FIRE SCANNED
3. HEARD any AND LOADED      -> FIRE HEARD
4. OTHERWISE                 -> WAIT

bot "Wolf" tier 2                # typical AI output: bait, flank, strike
always:
  BULLET any                 -> SIDESTEP
  HP <= 1                    -> GOTO flee
state hunt:
  ON_LINE TOWARD AND NOT ARMED TOWARD AND LOADED -> FIRE TOWARD   # shoot the one who just fired
  STEP_IN any AND LOADED     -> LEAD
  ENEMY any >= near          -> SET D = TARGET ; START T1 3 ; GOTO bait
  OTHERWISE                  -> SWEEP CW
state bait:
  T1 DONE                    -> GOTO hunt
  OTHERWISE                  -> DECOY D               # footsteps one way...
state flee:
  HP >= 2                    -> GOTO hunt
  OTHERWISE                  -> MOVE AWAY             # ...body the other
```

**Built-in rock-paper-scissors:**
- Lamp (sniper) beats Magpie (collector).
- Possum (ambusher) beats Lamp.
- Hunter (pinger) beats Possum.
- Magpie with its bonus double-tap beats Hunter.

## 5. Scoring (kills mode)

| Event | Points |
|---|---|
| Kill | +3, but repeat kills on the same victim pay +1 (anti-farming) |
| Assist (hit within 8 beats before an ally's kill) | +1 |
| Coin (4 on field) | +1 score, +1 bonus shot (max 2) |
| Death | -1 (floor 0), and stored bonus shots are lost. Respawn after 5 beats at the edge farthest from enemies, 2 beats shielded and unable to fire |
| Team kill | -3 |

There is one closer: the outer ring burns for the last 30 s. Don't stack storm + golden coin + bounty.

## 6. The TV (spectator layer)

### 6.1 Scan x-ray (the signature visual)

> SCANLOOP update: one silent `scan` verb. The x-ray tints the whole cone to the walls (no scan shadows), silhouettes
> each reported enemy/coin/bullet with its 0-3 level bars, and the badge shows the line running (`scan E`, `30% -> move S`).
> PING laser, HEARD ripples and SPOTTED below are dropped for v1.

Every scan is drawn as exactly what the bot looked at and exactly what it learned, in that bot's color, for the beat it happens.

- **SWEEP = x-ray wedge.**
  - **What it shows:** the precise cells the wedge covered light up in a translucent tint of the scanner's color with a scanline texture. Cells in scan shadow behind cover stay dark, so the room sees the blind spot.
  - **What the bot learned:** every enemy or coin inside the wedge gets an x-ray silhouette (a glowing outline in the scanner's color). A tag next to it shows exactly what was reported: 1-3 bars for `faint / near / locked`. A dark (still) bot shows as a dimmer, broken outline, one level lower. Objects outside the wedge stay plain, so you see what the bot **missed**.
  - **Timing:** it fades over ~1 beat, leaving a faint 2-beat "memory" ghost, the same window the bot's scan data stays fresh.
- **PING = laser.** A solid beam runs cell by cell from the scanner and stops at the first object. The distance number is printed at the hit point. The target flashes **SPOTTED** with an eye icon, matching its `SCANNED` sensor.
- **HEARD = ripples** from moves and shots, so the room sees what passive hearing picked up.
- **Badge link.** The bot's badge (`#4 SWEEP N`) shares the tint of its wedge, so overlapping scans stay attributable.
- **Clutter control.** It never draws more than one scan per bot per beat. Overlapping tints blend up to an alpha cap. The host can toggle the x-ray mode:
  - **all**: every scan drawn as described above (default up to 6 bots).
  - **outline-only for unfocused bots**: wedge edges only, plus full x-ray for the bots the director is watching (above 6 bots).
  - **focused only**.
- **Replays.** The kill-cam replays both bots' last scans: "Wolf swept east and saw nothing. Possum was in the scan shadow behind the pillar."

The x-ray also teaches the game. Within a round, people see that a fuzzy sweep does not reveal much, and that a ping is precise but warns the target. That pushes them to think about information on their own.

### 6.2 Other v1 visuals

- A **card badge** over every bot each beat (`#2 FIRE N`), plus a state badge in Tier 2.
- A **reload ring** and a "loaded" muzzle glow, plus gold bonus-shot pips.
- Bullets as tracers, with a miss drawn through the cell the target just left. Dark bots are dimmed.
- **Lookahead director**: the sim runs 1.5 s ahead of the render, so the TV knows a kill or near-miss before it happens. It slows to 0.5x for 2-4 beats at most, once every 10 s, and never pans or crops the arena.
- **Kill-cam inset** at half speed while live play continues. Two lines tell the story: killer `#2 ON_LINE E -> FIRE E`, victim `#4 SWEEP W - looked the wrong way`. Both scans are x-rayed.
- **Sidebar**: huge names, HP ticks, reload/bonus state, a sortable score, a template ticker ("Possum ambushed Wolf - card #2").

### 6.3 v1.1

- `say`/mood bubbles (free side-channel, may lie, 1 change per 2 s). Silent bots get an auto-label from their card.
- A **brain strip** for the 2 most interesting bots, firing card highlighted.
- A victim-POV replay ("what the bot knew"): only the scanned, x-rayed cells are visible, and the rest is fog.

### 6.4 Season 2 (from 07)

- Auto awards (Bot of the Week, Giant Killer, Bug of the Week, Hall of Shame for friendly fire).
- A hall of fame, so last week's champion becomes this week's boss bot.
- A "what if" replay (swap one program, same seed).
- A Slack clip export.
- Maybe an LLM caster ("roast the program, never the person").

## 7. Session format

- **Join**: QR code on TV, then a phone web app. Name the bot, pick a template (Lamp / Magpie / Possum / Hunter) or paste text.
- **Coffee-break match (~15 min)**: 3-4 rounds x 3 min, 90 s edit windows. During each edit window, the **top-3 programs are shown on TV** and every program is public on phones. This minute is the meta.
- **Weekly ritual (~40 min)**: Rookie Heat (Tier 1 only), then Open Heat (mixed), then a Final with the top teams. Elo ladder per player, and a physical trophy (rubber duck) passed on.
- **FFA only** (no teams for now). Empty seats are filled by house bots.
- Edits are allowed **any time** (hot-swap rules in `LANGUAGE.md` section 5). Edit windows between rounds remain the main counter-programming moment.

## 8. AI-written programs

- **Encouraged**: a one-page **Prompt Pack** (grammar, beat order, numbers, 6 example bots, last week's winners) behind a "Copy AI prompt" button. The AI returns text, and the phone renders it as cards.
- **Fair by structure, not bans**: the same caps for everyone (8 or 24 cards, 6 ops). Programs are data, with no code execution, no access to enemy source or identity, and no lookahead. An AI can make a bot **smarter**, never **better informed**.
- **Brute aggression** is capped by the reload plus the visible "loaded" tell. **Spin-scan-fire** is capped by quiet/loud scanning. The x-ray shows the room exactly how predictable an AI bot's scan pattern is, and people learn to bait it.
- **Practice parity**: a "vs house bots" button for everyone, at 8x speed. Hold back the batch-seed CLI until the meta feels solved.
- Rookies get a 1-hit spawn shield in Rookie Heat, and a Tier 1 bot can copy any revealed program.

## 9. Degenerate strategies -> counters

| Strategy | Counter |
|---|---|
| Fire every beat | 3-beat reload. Bonus shots are finite and only come from coins |
| Coin hoarding for burst | Max 2 stored, lost on death, so hoarders become priority targets |
| Spin-scan-fire | SWEEP only gives levels. An exact lock needs a loud PING, which warns the target, and the x-ray shows the room the pattern |
| Perma-dodge | Dodging spends the beat. Point-blank (<= 2 cells) can't be dodged, and crossfire herds |
| Camping in the dark / scan shadow | Idle 20+ beats = "restless" (visible to all sweeps), plus the outer ring |
| Spawn / name farming | Far shielded respawn, repeat kills pay +1 |
| Everyone pastes the same AI bot | Reveals, the RPS archetypes, seeded maps, RAND, and the hall-of-fame boss |
| Infinite GOTO loops | 6-op cap, then overheat (visible) |
| Snowball | Equal caps, no unlocks, assists, one closer |

## 10. Build plan

**Stack:** TypeScript monorepo.
- `sim`: a pure integer `step(state, locks, rng)`, deterministic. A replay is seed + programs.
- `lang`: a text parser and validator into a JSON AST, plus a bounded interpreter. No eval, no sandbox needed.
- `server`: Node + WebSocket, authoritative, with room codes.
- `tv`: PixiJS, tweening, the director.
- `phone`: a PWA editor.
- `SQLite` for programs, replays and Elo.

**One addition to the sim for the x-ray.** Each beat it emits a **scan record** per scanning bot: the covered cells, the shadowed cells, and the objects reported with their levels. The TV draws exactly that record and never recomputes it, so what you see is literally what the bot got. The kill-cam and the POV replay reuse the same records.

**MVP (3-4 weeks):**
1. sim + text language Tier 1 + 4 house bots, running 50+ headless rounds to tune reload, bullet speed and coin rate.
2. A TV renderer with badges, the **scan x-ray**, the PING laser, reload rings and tracers, tested **on the real TV at 4 m** with a beat-speed slider.
3. Phone: QR join, a text editor with live card preview, paste validation, templates, the reveal screen.
4. Lookahead director + kill-cam.
5. Tier 2 (states, timers, registers).

**Later:** a drag-and-drop card editor, decoy + noise ripples, bubbles, the brain strip, POV replay, the ladder and hall of fame, **PAINTLINE territory mode** (floor = scoreboard, kills splatter paint), and maybe a script layer if Tier 2 gets solved.

## 11. Open decisions

1. **Beat speed**: 400 vs 600 vs 800 ms. Settle it on the real TV with 8 bots and the x-ray on; keep the slowest one where people still lean in.
2. **Reload length**: 3 beats as a start. Tune it so a duel lasts about 3-5 beats.
3. **Fog**: blind locks already hide intent. The recommended v1 is god view on TV and sensors-only for bots, with fog only in the POV replay.
4. **Reveal policy**: top-3 every round (fast meta, copying allowed) vs everything after the match.
5. **Max bots**: the judges say 8-10, not 12, until badges and x-rays stay readable.
6. ~~Card UI in v1~~ - decided: the tap editor ships in v1, paste is for AI text.
7. **Fake-currency betting** (07): fun, or an HR conversation in your office?
