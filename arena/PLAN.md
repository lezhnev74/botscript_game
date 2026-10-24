# BOT ARENA - Architecture and development rules

Source of truth for rules: `../VISION.md`, `../LANGUAGE.md`. This file covers how they are built.
Everything lives in `arena/`. Each change = TDD (red -> green -> refactor), and ends with `make check` passing.

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
  a minimum count. Rounds/edit windows from VISION are **[later]**. A game can end on an admin timer:
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
- `GET/PUT /api/bot` (own bot only, token checked in app layer; GET also returns the bot's `look` a PUT in a running
  game reboots the bot: no 429 for saves, `GET` adds `rebootLeftBeats` / `rebootTotalBeats`, `-reboot` flag, default 10 s; with the reboot on, a return after the
  player's own Leave answers 429 until `RebootBeats` beats have passed since the leave landed),
  `PUT /api/bot/profile {name, look}` -> 204 (`Arena.SetProfile`: rename with the Join rules + look, no hot-swap rate limit,
  the latest call per beat wins; 400 with the bad field for a bad name or look, 401/403 as the other bot routes;
  looks are cosmetic and never enter the Record or any hash; the roster, `bots[]` and `fallen[]` of WS frames carry `look`),
  `POST /api/bot/leave` / `POST /api/bot/return` (detach the bot from the arena and bring it back, token/name/look/program/board line
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

## 5. Defaults for gaps in the spec

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
| Memory on screens after `a b c` removal | the TV has no memory badge; all 8 nums and pins on the phone live view; the TV draws no pin markers |

---

## Steps

New work is added here as `### Step N - title` (scope, rules touched, tests first), implemented per `CLAUDE.md`, and
removed once committed. Completed steps live in git history.
