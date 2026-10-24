# BOT ARENA - Implementation Plan

Source of truth for rules: `../VISION.md`, `../LANGUAGE.md`. This plan covers how to build them.
Everything lives in `arena/`. Each step = TDD (red -> green -> refactor), and a step ends with `make check` passing.

---

## 0. Ground rules

- **TDD only.** No production code without a failing test first. A behaviour that has no test does not exist.
- **Scenarios are the spec.** Every rule in VISION/LANGUAGE maps to at least one scenario file (see 3). The table
  `engine/testdata/scenarios/INDEX.md` lists rule -> scenario(s); a rule with no scenario counts as not implemented.
- **CRAP < 8 per function**, enforced in `make check`. With 100% coverage this allows cyclomatic complexity up to 7;
  in practice aim for <= 4. Lint also runs `gocyclo -over 7` and `gocognit -over 10`.
- **Determinism is a hard constraint** in `domain/` and `lang/`: no `time`, no `math/rand` global, no map-order
  iteration in output paths (sorted IDs), no goroutine-order effects. Enforced by `forbidigo` + `depguard` rules.
- **Clean Architecture dependency rule:** `domain` <- `lang` <- `app` <- `adapters` <- `cmd`. Inner layers never
  import outer ones (`depguard` enforced).

## 1. Layout

```
arena/
  engine/                      Go module (go 1.26)
    domain/                    pure game rules, no IO
      geo/                     Dir, Pos, Grid, ASCII map parsing
      sim/                     World state + Resolve(world, actions) -> (world', events)
        rules.go               Rules config (bullet speed, reload, HP, ...) - tuning without code changes
        <phase>.go             one resolver per beat phase (move, scan, fire, bullets, hits, coins, respawn); in sim to avoid an import cycle
      event/                   typed events (Moved, Bumped, Scanned, Fired, Clicked, Hit, Killed, CoinTaken, ...)
      brain/                   port: Brain interface (what a bot "is" to the sim)
    lang/                      BOTSCRIPT: parse (goja/parser) -> validate -> interpret; implements brain.Brain
    app/                       use cases: Match loop, Players/sessions, HotSwap, Leaderboard, Replay
      ports.go                 Clock, Store, Broadcaster, IDGen, Rand
    adapters/
      httpapi/                 REST: join, get/put my bot, leaderboard
      ws/                      live stream of frames/events to UI
      store/                   JSON-file store (leaderboard + bots); swappable
      clock/                   real ticker; ManualClock lives in tests
    cmd/
      arena/                   web server
      sim/                     headless runner: run N matches, print stats (tuning beat/reload/cone)
      replay/                  rerun a record, verify hash, export JSON for UI fixtures
    testdata/scenarios/*.scn   behaviour scenarios (section 3)
    tools/crap/                CRAP calculator (cover profile + AST complexity), itself TDD'd
  web/                         Vite + TypeScript, no framework lock-in
    src/protocol/              TS types generated from Go (tygo) - the only contract with the engine
    src/net/                   WS + REST client
    src/store/                 state reducer: protocol events -> view model (pure, unit-tested)
    src/render/                Renderer interface; canvas2d implementation (PixiJS later, same interface)
    src/views/tv/              arena + x-ray + sidebar
    src/views/editor/          my bot: paste box, errors, templates, "Copy AI prompt", live slots/trail
    src/views/leaderboard/
    fixtures/                  replay JSON exported from engine scenarios
  Makefile                     test, cover, crap, lint, check, gen (TS types), fixtures
```

## 2. Core design

**Domain (pure).**
- `sim.Resolve(w World, acts map[BotID]Action, r Rules) (World, []event.Event)` - a pure function. The beat order
  from VISION (moves -> shots -> bullets -> scans -> hits -> coins -> respawn) is a list of `Phase` values, each
  `Apply(*beatCtx)`. New rules = new phase or new Rules field (open/closed); each phase is small and tested alone.
- Randomness inside the sim (coin spawn, respawn spot) comes from a seeded PCG in the World, never global.
- The sim knows nothing about scripts. It only sees `Action{Kind, Dir}` per bot.

**Brain port.**
```go
type Brain interface {
    // Decide gets the result of the previous beat and returns this beat's action.
    // It must be deterministic for the same inputs and seed.
    Decide(p Perception) Action
}
```
- `Perception` = readings, x/y, hp, loaded, bonus, trail, interrupt (hit/bump from).
- Implementations: `ScriptedBrain` (fixed action list, for scenarios), `lang.ScriptBrain` (BOTSCRIPT),
  house bots = BOTSCRIPT programs from `lang/bots/*.js`.
- This split is what makes the engine testable without the language, and the language testable without the arena.

**Language.** goja parser -> validator (whitelist from LANGUAGE.md section 8, call graph check for recursion,
depth <= 3, 120 lines) -> tree-walking interpreter with op counting and integer clamping. Execution model: the
interpreter runs as a **resumable stepper** (one goroutine per bot, but lockstep: the orchestrator gives it a budget
of 100000 ops and waits until it either queues an action or spends the budget). No wall clock is involved, so the
result is the same on any machine. Interrupts unwind via a sentinel panic, as in the spec.

**App.**
- `Match` owns the tick loop: for each beat, collect `Decide` from all brains (sorted by BotID), `Resolve`,
  publish frame + events, feed perceptions back. Time comes from the `Clock` port, so tests drive beats by hand.
- `Players`: join creates a session token + bot (with the Rookie template). The token is the only "auth".
- `HotSwap`: validate, then apply at the next beat boundary; rate limit 1 per 3 beats; logged in the edit log.
- `Leaderboard`: a projection over events: `kills`, `deaths`, `coins`, `score` per VISION section 9 (kill +3, repeat kill on the same victim +1, coin +1, configurable: `app.Points`, flags `-coin-points`, death -1, floor 0). Persisted through `Store`.
- Arena mode for v1: **one continuous arena**, bots join/leave any time, respawn after death, house bots fill up to
  a minimum count. Rounds/edit windows from VISION are **[later]**. Since step 44 a game can end on an admin timer:
  the arena stops (no beats, no bot goroutines), the board is archived as the last game (`last.json`), joins/edits/kicks
  queue until the admin starts the next game (`Arena.StartGame`, a fresh match as in `ApplyMap`, empty board). The timer
  is not persisted; a restart is a running game with no timer.

**Replay.** `Record{Rules, Map, Seed, Programs (initial), EditLog [(beat, botID, source)], Joins/Leaves}`.
`Replay(record)` runs a fresh Match on a ManualClock and returns the event log. The event log is hashed (SHA-256
of canonical JSON). Same record -> same hash, always.

## 3. Scenarios and replay testing (the heart of the plan)

**`.scn` files** - small, readable, one behaviour each. Example:

```
name: target that moves on the beat you fire wins
rules: default            # or overrides: reload=3 bullet_speed=3
seed: 1
map: |
  #######
  #A...B#
  #.....#
  #######
bots:
  A: script fire E
  B: script move S
beats: 2
expect:
  beat 1: Fired A E
  beat 1: Moved B S
  beat 2: no Hit
  final: B.hp 3
frame 1: |                # optional ASCII snapshot after beat 1
  #######
  #A..*.#
  #....B#
  #######
```

- Bots can be `script <actions...>` (ScriptedBrain) or `program <file.js>` / inline BOTSCRIPT (real interpreter).
- `expect` lines match events **by beat, any order within a beat** (robust to unrelated events; each line consumes a distinct event); `no X` asserts absence in that beat;
  `final:`/`at beat N:` assert state; `frame N:` compares an ASCII render of the world.
- Runner: `scenario_test.go` loads every `.scn` as a subtest (`go test -run Scenarios/move_into_wall`).
- `-update` flag writes the full event log as a golden file next to the scenario (`.golden.jsonl`); goldens are
  reviewed in diffs, so any behaviour change in the engine is visible.

**Determinism proofs:**
1. **Replay equality:** every scenario is run twice and once more via `Replay(Record)`; all three hashes must match.
2. **Fuzz/property:** `FuzzMatch` generates random small maps (5x5..9x9), 2-4 bots with random house programs and
   random edit logs; asserts run twice -> same hash, plus invariants (no two bots in one cell, HP in 0..3, bullets
   only in-bounds, kills == deaths, leaderboard = projection of events).
3. **Scheduler independence:** the replay test runs under `-race` and with `GOMAXPROCS=1` and `=8`, same hash.
4. **Long goldens:** 3 recorded matches (2 bots/5x5, 4 bots/9x9, Rookie vs Lighthouse vs Tracker) of ~300 beats
   kept as golden hashes - a cheap regression net for the full engine.

**UI fixtures:** `cmd/replay -export` turns any scenario/match into the same JSON the WS sends, written to
`web/fixtures/`. The UI is developed and tested against these without a running engine.

## 4. Web and sessions

- `GET /` TV view (arena + leaderboard sidebar). `GET /bot` my bot editor. `GET /leaderboard`.
- **Join by link:** first visit to `/bot` -> `POST /api/join {name}` -> `{token, botId}`; token kept in
  `localStorage` and sent as `Authorization: Bearer <token>`. Lost token = new bot. No accounts.
- `GET/PUT /api/bot` (own bot only, token checked in app layer; GET also returns the bot's `look`; since step 51 a PUT in a running
  game reboots the bot: no 429 for saves, `GET` adds `rebootLeftBeats` / `rebootTotalBeats`, `-reboot` flag, default 10 s; with the reboot on, a return after the
  player's own Leave answers 429 until `RebootBeats` beats have passed since the leave landed),
  `PUT /api/bot/profile {name, look}` -> 204 (`Arena.SetProfile`: rename with the Join rules + look, no hot-swap rate limit,
  the latest call per beat wins; 400 with the bad field for a bad name or look, 401/403 as the other bot routes;
  looks are cosmetic and never enter the Record or any hash; the roster, `bots[]` and `fallen[]` of WS frames carry `look`),
  `POST /api/bot/leave` / `POST /api/bot/return` (step 50: detach the bot from the arena and bring it back, token/name/look/program/board line
  kept; 204, 409 when full, 429 + `retryBeats`; `GET /api/bot` adds `arena{in,leftBy,stayLeftBeats}` + `beatMs`; every stay is capped by `-max-stay`,
  default 24 h, in beats of the match), `GET /api/leaderboard`, `GET /ws` (frames + events,
  read-only for everyone). Validation errors come back with line/column + hint.
- **UI modularity:** views depend only on `store` (view model) and `render.Renderer`; `store` depends only on
  `protocol`. Replacing the whole look = new `render/` + new `views/`, nothing else changes. A `?fixture=name` query
  param plays a recorded fixture instead of the live WS.
- **Admin:** two doors. (1) The running `cmd/arena` reads commands from stdin (type in the terminal of `make run`):
  `list` (id, name, human/house, score per bot), `kick <id|name>` (removes a human bot like `Leave`, deletes its
  leaderboard line, its token then gets 401 so the browser shows "Add my bot"; house bots are refused; an ambiguous
  name lists the matches), `reset-board` (wipes the leaderboard), `min-bots <n>` (house-bot fill level), `help`. EOF on
  stdin ends the console only. (2) `/admin` in the browser, on only when env `ARENA_ADMIN_PASSWORD` is set (else
  `/api/admin/*` is 404): `POST /api/admin/login` sets an `HttpOnly; SameSite=Strict` session cookie (12 h, in memory,
  not `Secure` because the arena runs on plain http); wrong password = 401 after a delay, 5 failures per minute per IP
  = 429; the other routes (`status`, `kick`, `reset-board`, `min-bots`) need the cookie and mutate by POST only.
  Time (expiry, rate window) lives in the adapter behind an injectable clock. House bots are never kicked: the admin
  sets their number with `min-bots`.
  Map: `POST /api/admin/map/preview {width, height, walls?, seed?}` -> `{seed, map}` (a generated room, `geo.Generate`; no seed
  = a random one from the adapter) and `POST /api/admin/map/apply {map}` (`Arena.ApplyMap`: a new game on that map at
  the next beat, players/tokens/programs and the leaderboard kept, seed + 1, a fresh live Record per game, the map
  saved as `map.txt` and used at the next start unless `-map` is given; `status` also reports `mapWidth`/`mapHeight`).
  Game timer: `POST /api/admin/timer {minutes}` (the adapter converts minutes to beats with the beat length; 0 cancels;
  409 once the game is over) and `POST /api/admin/start` (409 while a game runs); `status` adds `running`, `remaining`
  (beats, -1 = no timer) and `beatMs`. Console: `timer <minutes>`, `start`. `GET /api/leaderboard/last` is the final
  board of the latest finished game. WS messages carry `remaining`, `gameOver`, `beatMs`; while the game is over the
  hub snapshot is the final update (`gameOver: true`), so a new TV shows "Game over" and the final board.
- **Check (public):** `POST /api/check` (raw program as body) and `GET /api/check?src=...` -> `{ok:false, errors:[{line,
  col, message, hint}]}` or `{ok:true, trial:{beats, deadBeats, actions{scan,move,fire,wait}, idle, noAction, clicks,
  hitsTaken, hits, kills, deaths, coins, warnings[]}}` (`app.Check`: validation, then a 100-beat trial in
  a small fixed 9x7 trial room against hound, forager and tracker, fixed seed, no clock, no live arena); 16 KB limit (413 for the body, 400 for
  `src`), 30 per minute per remote IP then 429 (own limiter type, injectable clock, bounded table). `GET /api/config`
  -> `{baseUrl}` from env `ARENA_BASE_URL` (trailing slash trimmed, `""` when unset). The rulebook ("Check your
  program" section) uses it, else `location.origin`.
- Placeholder look: dark grid, one color per bot, scan cones as translucent wedges, tracers for bullets, a sidebar
  table (name, HP, kills, deaths, score). Deliberately plain, to be redesigned.

## 5. Defaults for gaps in the spec (decide now, revisit after sim)

| Gap | Default |
|---|---|
| Two bots move into the same cell | both fail, both get `onBump` from each other's direction |
| Two bots swap cells | both fail, both bump |
| Cone geometry for `scan(d)` | cells with perpendicular offset <= distance along `d` (90 degree wedge); level 3 = offset 0, 2 = offset 1, 1 = other |
| Move into a cell occupied at beat start | fails with bump even if the occupant moves away (chains/rotations fail) |
| Bump direction | always the mover's own direction, incl. perpendicular claims |
| Bullet meets bullet | pass through each other |
| Bullet path within a beat | checked cell by cell (no tunnelling through a bot) |
| Respawn spot | the empty cell with max min-distance to enemies; ties broken by seeded RNG |
| Bullet reaching a bot killed earlier in the same beat | spent, no `Hit` event |
| Shield | bullet spent, no damage, no `Hit`, no interrupt; shielded bot acts normally |
| Coin spawn | every `coin_every`=50 beats while fewer than `coin_max`=5 on the map (see TUNING.md), uniform free cell; coin consumed even at bonus cap |
| Kill credit | shooter of the killing bullet, even if dead; self-kill = `Killed B B` |
| `true` / `false` in BOTSCRIPT | accepted as 1 / 0 (`null` rejected); call depth = user functions on the stack incl. entry point, max 3 |
| Script after respawn | fresh `loop()`, handler mode cleared; slots and dice kept, trail cleared |
| Bump during `onBump` | restarts `onBump` (mirrors the hit rule) |
| Leaderboard scope | all-time per bot, persisted in a JSON file; reset by deleting the file |
| Memory index out of range (`num[9]`, `pin[-1]`) | constant index: rejected by the validator; computed index: read gives 0 / `none`, write is a no-op |
| Memory after death / hot-swap | `num` and `pin` are kept (as the old slots were) |
| `pin` values in user functions | not allowed as parameters (numbers only); pass the index and read `pin[i]` |
| `path` / `steps` / `nearest` distance | walking distance over terrain (BFS, walls only, bots ignored); `path` ties: N, E, S, W order; `nearest` ties: lowest index |
| `heard` (a place) | the cell of the nearest shooter (real shot, Manhattan, within hear_range) after moves; ties go to the lowest bot id; my own shots and clicks are silent; walls do not block sound; kept until a newer sound, `none` before any, reset on respawn; no `heardAgo` |
| Negative action direction | `move` / `scan` / `fire` with dir < 0 = `wait()`; dir >= 0 wraps mod 4 |
| Memory on screens after `a b c` removal | the TV has no memory badge (removed in the TV redesign); all 8 nums and pins on the phone live view; the TV draws no pin markers (removed in step 38) |

---

## Steps

### Step 1 - Scaffold and quality gates [DONE]
Go module, Makefile (`test`, `cover`, `crap`, `lint`, `check`), golangci-lint config (gocyclo, gocognit,
forbidigo, depguard). Build `tools/crap` test-first: parse cover profile + compute cyclomatic complexity per
function, print offenders, exit 1 if any CRAP >= 8. Done when `make check` runs green on an empty project and red on
a planted complex untested function.

### Step 2 - Geometry and maps [DONE]
`geo`: Dir (N E S W = 0..3, left/right/back), Pos, Grid, ASCII map parse/render (`#`, `.`, bot letters, `$` coin).
Table tests for turning, bounds, parse errors.

### Step 3 - Scenario harness [DONE]
`.scn` parser, ScriptedBrain, a minimal Match runner over a stub `Resolve` that does nothing, expectation matcher
(per-beat unordered match, `no`, `final`, `frame`), golden `-update`. Tests of the harness itself use tiny fake event logs.
First real scenario: "bots that wait stay put".

### Step 4 - Moves [DONE]
Move phase: free cell, wall no-op (`ok=false`, no event), bump into a bot (mover gets bump, the other nothing),
same-cell conflict, swap conflict. One scenario per row of this list.

### Step 5 - Scan [DONE]
Cone geometry and 0-3 levels for enemy/coin/bullet (bullet only if flying toward the scanner), scans after moves.
Scan record event (cone cells + what was reported) for the TV x-ray. Scenarios on 5x5/7x7 maps for each level.

### Step 6 - Fire, reload, bonus [DONE]
Reload 3 beats, bonus shots (max 2, fire while reloading, lost on death), `fire` with nothing loaded = Click.
Scenarios: fire-reload-click sequence, bonus during reload, bonus cap.

### Step 7 - Bullets and hits [DONE]
Travel 3 cells/beat, cell-by-cell, stop at wall or bot, damage 1, hit direction, "target that moves wins",
distant dodge. Scenarios incl. the example in section 3. Also `scan_bullet_level{0..3}.scn` (bullet scan levels,
unit-tested only in Step 5).

### Step 8 - Death, respawn, coins [DONE]
HP 3 -> death, Killed event (killer, victim), respawn after 5 beats at the default spot, 2 beats shielded, coin
spawn (seeded), pickup -> bonus shot. Scenarios for each.

### Step 9 - Replay record and determinism suite [DONE]
`Record`, `Replay`, canonical event hashing, run-twice checks on all scenarios, `FuzzMatch` with invariants,
GOMAXPROCS/race runs. `cmd/sim` headless runner with stats (kills/beat, avg time to first hit) for tuning.

### Step 10 - BOTSCRIPT validator [DONE]
goja parse -> whitelist walk. Table tests: every allowed construct accepted, every forbidden one (let/var, strings,
arrays, closures, recursion, depth > 3, assignment to non-slots, > 120 lines) rejected with line/col + hint.

### Step 11 - Interpreter core [DONE]
Expressions, statements, integer division, clamping +-9999, slots, built-ins (`left/right/back/pick/chance/rand/
abs/min/max/dist`, seeded per bot), op counting. Pure unit tests, no arena.

### Step 12 - Actions, budget, interrupts [DONE]
Action built-ins block and yield `Action`; budget of 100000 ops -> IDLE and resume next beat; `loop()` without action
= wait; onHit/onBump unwind and restart rules from LANGUAGE.md section 3; trail and readings. `lang.ScriptBrain`
implements `brain.Brain`. Tests drive it with hand-made Perceptions.

### Step 13 - Program scenarios [DONE]
Rookie, Lighthouse, Tracker as house bots. Scenarios with real programs on small maps: Lighthouse dodges a bullet at
level 3, Tracker crosses over after a wrong sidestep, infinite loop = frozen bot (IDLE every beat), hot-swap keeps
slots/HP/reload. Long goldens (section 3, item 4).

### Step 14 - App layer [DONE]
Match with Clock port (ManualClock in tests), join/leave, house-bot filler, HotSwap with rate limit and edit log,
Leaderboard projection (kills, deaths, coins, score) + Store port. Tests: leaderboard equals a projection computed
independently from the event log; replay of a live session's Record gives the same hash.

### Step 15 - Adapters and server [DONE]
REST + WS + JSON store + real clock + `cmd/arena`; serves `web/dist`. httptest tests: join, edit own bot,
403 on someone else's bot, validation errors shape, WS sends frames. `make gen` produces TS protocol types (tygo).

### Step 16 - Web foundation [DONE]
Vite + TS + vitest. `protocol` (generated), `net`, `store` reducer tested against `fixtures/` JSON, fixture player.

### Step 17 - TV view [DONE]
`Renderer` interface + canvas2d impl: grid, bots, bullets as tracers, scan x-ray from scan records (never
recomputed), badges (line + slots), sidebar. Tested via store/view-model unit tests; rendering checked by hand on
fixtures.

### Step 18 - Editor and leaderboard views [DONE]
Paste box with error markers, templates, "Copy AI prompt" (LANGUAGE.md + rules + example bots), live slots/trail of
my bot, leaderboard table. Playwright smoke: open link -> join -> paste bot -> see it on TV.

### Step 19 - Tuning pass [DONE]
Use `cmd/sim` to tune beat length, bullet speed, reload, cone, arena size, coins (VISION section 12). Each tuned
value lands in `Rules` defaults with a scenario that pins it.

### Step 20 - Terrain sense: `wall(x, y)` [DONE]
A free built-in (no beat spent, counts as ops like any call) so scripts can read the static map around them and plan
dodges with `x`, `y`, `WIDTH`, `HEIGHT` (all already readable). `wall(x, y)` is true for a wall cell and for any cell
outside the arena, false for floor. Terrain only: bots, bullets and coins are never revealed (they still need a scan
or a bump). No `open(d)` helper: scripts compute neighbours themselves (e.g. `wall(x + 1, y)` for E).
- `lang`: built-in table entry (`wall(x, y)`, 2 args), validator accepts it, interpreter evaluates it through the
  script env; the env gets read-only access to the match's `geo.Grid` (via the Perception or the env, no new
  import direction - depguard must stay green). Deterministic: the map is fixed for the match.
- Tests first: interpreter unit tests (floor, wall, each edge, negative and past-the-edge coordinates, op cost),
  validator accepts `wall(1, 2)` and rejects wrong arity with a hint.
- Program scenarios (INDEX.md rows): a bot in a 1-wide corridor sees `bullet == 3`, finds both sides walled with
  `wall()`, steps back to a side opening and survives the next beat by sidestepping; a twin where the bullet is too
  close and it is still hit; a bot that reads `wall()` never learns about a bot standing in a floor cell.
- Docs: LANGUAGE.md section 5 (reading the world) and section 7 (built-ins, free), `make gen` for editor docs and
  the AI prompt.
- House bots and goldens are untouched in this step.

### Step 21 - House bots use the terrain [DONE]
Lighthouse and Tracker dodge with `wall()`: sidestep to an open side, and when both sides are walls, retreat along
the line (remembering the dodge in a slot) and sidestep on the next beat; Tracker also uses it instead of walking
into walls. Rookie stays the simplest template (optionally `pick` away from walls). Update LANGUAGE.md section 9
listings to match `engine/lang/bots/*.js`, the program scenarios that pin their behaviour, the long goldens
(`go test ./app -update`, review) and fixtures. Re-run the `cmd/sim` sweep from TUNING.md, record the new dodge
rate, kill pace and ladder (tracker > lighthouse > rookie must hold) in TUNING.md.

---

## Language v2: memory, navigation, hearing, coin radar (steps 22-28)

Goal: richer strategies (hunter, forager, mode switchers) that an AI can write and a non-tech player can still read.
Each new feature is one word in the AI prompt and needs no new syntax beyond array indexing.
Order matters: 22 changes the memory model every later step builds on. Step 21 runs first, unchanged.

**Target language surface (end of step 28):**
```js
num[0..7]                      // 8 integers, +-9999, start 0, kept through interrupts, hot-swap and death
pin[0..7]                      // 8 places, start none, same lifetime
none  here()  at(x, y)         // place values; pin[i].x, pin[i].y; pin[i] == none, pin[i] != pin[j]
free() count()                 // first empty pin index (-1 if full), number of pins set
path(p)  steps(p)  nearest()   // first step toward place p (-1 = unreachable or already there), walking distance (-1), index of the closest pin by walking (-1)
heard                          // place of the nearest sound last heard (kept until a newer one, none = never)
ping()                         // action: costs a beat, sets `pinged` = place of the nearest reachable coin, or none
```

### Step 22 - Typed memory, part 1: `num[0..7]` replaces `a b c` [DONE]
Remove the slots `a b c` completely and add `num[0]`..`num[7]` with the same semantics (integers, clamp +-9999,
start 0, kept through interrupts, hot-swap and respawn).
- `lang` validator: `num[expr]` is the only writable target (`= += -= ++ --`, also as `for` init/update); a constant
  index outside 0..7 is rejected with a hint; `a`, `b`, `c` become unknown names with the hint "slots a b c were
  removed: use num[0], num[1], num[2]". Member/index syntax stays forbidden everywhere else (only `trail[i].f` and
  `num[i]` / later `pin[i]`).
- Interpreter: memory struct `Mem{Num [8]int}` replaces the 3 slots; computed index out of range: read 0, write no-op
  (section 5). Indexing counts ops like any expression.
- App/protocol/web: `BotState.Slots [3]int` and protocol `Slots` -> `Num [8]int`; probes keyed `num_0..num_7`; no TV
  memory badge (section 5); phone live view shows all 8. `make gen`.
- Rewrite house bots (`lang/bots/*.js`), every inline program in `testdata/scenarios/*.scn`, LANGUAGE.md sections
  4, 8, 9, 11 and VISION.md section 5 ("Memory"). Rename the `hot_swap_keeps_slots` scenario to `..._keeps_memory`.
- Tests first: validator table (accept `num[0] = 1`, `num[a2]` via param, `num[i]++`; reject `a = 1`, `num[8]`,
  `num = 1`, `num[0].x`), interpreter (clamp, out of range read/write, survives unwind), hot-swap and respawn keep
  memory. Goldens and long hashes must stay identical once the programs are rewritten 1:1 (`a` -> `num[0]` ...): this
  is the proof that the rename changed no behaviour.

### Step 23 - Typed memory, part 2: places `pin[0..7]` [DONE]
Add the place type. Values: `none`, `here()`, `at(x, y)`, `pin[i]`, later `pinged`. Allowed uses: assign to `pin[i]`,
compare with `==` / `!=` (place vs place), read `.x` / `.y` (`none.x` = -1), pass to place built-ins (`path`,
`steps`). Everything else is a type error.
- Validator: a small static type check (number vs place) over expressions; the language has no variables except
  params, so typing is local: params are numbers, `pin[i]` / `here()` / `at()` / `none` / `pinged` are places. Errors
  with hints, e.g. `pin[0] = 5` -> "a pin holds a place: use at(x, y) or here()", `pin[0] + 1` -> "places have no
  arithmetic: use pin[0].x". Passing a place to a user function is rejected (section 5).
- Interpreter: `Mem.Pin [8]Place` (`Place{X, Y int; Set bool}`); `at(x, y)` stores the coordinates as given (no
  bounds check; off-map places are just unreachable). `free()`, `count()` built-ins.
- App/protocol/web: pins in `BotState` and protocol `Bot` (`[]{x, y} | null` per index); phone live view lists pins;
  TV draws each set pin as a small marker in the bot's colour with its index, outline-only above 6 bots (VISION 8
  clutter rule). Store/view-model unit tests, check by hand on a fixture.
- Docs: LANGUAGE.md sections 4, 5, 7, 8; `make gen`.
- Tests first: type checker table (every allowed and rejected form above), interpreter (`here()` after a move,
  equality, `.x` / `.y`, `none`, `free` / `count` on empty, partial and full memory), program scenario: a bot pins
  its start cell, walks away, comes back by comparing `here()` with the pin.

### Step 24 - Navigation: `path(p)`, `steps(p)`, `nearest()` [DONE]
- `geo`: `Grid.Distances(to Pos) []int` - BFS over floor cells (walls only, bots ignored), deterministic. Grids are
  static per match, so a per-match cache keyed by target is allowed (pure, no shared mutable state across bots:
  each brain owns its cache, or the cache is built before goroutines start).
- `lang` built-ins (free, ops like any call): `path(p)` = first step toward `p` on a shortest path, ties N, E, S, W;
  -1 if `p` is `none`, unreachable or my own cell. `steps(p)` = walking distance, -1 if unreachable/`none`.
  `nearest()` = index of the set pin with the smallest `steps`, ties lowest index, -1 if none reachable.
- Docs: LANGUAGE.md section 7, an example in section 9; `make gen`.
- Tests first: `geo` BFS tables (open room, corridor, dead end, unreachable pocket, target is a wall, off-map),
  tie order; interpreter tests for the three built-ins; program scenario: a bot walks around a pillar to a pinned
  cell in the minimal beats; another where the pin is sealed off and `path` returns -1.

### Step 25 - Hearing: `heard` [DONE]
Gunfire is audible. Firing now reveals your rough position, so shooting becomes a choice.
- `sim`: `Rules.HearRange` (default 6, Manhattan, scenario key `hear_range`, min 0 = deaf world). After the fire
  phase, each bot gets `Heard` = direction to the nearest real shot (loaded or bonus; clicks are silent) fired this
  beat by another bot within range, else -1 (section 5 for direction and ties). Shooter position = after moves.
- `brain.Perception.Heard`, `HeardAgo`; `lang` readings `heard` and `heardAgo` (numbers). `heard` keeps the last
  heard direction until a newer shot is heard (-1 before any); `heardAgo` = beats since that shot (0 = last beat,
  capped at 99, 99 before any). Both reset on respawn, like the trail. Kept per bot in the sim World (replayable).
- TV: a faint sound ring of radius `HearRange` around every shooter on its fire beat (from the existing `Fired`
  event + rules in the frame; no recomputation of who heard what). `make gen`.
- Docs: LANGUAGE.md sections 1, 5; VISION.md section 3 table (new row, [default]).
- Tests first: scenarios (INDEX.md rows) for: heard from each of N E S W, diagonal tie -> N/S, out of range -> -1,
  through a wall still heard, own shot not heard, click not heard, two shooters -> nearest wins, `heard` kept and
  `heardAgo` counting on silent beats, a newer shot replaces it, reset on respawn; program scenario: a bot with
  `if (heard >= 0 && heardAgo <= 2) move(heard)` walks toward a duel.

### Step 26 - Coin radar: `ping()` [DONE]
- `brain.Kind` `Ping` (no direction); `lang` action `ping()`, `ActPing`, trail act constant `PING` (= 5, after IDLE);
  `ParseAction("ping")` for scripted scenarios.
- `sim`: resolved in the scan phase (post-move snapshot): the bot's `Pinged` = the nearest reachable coin by walking
  distance (step 24 BFS), ties row-major, or none. Persists until the next ping, like scan readings; does not
  touch scan readings. Silent to other bots. Event `Pinged{Bot, Found bool, X, Y}` for the TV.
- `lang`: place reading `pinged`; the canonical forager is `ping(); pin[0] = pinged; move(path(pin[0]));`.
- TV: a short radar ripple from the bot and a line to the found coin (x-ray rule: draw what the bot learned, from the
  event). `make gen`.
- Docs: LANGUAGE.md sections 3, 5, 7; VISION.md section 5 (a second sensing action).
- Tests first: scenarios: ping finds the nearest coin, prefers walking distance over straight-line distance across a
  wall, none on an empty map, tie order, persists across beats, a coin taken after the ping leaves a stale `pinged`;
  program scenario: a forager pings, walks, and takes a coin.

### Step 27 - Scoring: coins make foraging a real role [DONE]
- Coin points become configurable (leaderboard config, not hashed: the event log is unchanged). Default **+1** (chosen by the
  step 28 sweep). VISION.md section 9, leaderboard tests (projection equality still holds).

### Step 28 - House bots, templates, AI prompt, tuning [DONE]
- Rewrite Lighthouse and Tracker to use `num` / `pin` where it reads better; Rookie stays minimal.
- New house bots: **Forager** (ping, pin, path; flees on `enemy >= 2` unless loaded and healthy) and **Hound**
  (follows `heard`, scans on arrival, fires on 3, pins the last sighting). Add them to the editor templates.
- AI prompt (`make gen`): LANGUAGE.md section 9 lists all five bots and one short "strategy recipe" line each
  (hunter, forager, mode switch on `hp` / `loaded`).
- Program scenarios that pin the key behaviour of each new bot; long goldens updated (`go test ./app -update`,
  review); fixtures regenerated.
- `cmd/sim` sweep over `hear_range` (4, 6, 8) and coin points (1, 2, 3): record kills/beat, coins/beat and the
  per-bot ladder in TUNING.md. Pass criteria: Forager and Hound both beat Rookie, and neither pure Forager nor pure
  Hound wins every seed (no single dominant role). Land the chosen values as defaults with a scenario that pins them.
- **Balance round (first sweep: Forager topped 88-99% of seeds at every setting):**
  - **A ping is audible:** other bots within `hear_range` hear a ping exactly like a shot (same `heard` /
    `heardAgo` readings, same direction and tie rules); the TV draws the sound ring for it too. Hunters can now track
    foragers.
  - **Rarer coins:** sweep `coin_every` (10, 20, 30, 50) with audible pings and coin points 1/2; land the value
    that best spreads the winners.
  - **Negative direction = wait:** `move`, `scan`, `fire` with a direction < 0 spend the beat as `wait()` (trail act
    `WAIT`), so `move(path(p))` on an unreachable target stands still instead of walking W. Directions >= 0 keep
    wrapping mod 4.

---

## Lobby and polish (steps 29-32)

Owner requests: a coin sprite with some life, a cleaner score table, a one-tap way in, admin removal of bots, and a
starting field of 5 house bots that real players replace one by one.

### Step 29 - Coin sprite [DONE]
- Use `coin.png` from the repo root (512x512 RGBA): copy it into the web assets (e.g. `web/public/coin.png`), loaded
  once by the canvas renderer; a plain gold circle stays as the fallback until the image has loaded.
- Animation, render-side only (no sim or protocol change): a slow idle bob (1-2 px), and every few seconds a coin
  does a quick spin (horizontal scale through `cos`) or a small hop; each coin gets its own phase from its cell so
  they do not move in sync. Ping lines and x-ray silhouettes of coins keep working.
- Tests: the pure animation function (phase per cell, bob range, spin / hop windows) in vitest; drawing checked by
  hand on a fixture with coins.

### Step 30 - Score table: kills, deaths, coins [DONE]
- TV sidebar (`web/src/views/tv/sidebar*.ts`): remove the `state` column, add a coins column. Column headers are
  icons: kills = crosshair, deaths = skull, coins = the coin sprite; keep `name`, `HP`, `score`. Same icon headers on
  the `/leaderboard` page (which already has a coins column).
- Coins = the bot's leaderboard `coins` counter (already in the board entries); bots without a board entry show 0.
- Tests: `sidebarRows` / `buildRows` unit tests for the coins value and the column set.

### Step 31 - Join flow: 5 house bots, "Add my bot" [DONE]
- The arena starts with **5 house bots** (`-min-bots` default 5, TUNING.md note); each human who joins replaces one
  house bot (existing balance logic: the oldest house bot leaves first); a leaving or removed human is replaced by a
  house bot again. Pin it with an app test: 5 house bots at start, after 2 joins 3 house + 2 humans, after a leave
  4 + 1.
- A big **"Add my bot"** button on the TV and leaderboard views when this browser has no bot (no token in
  `localStorage`, or the stored token is rejected). It opens `/bot`, where the existing join form asks for a name
  (required, 1..max runes, trimmed - already enforced by `Arena.Join`). Hidden once I have a bot. If the arena is
  full of humans, the editor shows the existing "full" state.
- Tests: view-model tests for when the button shows; the editor join flow tests stay green; Playwright smoke if the
  existing e2e covers the join.

### Step 32 - Admin console [DONE]
- No auth exists, so admin acts only from the server side: the running `cmd/arena` reads commands from **stdin**,
  one per line:
  - `list` - every bot: id, name, human/house, score.
  - `kick <id|name>` - removes a human bot from the arena (same path as `Leave`, by id; a name must match exactly
    one bot, else an error lists the matches) and deletes its leaderboard line; its token stops working, so that
    browser sees "Add my bot" again. House bots cannot be kicked (error message).
  - `help`; unknown commands print a hint. Output goes to stdout; stdin closed (EOF) just ends the console, the server
    keeps running.
- `app`: `Arena.Kick(id string) error` (and a board `Remove`) next to `Leave`; persisted on the next save.
- Tests first: app tests (kick by id, unknown id, house bot refused, board line gone after save/restore, token
  rejected after), `cmd/arena` console tests with a fake stdin/stdout (list, kick by name, ambiguous name, EOF).
- Docs: a short "Admin" section in `arena/README` (or the existing run docs) and the Makefile `run` target comment.

## Hearing cleanup (steps 33-34)

Owner requests: `heard` is just a coordinate, no time; the TV sound diamonds are visual noise, remove them.

### Step 33 - `heard` is a place [DONE]
- `heard` becomes a **place** (like `pinged`, section 4 of LANGUAGE.md): the cell of the nearest bot that fired a real
  shot or pinged within `hear_range` on the latest beat it heard anything (shooter/pinger position after moves; ties
  go to the lowest bot id; own sounds and clicks are silent). Kept until a newer sound is heard, `none` before any,
  reset on respawn. Read-only: `pin[i] = heard`, `heard == none`, `heard.x` / `heard.y`, `path(heard)`,
  `steps(heard)` all work; assigning to `heard` is a validation error with a hint (same as `pinged`).
- Remove `heardAgo` entirely (sim state, `brain.Perception`, `lang` reading, docs, generated docs/AI prompt). Using it
  is now an unknown-name validation error.
- House bots and templates that used `heard`/`heardAgo` (Hound and any other) are rewritten for the place form
  (e.g. a new sound = `heard != pin[k]`, then `pin[k] = heard` and walk `path(pin[k])`). Balance test from step 28
  must stay green (no bot tops more than its allowed share of seeds); retune only the bot programs, not rules.
- Scenarios: update the step 25 hearing scenarios and goldens to the place form (from each side, out of range ->
  none, through a wall, own shot silent, click silent, nearest of two wins, tie -> lowest id, kept on silent beats,
  newer sound replaces it, reset on respawn, ping heard). Determinism hashes regenerated.
- Docs: LANGUAGE.md sections 1, 5 and recipes; VISION.md section 3 row; `make gen`.

### Step 34 - No sound diamonds on TV [DONE]
- Remove the TV sound diamond (`SoundRing`, `scene.sounds`, its drawing and the scene-building code and tests). The
  `Fired` and ping events and the bullet / ping drawing stay as they are. VISION.md section 3 row no longer mentions
  a TV ring.

## Coin pickup effect (step 35)

### Step 35 - Coin pickup: overlap, shake, golden glow [DONE]
- Render-side only (no sim or protocol change), from the existing `CoinTaken` event in the frame.
- On a beat with `CoinTaken{Bot, X, Y}`, the taken coin is still drawn at its cell during the move phase, under the
  bots, so the taker visibly glides onto it and covers it (today it just vanishes when the frame arrives).
- Effect phase (`scene.effect` 0..1, like hit blinks): the coin shrinks and fades away under the bot; the taker
  shakes a little (a few quick sideways jitters, amplitude about 0.06 cell, decaying to 0) and gets a soft golden
  glow (radial gradient around the bot, peaking early and fading out by the end of the effect). A bot that took a coin
  and was hit on the same beat shows both. Respects the step 29 coin sprite and its fallback disc.
- Tests first (vitest): the scene builder keeps taken coins as a separate `taken` list with the taker id; pure
  functions for shake offset (0 at effect 0 and 1, bounded amplitude) and glow alpha (0 at the end); the tween keeps
  taken coins through the move phase; canvas draws the taken coin under bots. Drawing checked by eye on a fixture
  with a pickup.

## Continuous bullets (step 36)

### Step 36 - Bullets fly at a constant speed
Today a bullet eases over `MOVE_MS` (600 ms) and then stands still until the next beat (1000 ms), so it moves in jumps.
- Render-side only. Bullets move **linearly** (no ease) at a constant speed of `BULLET_SPEED` cells per beat,
  stretched over the **whole beat interval**, so a bullet in flight never stops between beats and its head on the
  next beat starts where it was last drawn (no jump back, no stall).
- Beat interval: `TweenRenderer` learns it from the time between successive scenes (live server or fixture
  player), smoothed and clamped (e.g. 200..3000 ms), default 1000 ms before two scenes arrived; a long gap (pause,
  reconnect) does not stretch the next flight.
- Hits and walls: a bullet that stops short arrives after `distance / BULLET_SPEED` of the beat; the victim's hit
  blink, HP drain and death burst start when its bullet arrives (not at the fixed move end), so the hit lands with the
  bullet. Other effects (pickup from step 35, non-bullet deaths) keep their current timing. Bots keep their eased
  600 ms glide.
- Tests first (vitest, fake `Frames` clock): linear head position over the beat; head continuity across two beats
  (last drawn point of beat n == first point of beat n+1 for a flying bullet); interval learning (smoothing, clamp,
  long gap ignored); hit effect starts at arrival time; bots still eased. Checked by eye on `duel_5x5` fixture.

## No coin radar (step 37)

Owner request: `ping()` makes coins too easy to find; bots must find coins with `scan` and their own guessing.

### Step 37 - Remove `ping()` [DONE]
- Remove the `ping()` action end to end: `brain.Kind` `Ping`, `lang` action `ping()`, trail act `PING` (acts are
  0-4 again), `ParseAction("ping")`, the sim ping resolution, the `pinged` reading and per-bot state, the
  `Pinged` event, the protocol/web ping mark (`PingMark`, `scene.pings`, its drawing and scene-building) and
  generated docs/AI prompt. Using `ping()` or `pinged` is now an unknown-name validation error.
- Hearing: only real shots are audible now (step 28's audible ping goes away); `heard` stays a place (step 33).
- `path`, `steps`, `nearest`, `pin`, `num` stay - a bot can still pin a coin it saw on a scan (from `x`, `y`, `last`
  and `coin`) and walk to it.
- **Forager** is rewritten without `ping`: scan around, when `coin > 0` pin the coin cell it saw and walk there with
  `path`, otherwise explore (e.g. walk toward unvisited quarters with pins); keep its flee rule. Update its template
  and LANGUAGE.md recipes (section 9 strategy lines, the canonical forager example in section 7).
- Balance: rerun the step 28 `cmd/sim` sweep at the current defaults; pass criteria as in step 28 (Forager and
  Hound both beat Rookie; no bot tops every seed). If it fails, retune `coin_every` only (sweep 20, 30, 50) and
  record the ladder in TUNING.md.
- Scenarios: delete the ping scenarios and the "ping heard" hearing scenarios (INDEX.md rows too); add a program
  scenario where the scan-based Forager sees a coin, walks and takes it; regenerate goldens, long hashes, fixtures.
  Old records with a `ping` action: records v4 are unreleased, so no compatibility needed (note it in record.go
  addedRules comments if they mention ping).
- Docs: LANGUAGE.md sections 1, 3, 5, 7, 9; VISION.md sections 3 and 5; PLAN.md section 5 defaults rows mentioning
  ping; `make gen`.

## TV cleanup (steps 38-39)

### Step 38 - No pin markers on TV [DONE]
- Remove the pin markers drawn on the arena (`PinMark`, `scene.pins`, `pinMarks`, `PIN_OUTLINE_ABOVE`, the canvas
  `pin()` drawing and their tests). Keep pins in the protocol/store only if something else still reads them (e.g. the
  editor memory panel); otherwise drop them end to end. VISION.md section 8 (TV) and any doc that mentions pin
  markers on TV are updated; `make gen`.

### Step 39 - Death while moving: no flicker, no jump [DONE]
Owner saw flickering / jumping when a bot dies on a beat it also moved.
- First reproduce it in tests with a fake `Frames` clock: a bot that moves one cell and is killed that beat (bullet
  lands at its post-move cell, step 36 timing), then the next scenes (corpse/fallen phase, respawn after
  `respawn_beats`). Trace what is drawn per frame: live sprite vs `fallen` sprite position, glide start point, hit
  blink, death burst, HP bar, and the carried-effect logic from step 36.
- Expected look: the bot glides from its old cell to the new cell (eased, like any move), the bullet reaches it there,
  then it blinks and bursts at the new cell; it is drawn exactly once per frame (never live and fallen at the same
  time, never at the old cell after the glide), no snap back, and no 0-HP live sprite. On respawn it appears at the
  spawn cell without gliding across the map. Same for a bot that dies without moving and one that moves into a
  bullet.
- Fix every cause found; tests pin each one (per-frame positions and which sprite is drawn). Report the causes.

## Scan timing (step 40)

Owner request: a scan should see a shot fired on the same beat. Analysis (cmd/sim, 19x13, forager, hound, lighthouse,
tracker, rookie, 200 x 300 beats) compared scans after shots (before flight) with scans after flight; the owner chose
scans after flight: kills 2422 -> 2243, accuracy 0.77 -> 0.72, Hound and Forager still beat Rookie.

### Step 40 - Scans resolve after bullets fly [DONE]
- New beat order: **moves -> new shots -> hearing -> reload -> bullets fly -> scans -> hits -> coins -> respawn**
  (`phases` in `domain/sim/resolve.go`). A scan now sees the world after this beat's flight: a bullet fired this beat
  that is still in the air is counted; a bullet that hit a wall or a bot this beat is gone and not counted. Enemy and
  coin readings are unchanged in meaning (bots already moved; bots killed this beat are still on the grid at scan
  time, as before; coins taken this beat are still seen, as before).
- What a bot sees: `bullet > 0` means a bullet still flying toward it after this beat, so a sidestep on the next beat
  (moves resolve before flight) always dodges it. A target within `bullet_speed` cells of the shooter is still hit on
  the shot beat; a target that moves on the beat you fire still wins.
- Tests first: sim unit tests and scenarios for (a) A fires at B 5 cells away while B scans toward A on the same
  beat -> B reads `bullet` 3; (b) B sidesteps next beat -> no Hit; (c) a bullet that hits a wall this beat is not
  seen; (d) a bullet that hits another bot this beat is not seen by a scanner behind it; (e) a scanner hit on the scan
  beat still gets its interrupt. INDEX.md rows for the beat order rule. Fix any existing scenario whose expectation
  relied on the old order (say which and why in the report).
- Records: v4 is unreleased, so no compatibility gate; note in the `RecordVersion` comment (record.go) that v4 also
  carries scans after bullet flight. Regenerate goldens, long golden hashes (`hashes.txt`), web fixtures.
- Web: check the TV still draws scan cones and seen objects correctly with the new event order inside a beat
  (reducer, scene); no visual change expected.
- Docs: LANGUAGE.md section 1 (beat order) and section 6 (what `bullet` counts); VISION.md beat order row; PLAN.md
  section 2 beat order; phase doc comments (`scanPhase`, `firePhase`, `bulletPhase`); TUNING.md: a "Scan timing (Step
  40)" entry with the rerun of the step 37 sweep (same lineup, 200 matches x 300 beats, seed 1) and the pass criteria
  (Forager and Hound beat Rookie; no bot tops every seed over 100 single-match seeds); `make gen`.

## Deployment and web admin (steps 41-44)

Owner request: a tiny Docker image on one port the owner reserves, and a password-protected `/admin` page to run the
arena: kick bots, reset the leaderboard, generate and pick a new map size, and a global game timer. When the timer
runs out the game freezes, its leaderboard is kept as the "last game" board, and the admin starts the next game by hand.

Owner decisions: map = admin types a size, the server generates a room with a few random walls, the admin can
regenerate until they like it and then applies it; timer end = freeze + archive (only the latest finished board is
kept) + manual start of a fresh game (empty board, same players and programs); "reset leaderboard" just wipes the
live board (no archive), the game keeps running.

### Step 41 - Docker image [DONE]
- `arena/Dockerfile` (build context `arena/`), multi-stage: (1) `node` stage runs `npm ci && npm run build` in `web/`;
  (2) `golang` stage builds `cmd/arena` with `CGO_ENABLED=0 -trimpath -ldflags="-s -w"`; (3) final stage `FROM
  scratch` with only the binary, `web/dist` and an empty `/data` (owned by a non-root uid, `USER 65532`). `ENTRYPOINT
  ["/arena", "-web", "/web", "-data", "/data"]`, `CMD ["-addr", ":8080"]`, `EXPOSE 8080`, `VOLUME /data`. The host
  port is chosen at run time (`docker run -p <reserved>:8080 -v arena-data:/data ...`). No CA certs or tzdata needed.
- `arena/.dockerignore`: `node_modules`, `web/dist`, `data`, coverage files, `.git`.
- The binary must shut down cleanly on SIGTERM as PID 1 (it already handles SIGTERM; verify) and must not exit when
  stdin is closed (the console just ends at EOF; verify with `docker run` without `-i`).
- Makefile: `docker` target (`docker build -t arena .`, `IMAGE ?= arena`), `docker-run` (`PORT ?= 8080`, maps
  `$(PORT):8080`, named volume, passes `ARENA_ADMIN_PASSWORD` through for step 42). Not part of `make check`.
- Tests: none in Go beyond what exists; verify by hand: build the image, report its size (target: under 20 MB),
  run it, `curl /api/leaderboard` and `/` answer, `docker stop` exits within the grace period. Docs: a "Docker"
  section in `arena/README.md` (create it if missing; keep it short: build, run with a port, data volume).

### Step 42 - Admin auth and the `/admin` page: bots and leaderboard [DONE]
- **Password:** env `ARENA_ADMIN_PASSWORD` (flag `-admin-password-env` not needed; read the env in `cmd/arena`, never
  log it). Unset or empty = admin disabled: `/api/admin/*` answers 404 and `/admin` shows "admin is disabled".
- **Auth adapter** (`adapters/httpapi` or a new `adapters/admin`): `POST /api/admin/login {password}` compares in
  constant time (`crypto/subtle`); success sets an `HttpOnly; SameSite=Strict; Path=/api/admin` cookie holding a
  random session id (in memory, expires after 12 h, lost on restart); failure = 401 after a fixed small delay, and at
  most 5 failures per minute per remote IP (then 429). `POST /api/admin/logout`. Every other `/api/admin/*` route
  needs a valid session (401 otherwise). Mutating admin routes are POST only (SameSite cookie is the CSRF guard).
- **App:** `Arena.ResetBoard()` wipes the live leaderboard (all entries, persisted on the next save, broadcast on the
  next beat); `Arena.SetMinBots(n)` changes how many house bots fill the arena (0..floor cells; takes effect at the
  next beat through the existing balance logic). `Kick` already exists. Extend `Kick` to house bots only if it stays
  simple; otherwise house bots are controlled through `SetMinBots` (say which in the report).
- **Routes:** `GET /api/admin/status` -> `{bots: [{id, name, house, score}], minBots, beat}`; `POST
  /api/admin/kick {id}`; `POST /api/admin/reset-board`; `POST /api/admin/min-bots {n}`.
- **Web:** new route `/admin` (SPA route in `static.go` and `route.ts`): a password form, then a plain panel: bot
  table with a Kick button per human bot, house-bot count input, "Reset leaderboard" with a confirm. Refreshes status
  every few seconds. Not linked from the TV nav.
- The stdin console stays and gains `reset-board` and `min-bots <n>`.
- Tests first: app tests (reset board, persisted after save/restore, min-bots up and down), httpapi tests (disabled
  without a password, login ok/fail, 429 after 5 failures, cookie required, each route), web unit tests for the admin
  view model. Docs: PLAN.md section 4 Admin bullet, README admin section, Makefile `run` comment.

### Step 43 - Map generator and "apply map" [DONE]
- **Generator** (`domain/geo`, pure, deterministic from a seed with a local seeded PCG): `Generate(w, h, walls int,
  seed uint64) (string, error)` returns an ASCII room (format of `geo.ParseMap`, no bots): wall border, plus a few random
  interior walls: short straight segments (1-3 cells) and single pillars, `walls`% of the interior cells (0..15, default 3), never
  touching the border, and every floor cell reachable from every other (BFS; retry with the next draw until it holds).
  Size limits: 9..41 wide, 7..31 high (errors outside).
- **App:** `Arena.ApplyMap(text string) error` starts a new match on that map at the next beat boundary: same
  players, same programs and tokens, positions/HP/reload/memory/trail fresh (they rejoin like at server start), the
  live leaderboard is **kept**, a new live Record (new seed = old seed + 1 so replays stay exact per game).
  Refuses a map whose floor is smaller than the current number of bots. The current map is persisted in the Store
  (`map.txt` next to the board) and used at the next server start instead of the built-in room (the `-map` flag still
  wins when given).
- **Routes:** `POST /api/admin/map/preview {width, height, walls?, seed?}` -> `{seed, map}` (no seed = a random one from the
  adapter, so "regenerate" = call again); `POST /api/admin/map/apply {map}`. `GET /api/admin/status` adds the current
  map size.
- **Web:** on `/admin` a "Map" box: width/height inputs, "Generate" shows the preview drawn as a small grid (reuse the
  renderer's wall drawing or a plain canvas), "Regenerate", "Apply" (confirm: "restarts positions, keeps scores").
- **TV:** the TV and the WS must handle a world whose size changes between two frames (canvas resize, no stale cells).
- Tests first: generator tests (border, density range, connectivity over many seeds, determinism per seed, size
  limits), app tests (apply keeps board and players, fresh positions, refuses too-small floor, Replay of each game's
  Record matches its log, persisted map reloaded), httpapi tests, web test for the size change in the reducer/scene.

### Step 44 - Game timer, game over, last game board [DONE]
- **App:** `Arena.SetTimer(beats int)` (0 = no timer) counts down in beats (the admin enters minutes; the adapter
  converts with the beat length). When it reaches 0 at a beat boundary: the arena **stops** (no more beats played, Run
  keeps waiting for ticks), the live board is saved as the **last game** board (Store: `last.json`, replacing the
  previous one), and an Update with `GameOver: true` is broadcast. `Arena.StartGame()` (only while stopped) starts a
  fresh game: empty live board, a new match on the current map (as in step 43), same players and programs, timer
  off. While stopped, joins/edits/kicks are still accepted and apply when the next game starts (edits skip the swap
  rate limit, the latest wins); `ResetBoard` is refused with `ErrNotRunning` (409; the next start empties the board).
- **Protocol:** `Update` gains `Remaining` (beats left, -1 = no timer) and `GameOver`; `GET /api/leaderboard/last`
  returns the last game board (`[]` when none). `make gen`.
- **Routes:** `POST /api/admin/timer {minutes}` (0 cancels), `POST /api/admin/start`; status adds `running`,
  `remaining`.
- **Web:** TV shows the remaining time (mm:ss) while a timer runs and a "Game over" overlay with the final board when
  stopped; `/leaderboard` gets a "Last game" table under the live one; `/admin` gets a timer input + Cancel and a
  "Start new game" button (enabled only when stopped); "Reset leaderboard" is disabled when stopped.
- Stdin console gains `timer <minutes>` and `start`.
- Tests first: app tests (countdown to stop, no beats while stopped, last board persisted and replaces the previous,
  start gives an empty board and keeps players, commands queued while stopped apply on start), httpapi tests, web
  tests (countdown format, overlay state, last-game rows). Docs: PLAN.md section 2 (arena mode: games end on the timer)
  and section 4, VISION.md section 10 note that rounds are now admin-timed games.

## My bot screen (step 45)

Owner request: a clean "my bot" page with an obvious flow.

### Step 45 - My bot screen redesign [DONE]
- **Look:** Tailwind CSS (v4 via `@tailwindcss/vite`), clean and plain like the Tailwind UI examples: a centred card
  layout, clear headings, rounded inputs, primary/secondary buttons, readable monospace code box. Tailwind's
  preflight must not change the TV, leaderboard or admin pages (scope it to the editor or import only theme +
  utilities); other pages keep their look in this step.
- **Layout, two clearly separate sections:**
  1. **Edit my bot (manual):** the code box with its line gutter and error markers, and one **Save** button under
     it. The box shows the saved program on load. A status badge says "Saved" or "Unsaved changes". Save is
     **disabled** while the box equals the last saved program (and while saving). A successful save shows a clear
     success message ("Saved - your bot runs it from the next beat"); errors, rate limits and failures show as now.
  2. **Start from a preset:** the template picker + "Load into editor" (it only fills the box, the user still saves;
     confirm before replacing unsaved changes), and the **"Copy rulebook for AI"** button.
- **"Copy rulebook for AI"** replaces "Copy AI prompt" and the wish input is removed. The copied text, in order:
  (1) the agent's role (an expert BOTSCRIPT bot author helping a player), (2) the rules: LANGUAGE.md plus the VISION
  rule sections (as now), (3) the example bots, (4) the request: before writing any code, ask the player what the bot
  should be optimised for (kills, survival, coins, or a mix / a style), then write one program as a single code block
  within the spec limits.
- **Removed for now:** the bot's live stats panel (memory, trail, HP) from this page; keep its modules if other code
  or tests use them, otherwise delete them and their tests.
- Join, full and offline states keep working and get the same look.
- Tests first: controller tests (dirty/clean state, Save disabled when clean and while saving, success state after
  save, clean state updated after a successful save, template load keeps the saved state as the reference), prompt
  test (section order, the "ask what to optimise for" request at the end, no wish), layering tests stay green; check
  the page by hand in a browser (or Playwright) and report.

## Bot looks (steps 46-47, 49)

Owner request: players edit their bot's name and look: a free color (color picker), a body shape, eyes and a mouth
that express an emotion, plus a hat; a Randomize button; the face reacts by itself to what happens in the arena.
Looks are cosmetic: not part of the sim, the Record, the event log or any hash (like names).

**The look** (one value type, shared by Go and TS through the protocol):

| Part | Values |
|---|---|
| `color` | `#rrggbb` (any) |
| `body` | `circle`, `square`, `rounded`, `triangle`, `hexagon`, `diamond`, `ghost` |
| `eyes` | `dots`, `wide`, `sleepy`, `angry`, `x`, `hearts`, `visor`, `cyclops` |
| `mouth` | `smile`, `grin`, `flat`, `frown`, `o`, `teeth`, `tongue`, `zigzag` |
| `hat` | `none`, `antenna`, `crown`, `horns`, `headband`, `bow` |

### Step 46 - Looks and names on the server [DONE]
- `app`: a `Look` struct with validation (color regexp, each part from its list; unknown = error naming the field).
  Every bot has a look. A default look is derived from the bot id (a stable hash picks color from the current
  `PALETTE` in `web/src/views/tv/color.ts` - move the palette to Go as the source of truth - and each part), so house
  bots and old saved bots get varied looks with no randomness. Players keep their look across restarts (`SavedBot`
  gains `look`, missing = default).
- `Arena.SetProfile(token, name, look)`: renames (same rules as Join: trimmed, 1..24 runes; the leaderboard line
  follows) and sets the look; takes effect in the next published Update. Not rate-limited by the hot-swap limit, but
  at most one profile change per beat per bot (latest wins). House bots cannot be changed through it.
- Protocol: `Player` in the roster gains `look`; `GET /api/bot` returns `look`; `PUT /api/bot/profile {name, look}`
  -> 204, 400 with the field error, 401/403 as the other bot routes. `make gen`. Leaderboard entries keep `name`.
- TV: the bot color everywhere (body, x-ray cone, sidebar) comes from `look.color` instead of `botColor(id)`; nothing
  else changes visually in this step.
- Tests first: look validation, default look determinism and spread over ids, SetProfile (rename + board line,
  invalid name/look, house refused, persisted and restored, unknown token), route tests, protocol tests, web reducer
  uses the roster color; determinism hashes unchanged (looks are not in the log).

### Step 47 - Drawing looks and auto reactions [DONE]
- `render`: draw a bot from its look: body shape filled with the color (with a darker outline for contrast), eyes,
  mouth and hat drawn on top in a contrasting ink (dark on light colors, light on dark ones). Shapes must read at the
  TV cell size; keep each part a small pure path function that can be unit-tested (path commands / bounding box) plus
  one canvas smoke test.
- **Auto reactions** (render-side only, from beat events already in the protocol; the chosen face is the resting
  mood): hit -> wince (`x`-squeeze eyes + `o` mouth) for one beat; got a kill -> `grin` + `wide` eyes for two beats;
  dead (fallen, until respawn) -> `x` eyes + `flat` mouth; respawn shield -> back to the resting face. Pure function
  `faceOf(look, reactions, beat)` with tests; precedence dead > hit > kill.
- Existing effects (glide, hit flash, death fade, coin glow, HP bar) keep working with the new shapes; ghost/triangle
  bodies stay inside the cell.
- Tests first: path helpers, contrast ink choice, `faceOf` precedence and timing, tween/scene tests updated. Hand
  check on a fixture with several looks; screenshot.

### Step 49 - Customize on the my bot page [DONE]
- A third card on `/bot`, "3. Look and name", same Tailwind style as step 45: name input; native `<input
  type="color">` color picker; body, eyes, mouth, hat as rows of small clickable tiles each drawn with the step 47
  renderer helpers; a large **live preview** that also cycles the auto reactions on hover/tap; **Randomize** (rolls
  every part and a color, client-side `Math.random` is fine here, it is not the sim); **Save look** disabled when
  unchanged, success/error messages as in step 45.
- Uses `PUT /api/bot/profile`; after saving, the TV shows the new look at the next update.
- Tests first: the look-editor controller (dirty/clean, randomize changes something and stays valid, save success /
  400 field error / rate), tile list built from the protocol value lists; e2e: change the name and color, save, the
  TV roster shows the new name.

## Program check for AI (step 48)

Owner request: one endpoint an AI assistant can call by itself to check a program, and the rulebook tells it the
right URL, taken from the server's configured base URL.

### Step 48 - `/api/check` and the base URL in the rulebook [DONE]
- **Endpoint (public, no token):** `POST /api/check` with the program as the raw body (`text/plain`), and the same as
  `GET /api/check?src=<url-encoded program>` (many AI chat tools can only fetch URLs with GET). Body/param limit 16 KB
  (413 / 400 above). Answer 200 JSON:
  - `{ok: false, errors: [{line, col, message}]}` - the same validation errors and hints the editor shows;
  - `{ok: true, trial: {...}}` - the program is valid; it also plays a short **trial**: 100 beats in a small fixed
    9x7 room against 3 moving house bots (hound, forager, tracker) so it meets enemies, fixed seed, headless (app layer, no effect on the live arena), and reports what the
    bot did: beats played, actions by kind (scan, move, fire, wait), idle beats (budget overrun / missed beats),
    clicks, hits taken, kills, deaths, coins, and `warnings` in plain words for obvious problems (e.g. "never fired",
    "never moved", "missed 12 beats: code is too slow", "no action in loop() on 40 beats - each became a wait").
  - Same program -> same answer (deterministic). Plain-text-friendly: the JSON is small and flat.
  - Rate limit per remote IP (e.g. 30 per minute, then 429) since the trial costs CPU; trials run outside the arena
    lock and are capped in wall time by the op budget (no way to hang the server).
- **Base URL:** env `ARENA_BASE_URL` (e.g. `https://arena.example.com`; trailing slash trimmed; never required).
  `GET /api/config` -> `{baseUrl}` (`""` when unset). Docker `docker-run` passes it through; README documents it.
- **Rulebook:** "Copy rulebook for AI" gets a section "Check your program" placed before the final request: the
  exact URLs with the base URL filled in (`ARENA_BASE_URL` if set, else the page's `location.origin`), how to call
  both forms, what the answer means, and the instruction to check every program it writes, fix all errors and act on
  the warnings before giving the final code to the player. If the URL is a private address (localhost, 10.x,
  192.168.x, 172.16-31.x) the section says the AI may not reach it and should still follow the rules by hand.
- Tests first: app trial (deterministic summary, warnings, invalid program, no effect on a live arena), httpapi
  (POST and GET, size limits, 429, JSON shape, config route), cmd env wiring, web prompt test (section present, URL
  from config, fallback to origin, private-address note), e2e: call `/api/check` with a broken and a good program.

## Leave and return (step 50)

Owner request: a player can take their bot out of the arena and bring it back later with everything kept (name,
look, program, token, leaderboard line). Every stay in the arena is limited to 24 hours; then the bot leaves by
itself the same way and can return at once.

### Step 50 - Leave the arena, return to it, 24-hour stay [DONE]
- **States of a human bot:** *in the arena* (in the match or queued to join) or *detached* (not on the map, not in
  the roster/TV, not counted by `-max-players`, but its token, name, look, program and leaderboard line stay). House
  bots never detach. The admin `kick` is unchanged (deletes the bot and its board line, token -> 401) and also works
  on detached bots.
- **App:** `Arena.Leave(token)` now detaches instead of deleting (token stays valid; it lands at the next beat as a
  normal recorded leave; leaving a detached bot is a no-op). New `Arena.Return(token)`: queues the bot to join at the
  next beat like a fresh join (new spawn, fresh HP/memory, its saved program and look); `ErrFull` when the player
  cap or the floor is reached; no-op when already in the arena. Leave/return of one bot is limited to one change per
  `SwapEvery` beats (`RateLimitError` with `RetryBeats`, like a hot-swap; not limited while the game is stopped -
  then they apply at Start like joins do). `PutBot` and `SetProfile` work while detached (stored, used on return; no
  hot-swap queued). `GetBot` reports `InArena`, `LeftBy` (`""`, `"player"`, `"time"`) and `StayLeft` (beats until the
  automatic leave; -1 when detached or no limit).
- **Stay limit:** `Config.MaxStay` in beats (0 = none). Each beat a human bot spends in the match counts; when its
  count reaches `MaxStay` it detaches at that beat boundary (`LeftBy = "time"`). Returning starts a new count. Beats
  are the clock (the arena never reads wall time), so a stopped game does not use up stays. `cmd/arena` flag
  `-max-stay` (Go duration, default `24h`, `0` = no limit) is converted to beats with the beat length (rounded up).
- **Persistence:** `SavedBot` gains `detached`, `leftBy` and `stayBeats` (omitted when zero/false). On start,
  detached bots are restored without joining; the others rejoin as now and keep their count. The count is saved at
  least every 60 beats and at shutdown (`Flush`), so a restart loses at most a minute of it.
- **Routes:** `POST /api/bot/leave` and `POST /api/bot/return` (Bearer; 204; 409 "arena is full" on return; 429 with
  `retryBeats`; 401/403 as the other bot routes; 405 for other methods). `GET /api/bot` adds
  `arena: {in, leftBy, stayLeftBeats}` and `beatMs`. The admin status lists detached human bots with `detached: true`
  (Kick button still shown); the console `list` marks them.
- **Web `/bot`:** a status card at the top of the page, same Tailwind style as steps 45/49:
  - in the arena: "Your bot is in the arena", time left of the stay ("leaves automatically in 23 h 59 min", from
    `stayLeftBeats * beatMs`, hidden with no limit) and a secondary **Leave the arena** button (confirm first);
  - detached: "Your bot is not in the arena" (+ "it left automatically after 24 hours" when `leftBy` is `time`) and a
    primary **Return to Arena** button; a full arena shows the 409 message, 429 shows the wait;
  - the editor and look cards keep working in both states; the state refreshes after each action and on load.
- Docs: PLAN section 4, README (flags, routes), `make run` comment if flags change.
- Tests first: app (leave keeps token/look/program/board line, roster and cap exclude detached bots, return joins at
  the next beat with the saved program, full -> `ErrFull`, rate limit, stopped game, auto-leave exactly at
  `MaxStay`, new count after return, house bots unaffected, kick of a detached bot, persistence round trip incl.
  restart), replay/determinism suite stays green (leaves/joins recorded), httpapi (routes, statuses, JSON shape),
  cmd (flag parsing and beat conversion), web controller (both states, buttons, messages, time-left text), e2e: leave
  then return on `/bot` and the TV roster follows.

## Reboot on program change (step 51)

Owner request: forbid live puppeteering (an AI watching the game and pushing a new program every second to steer the
bot by hand). A saved program no longer lands at the next beat: the bot **reboots** - it leaves the map for 10
seconds and comes back running the new program. The TV shows a matrix-rain "rebooting" effect with a progress bar.
("Reboot" in code and UI, never "reload": reload is the gun.)

### Step 51 - Reboot on program change [DONE]
- **Rule (update VISION section 7 "Editing is allowed any time" and LANGUAGE section 11 / wherever hot-swap is
  described):** saving a new program while the bot is in a running game starts a reboot at the next beat boundary:
  the bot leaves the map (its cell is free, it cannot be hit or seen by scans, it does not act; bullets it fired
  keep flying) for `RebootBeats` beats, then it respawns by the respawn-spot rule (section 5) running the new program
  from a fresh `loop()` (as a hot-swap does today). It keeps HP, gun reload/bonus state, `num`/`pin` memory, dice and
  score; the trail is cleared. Saves during a reboot replace the pending program (the latest wins) but **do not
  restart the countdown**. The hot-swap rate limit (`SwapEvery`) no longer applies to program saves (the reboot is
  the limit); leave/return keep their own limit (step 50), and with `RebootBeats > 0` a bot that left by the player's own
  Leave may `Return` no earlier than `RebootBeats` beats after its leave landed (`RateLimitError` with the right
  `RetryBeats`, HTTP 429), so a program saved while detached cannot skip the reboot; automatic leaves (`LeftBy = "time"`)
  and a leave that has not landed yet are exempt, and the /bot status card shows the wait like any rate limit.
- **Edge cases:** a bot that is dead when its reboot starts comes back at the later of its respawn and the reboot end;
  a detached bot just stores the program (no reboot, step 50); while the game is stopped the program is stored and
  applies at Start without a reboot (as edits do today); a leave or kick during a reboot cancels it (the Return wait above applies after a leave); `ApplyMap`/Start
  of a new game cancels a running reboot (the bot joins the new game with its latest program); the stay count (step
  50) keeps running during a reboot; house bots never reboot.
- **Config:** `Config.RebootBeats` (0 = old behaviour: hot-swap at the next beat with the `SwapEvery` limit).
  `cmd/arena` flag `-reboot` (Go duration, default `10s`, `0` = off) converted to beats with the beat length,
  rounded up (reuse the `-max-stay` conversion).
- **Determinism and replay:** reboots are recorded in the Record (start beat, bot, program) so replays and hashes stay
  exact; if the Record format changes, bump `RecordVersion` and keep older records decodable (upgrade path as done
  before). Add scenario/replay tests: a reboot frees the cell, the bot is not hit/scanned/acting meanwhile, comes back
  with kept HP/memory and the new program, latest save during a reboot wins with the original countdown, dead-bot
  case. Update `testdata/scenarios/INDEX.md` for the new rule.
- **Wire:** each WS update lists rebooting bots: `rebooting: [{bot, cell, hp, beatsLeft, total}]` (`hp` = what it comes back with) (`cell` = where it left,
  for the effect). `GET /api/bot` adds `rebootLeftBeats` (-1 = not rebooting; 0 = countdown over but still off the map, no free cell) and `rebootTotalBeats` (the full length, 0 = reboot off). `PUT /api/bot` stays 204.
- **TV:** at the cell where a rebooting bot left, draw a matrix-rain effect (falling green glyphs inside the cell, in
  the bot's color tint) with a thin progress bar under it filling up as `beatsLeft` goes down, and a "rebooting"
  label in the roster/sidebar row. The effect stays in its cell even if another bot walks in (drawn under bots).
  Cheap to draw (canvas, no per-frame allocations beyond a small glyph buffer). The pencil badge shows when the bot
  comes back (set on the beat it is really back, however late).
- **Web `/bot`:** after a save while in a running game the success message says the bot is rebooting and returns in
  N s; the status card (step 50) shows "Rebooting..." with a matrix-style progress bar and the seconds left (from
  `rebootLeftBeats * beatMs`), refreshing until it is back.
- **Rulebook for AI:** the copied rulebook says plainly that every saved program takes the bot off the map for
  the reboot time, so the bot must be steered by its algorithm, not by live edits (comes from the updated
  VISION/LANGUAGE text via `make gen`).
- Docs: PLAN section 4 (`PUT /api/bot` behaviour), README (flag), VISION/LANGUAGE as above.
- Tests first: sim/app (all rule and edge-case bullets above), record round trip and old-record decoding,
  determinism suite, httpapi (`rebootLeftBeats`, PUT no longer 429 for saves), cmd flag, web store/TV (rebooting
  list parsed, effect drawn at the cell with the bar fraction, roster label), `/bot` controller (message and status
  card), e2e: save a program on `/bot`, the TV shows the rebooting effect, the bot comes back.
