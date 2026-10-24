# CODEBOTS - code-first design

## 1. Pitch
Write a 30-line JavaScript function `think(w)` that returns one action per tick; 8-12 bots fight in a walled arena on the office TV. The language is real, the API is ~25 names, and every bot narrates itself ("say" bubbles, mood labels), so the room follows intent without reading code. An AI is the intended "pair programmer": the API is small and regular enough for any LLM to hit on the first try, and depth comes from information, timing and counter-programming, not syntax.

Fit: one screen (arena + labels), 300 ms ticks, 3 min rounds, non-programmers use templates + sliders + a prompt box, programmers get a full language with persistent memory.

## 2. World
- Grid 24x14, 4-direction movement, ~20 wall blocks (random per round, mirrored for fairness; seed shown on TV). 300 ms/tick, 600 ticks/round.
- **Fog:** each bot sees a radius-6 diamond. Allies pool vision. Everything on the TV is visible to humans, never to bots (this gap is the drama: the room sees the ambush the victim doesn't).
- TV: bots as big coloured shapes with owner name tag, HP pips, ammo pips, a mood icon and a speech bubble; bullets as bright tracers; faint vision outlines for the bot in focus; dotted "plan line" to the bot's declared target; scoreboard on the side.

## 3. Program model
Language: JS subset run in QuickJS-WASM. One file, <= 3000 chars (forces distilled logic, keeps AI output reviewable). No imports, no `Math.random`, no `Date`, no async.

```js
function think(w) { ... return <action>; }
```
Globals persist between ticks only through `w.mem` (plain JSON object, <= 1 KB serialised; overflow = wiped + flagged).

**Sensors (read-only, in `w`)**
| name | meaning |
|---|---|
| `w.me` | `{x,y,hp,ammo,face,guardCd,id,team}` |
| `w.t` | tick number (0-599) |
| `w.map(x,y)` | `'floor'` / `'wall'` / `'edge'` (always known, full map) |
| `w.see` | array of visible things: `{kind:'bot'/'coin'/'bullet'/'gold', x,y,dir,hp,team,id,say}` (`say` = what that bot's bubble currently shows) |
| `w.hit` | hits taken last tick: `[{from:'N'..., dmg}]` (direction only, not distance) |
| `w.zone` | current safe rectangle (shrinks in last 40 s) |
| `w.rand()` | seeded PRNG, deterministic per bot per round |
| `w.mem` | persistent memory |
| `w.team` | ally ids + their last `send()` message |

**Helpers (native, flat 10-op cost each, so naive code is never punished for using them)**
`w.dist(a,b)`, `w.nearest(kind)`, `w.aligned(a,b)` -> dir or null if same row/col with clear line, `w.step(target)` -> dir of first step on shortest path (walls aware), `w.threats()` -> `[{dir,eta}]` bullets that will hit my cell, `w.safe()` -> dirs not on a threatened line and not walls.

**Actions (return exactly one; invalid/none = `wait()`)**
- `move(dir)` - 1 cell.
- `fire(dir)` - bullet, speed 2 cells/tick, first thing hit takes 1 dmg (allies too). Costs 1 ammo.
- `guard()` - this tick, bullets from `face` direction are blocked and give the shooter nothing. Face = last move/fire direction. Can't be used 2 ticks in a row (cooldown 2).
- `ping()` - for 1 tick see radius 12, but every enemy gets a flashing ring at your cell for 3 ticks.
- `wait()` - recover (regain ammo twice as fast).

**Free side-channels (never consume the tick):** `say(str<=20)` (shows bubble, max 1 change per 2 s, can lie), `mood('hunt'|'loot'|'flee'|'ambush'|'bait'|'guard'|'scout')` icon, `aim(x,y)` draws the dotted plan line, `send(str<=8)` to allies (arrives next tick).

**Limits:** 3000 VM ops/tick (bytecode-counted, so deterministic, not wall-clock). Over budget = tick aborted, bot idles, "BRAIN FRY" smoke on TV. Memory 1 KB; no globals survive; hard caps on recursion/array size.

**Example 1 - Rookie Collector (template, 8 lines)**
```js
function think(w) {
  const th = w.threats();
  if (th.length) return move(w.safe()[0] ?? 'N');
  const c = w.nearest('coin');
  if (c) { mood('loot'); return move(w.step(c)); }
  return wait();
}
```
**Example 2 - what an AI typically writes (greedy + dodge + shoot-if-aligned)**
```js
function think(w) {
  const me = w.me;
  if (w.threats().length) { say('nope'); return move(w.safe()[0] ?? 'N'); }
  const foes = w.see.filter(e => e.kind==='bot' && e.team!==me.team);
  for (const f of foes) {
    const d = w.aligned(me, f);
    if (d && me.ammo > 0) { mood('hunt'); w.mem.last = [f.x,f.y]; return fire(d); }
  }
  const t = w.nearest('gold') || w.nearest('coin') || (w.mem.last && {x:w.mem.last[0],y:w.mem.last[1]});
  if (t) return move(w.step(t));
  return move(['N','E','S','W'][w.t % 4]);
}
```
This is a solid mid-pack bot; the meta then punishes its predictability (below).

**Example 3 - Baiter (human/AI-tuned, uses lying + guard)**
```js
function think(w) {
  const me = w.me, foe = w.nearest('bot');
  if (w.threats().length) return me.guardCd ? move(w.safe()[0]) : guard();
  if (foe && w.dist(me, foe) <= 5) {
    say('help!'); mood('bait');                 // looks hurt, lures hunters into ally's line
    const d = w.aligned(me, foe);
    return d ? (me.ammo ? fire(d) : guard()) : move(w.step(w.team.ally?.pos ?? foe));
  }
  send(me.x + ',' + me.y); mood('ambush');
  return wait();
}
```
Note: `ai` helpers (`threats`, `step`) make the 80% case trivial; the 20% edge (reading `say`, `ping` timing, allies' `send`, mem-based opponent modelling) is where depth lives.

## 4. Combat & scoring
- HP 3, ammo cap 4 (+1 per 4 ticks, +1 per 2 ticks while waiting), fire has no cooldown but ammo is the limit.
- Bullet speed 2, so at vision 6 a bullet is seen 3 ticks out: a side-step always works if you look, so **kills come from crossfire, corners, ammo exhaustion, guard-baiting and fog**, not raw aim.
- Respawn 5 ticks after death at the spawn-edge farthest from current action, 3 ticks of invulnerability.
- Points: coin +1, gold coin +5 (appears centre, announced on TV with a 5 s warning), kill +3 (last hitter), death -1, friendly kill -3 (to the shooter). Team score = sum.
- Last 40 s: safe zone shrinks; outside = 1 dmg/2 s. Hunger: no damage dealt/coin taken in 20 s = lose 1 HP per 10 s ("camper tax").

## 5. Spectator layer
- **Intent is visible:** mood icon + `say` bubble + dotted `aim()` line on every bot, always on. Auto-labels when code sets none (derived from action: moving toward coin = loot, firing = hunt), so even silent bots read.
- **Tension director:** auto-zoom when a bullet is <= 3 cells from a bot that hasn't reacted; drops to 0.35x slow-mo for the 2 ticks before impact, then a freeze-frame label ("Dodged by 1 cell!" / "Shot in the back").
- **Instant replay** of kills from the victim's POV: the fog circle is drawn, showing the room "what the bot knew" - the best comedy moment.
- **Commentary ticker**: template announcer from events ("Maria's bot is being hunted by 2", "Gold in 5..."). Optional LLM caster later; not required.
- **Post-round reveal:** top-3 snippets of code shown for 10 s with the bot's `say` highlights; fuels counter-programming talk.

## 6. Session format
- Setup: join via QR, name, pick a template or write code in the browser editor (Monaco + API panel + "Copy AI prompt" button that copies the API spec + rules + your current code).
- Free-for-all of 8-12 bots, or teams of 2 (one bot each, shared vision + `send`) - pair a programmer with a prompter.
- **Round = 3 min, edit window = 90 s, 5 rounds** (~25 min total, fits a break). Map seed changes each round.
- **Dry run:** 15 s headless sim (x20 speed) vs house bots, errors and timeouts shown with line number. Edit at any time; submission is frozen only at edit-window end.
- Weekly league: points per round (placement-based), top 3 of the week join the "Hall of Fame" pool: their code is saved as house opponents next week (old champion = boss fight).

## 7. AI-written programs
- Encouraged: copy-AI-prompt button, public API.md (< 1 page) and a deliberately tiny type definition so models get the shape right. Optional built-in "describe your bot" box that calls an LLM with the spec and shows the generated code + a plain-English summary; the player must press "deploy".
- Kept fair: 3000-char cap, op budget, 1 KB memory, no I/O, no network, no information the human TV viewer has but bots lack, identical API for all. Many AI bots converge on the same greedy bot (Example 2), so the meta rewards the *prompt* ("assume opponents shoot when aligned; bait them"), not typing speed. Players read `say` and see how enemies behave, then counter-program in the next 90 s: that is the loop.
- Non-programmers: Rookie mode = sliders (greed / caution / aggression / ammo discipline) that generate editable code. Cards from the baseline compile to the same `think` body, so the card editor is a view over code, not a separate game.

## 8. Risks / degenerate strategies
| Risk | Counter |
|---|---|
| Corner camping with guard | guard cooldown 2, ammo needs `wait`, hunger tax, shrinking zone |
| One AI bot everywhere (Example 2 clones) | randomised maps, gold events, bait/lie layer, hall-of-fame bosses reward counter-play |
| Hardcoded map coordinates | map seeded per round, only `w.map` reads it, mirrored spawns |
| Ally farming (suicide kills) | friendly kill -3, kill points to last hitter only |
| Heavy compute / infinite loops | deterministic op budget, tick abort, idle penalty |
| Opponent-ID hardcoding | ids shuffled each round, no owner names in `w.see` |
| Spam `ping` to see everything | reveals you to all, costs the tick, ammo not regenerated that tick |
| Stalemate between two sniper bots | bullets stop at walls, fog, hunger tax, shrink |
| Too hard for beginners | templates + dry run + live round-1 "chaos is fine" framing, round-1 scores halved |
| Unreadable TV with 12 bots | 3 colours of teams, max 12 bubbles, the director dims bots away from the action |

## 9. Implementation
- TypeScript, one **deterministic sim** (fixed seed, integer coordinates, no floats, tick-indexed PRNG); same module runs in browser (replay/dry-run) and server (authoritative match).
- Bots: QuickJS-WASM per bot, fresh context per round, `interrupt handler` counting ops for the budget, 16 MB heap cap, no host APIs except the frozen `w` snapshot (deep-cloned JSON) and action/side-channel functions.
- Server (Node/Bun): WebSocket; phones/laptops submit source; server compiles in the sandbox, runs a smoke test, then stores `(source hash, version)`. TV client is a pure renderer of an event stream (also saved, giving replays for free).
- Replay = seed + all sources (sim is deterministic), so highlights and "watch last week's boss fight" cost nothing.
- Build order: sim + 3 house bots (1 week) -> TV renderer with labels (1 week) -> editor + dry run -> director/slow-mo -> league.
