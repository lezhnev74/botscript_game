# BOTSCRIPT - the bot language

**A tiny JavaScript.** A bot is a JS program written mostly by an AI and pasted on the phone. Thinking is free; only
actions (`scan`, `move`, `fire`, `wait`) block until the next beat. Memory: **8 number cells** `num[0..7]`, **8 place cells**
`pin[0..7]` and a read-only **trail** of the last 3 beats.

Supersedes SCANLOOP (`archive/LANGUAGE-v0-scanloop.md`). Which rules are agreed vs proposed: see `VISION.md`.

---

## 1. World rules the language relies on

- **Grid with walls.** 4 directions. **No facing**: every action names its own direction.
- **Beat = 1 action.** All bots decide on the same snapshot.
- **Moving into a wall is a no-op** (beat spent, `ok = false` in the trail, no event).
- **Moving into a bot fails** and triggers `onBump` for the mover only.
- **Beat order:** moves -> new shots -> bullets travel -> scans -> hits -> coins. A bot that steps away on the beat you
  fire wins. A scan sees the world after this beat's flight.
- **Bullets** fly 3 cells per beat until a bot or a wall. **Reload** 3 beats. **Coins** give a bonus shot (max 2).
  `fire` with nothing loaded = a wasted beat ("click").
- **Gunfire is audible.** A real shot (loaded or bonus; a click is silent) is heard by every other bot within 6 cells
  (Manhattan, walls do not block). A bot learns only the cell of the nearest shooter, see `heard` in section 5.
- **No coin radar.** A bot finds coins with `scan` (the `coin` reading) and its own guessing.
- **HP 3.** Free-for-all.

## 2. Program shape

```js
function loop() { ... }          // required: the normal routine, called again each time it returns
function onHit(from) { ... }     // optional interrupt: a bullet hit me, `from` = where it came from
function onBump(from) { ... }    // optional interrupt: I walked into a bot, `from` = that direction
function anyHelper(d) { ... }    // optional helpers
```

## 3. Execution

- **Each bot runs in its own goroutine.** Thinking is free; an action call queues the action and blocks the script.
- **The orchestrator owns time.** At each tick it takes the queued action of every bot, resolves the beat on one
  snapshot, writes the results back into each bot's state, and unblocks the scripts. Actions only take effect at ticks.
- **Slow code misses beats.** "Slow" is measured in **ops, not wall clock** (budget 100000 ops per beat, high enough for real search algorithms), so replays
  are exact. A bot that hasn't queued an action within its budget misses the tick (stands still, trail act `IDLE`)
  and its code continues. An endless loop = a frozen bot that misses beats.
- **No free spinning:** if `loop()` returns without an action, that beat is a `wait`.
- **Negative direction = wait.** `move`, `scan` or `fire` with a direction below 0 (such as `path(p)` = -1) spends the beat as
  `wait()` (trail act `WAIT`, dir -1). Directions 0 and up wrap modulo 4: `move(5)` is `move(E)`.
- **Interrupts:** a hit or bump this beat aborts the current call stack; the next beat runs the handler. When the
  handler returns, `loop()` starts fresh. Hit beats bump. A new hit during `onHit` restarts it; a bump during `onHit`
  is ignored. No handler = no reaction.
- **Reboot (saving a new program):** every program saved while your bot is in a running game takes it **off the map for
  10 seconds** (10 beats by default; the arena may set another time). The bot leaves its cell (free for others), cannot
  be hit or scanned and does not act; bullets it already fired keep flying. Then it comes back on the respawn spot
  (the free cell farthest from the other bots) running the new code from a fresh `loop()`, with a short shield.
  `num` / `pin` memory, dice, HP, gun reload and bonus shots, and score are kept; the trail is cleared. A save during a
  reboot replaces the waiting program but does not restart the countdown. A bot that is dead when you save comes back
  at the later of its respawn and the end of the reboot. You cannot steer a bot by live edits: it must be steered by its
  algorithm. Leaving the arena and returning does not skip the reboot: a bot that left by its own Leave can return
  no earlier than the reboot time after it left (a bot that was sent out when its stay ran out can return at once).
  If the arena has the reboot switched off, a save is a hot-swap at the next beat (1 swap per 3 beats). Pencil
  badge on TV when the bot is back.

## 4. Memory

- **`num[0]` .. `num[7]`** are the only writable variables, and the only way to remember a scan. Integers, start at 0,
  clamped to +-9999, survive interrupts, hot-swaps and death. The index may be any expression (`num[i]`, `num[num[0]]`); a
  constant index outside 0..7 is a validation error, a computed one reads 0 and writes nothing. The old slots `a b c`
  were removed: use `num[0]`, `num[1]`, `num[2]`.
- **`pin[0]` .. `pin[7]`** hold **places** (a cell of the map, or `none`). They start `none` and survive interrupts,
  hot-swaps and death like `num`. A place is a second value type, not a number:
  - values: `none`, `here()` (the cell I stand on), `at(x, y)` (any coordinates, stored as given; off-map places are
    just unreachable), `pin[i]` and `heard` (read-only, see section 5: the last sound heard);
  - `pin[i] = place` is the only way to store one (`=` only: no `+=`, `++`), and `pin[i] = 5` is an error;
  - compare with `==` / `!=` (place with place, cell by cell; `pin[i] == none` asks if a pin is empty);
  - read a coordinate with `pin[i].x` / `pin[i].y` (`none.x` and `none.y` are -1), also on `here()` and `at(x, y)`;
  - `free()` = the first empty pin index (-1 when all 8 are set), `count()` = how many pins are set;
  - `path(p)`, `steps(p)` take a place and `nearest()` scans the pins: walking questions, see section 7;
  - a place has no arithmetic and cannot be a condition, a return value or an argument of your own functions (pass the
    index and read `pin[i]` inside); each of these is a validation error with a hint. Constant index outside 0..7 is a
    validation error; a computed one reads `none` and writes nothing.
- No `let`/`const`/`var`, no user arrays/objects/strings, no closures.
- Function parameters are allowed as temporaries. Call depth max 3, no recursion (no hidden memory on the stack).

## 5. Reading the world (read-only)

| Name | Meaning |
|---|---|
| `enemy`, `coin`, `bullet` | 0-3 from the last scan (see 6), overwritten by the next scan |
| `last` | direction of the last scan |
| `hp`, `loaded`, `bonus` | health 1-4 (a coin heals 1), gun ready (reloaded or bonus stored), bonus shots 0-2 |
| `x`, `y` | my exact position; `0,0` = north-west corner |
| `WIDTH`, `HEIGHT` | arena size |
| `wall(x, y)` | free terrain check: true for a wall cell or any cell outside the arena, false for floor. Bots, bullets and coins are never revealed (scan or bump for those) |
| `heard` | a place (read-only, see section 4): the cell of the nearest bot that fired a real shot within 6 cells on the latest beat it heard anything (the source's position after moves; ties go to the lowest bot id; my own shots and clicks are silent). It stays until a newer sound is heard, `none` before any, reset on respawn. A new sound is `heard != pin[k]` (a repeat sound from the same cell looks the same, since `heard` carries no time); walk to it with `path(heard)`; read it with `heard.x` / `heard.y`; it cannot be assigned |
| `none`, `pin[i]` | places, see section 4 (`here()` and `at(x, y)` are in section 7) |
| `N E S W` | directions are the numbers 0 1 2 3, so they fit in `num` and support arithmetic |
| `trail[i]` | what happened `i` beats ago, `trail[0]` = the latest beat, up to 3 entries (`trail.length`) |

**Trail entry fields:**

| Field | Meaning |
|---|---|
| `x`, `y` | my position after that beat |
| `act` | `SCAN` 0, `MOVE` 1, `FIRE` 2, `WAIT` 3, `IDLE` 4 (missed beat) |
| `dir` | the action's direction (-1 for `WAIT`/`IDLE`) |
| `ok` | move: I actually moved; fire: a shot left the gun; scan/wait: true |
| `hit`, `bump` | -1, or the direction I was hit / bumped from on that beat |

The trail does **not** store scan readings: to compare scans, the script must keep them in `num` (or, for a position, in `pin`).
It gives path history, stuck detection and revenge without spending memory.

## 6. The scan

`scan(d)` looks at the **cone** opening in direction `d`, from the bot to the walls. Walls block the view: a cell
behind a wall (no straight line between cell centres) is not seen; grazing a wall corner still sees. Scans pass through
bots, coins and bullets and are silent. Each reading is a **level 0-3 = how lined up** the best-aligned object of that
kind is: 3 = on my line, 2 = one lane off, 1 = elsewhere in the cone, 0 = nothing. `bullet` counts only bullets flying
toward me that are still in the air after this beat's flight (a bullet that hit a wall or a bot this beat is gone, a bullet
fired this beat is counted where it stopped), so a sidestep next beat always dodges it. No side, no distance.

## 7. Built-ins

```js
// actions: block until the next beat
scan(d)  move(d)  fire(d)  wait()

// free
left(d) right(d) back(d)      // turning
pick(N, N, S)                 // weighted random direction, skips walls
chance(30)                    // true 30% of the time
rand(lo, hi)                  // random integer, inclusive
wall(x, y)                    // true = wall or outside the arena, false = floor (terrain only)
abs(x) min(x, y) max(x, y) dist(x1, y1, x2, y2)   // dist = Manhattan

// places (see section 4)
here()                        // the cell I stand on
at(x, y)                      // the cell (x, y), stored as given
free()                        // first empty pin index, -1 if all 8 are set
count()                       // number of pins that are set

// navigation (walking over the terrain: walls block, bots and coins do not)
path(p)                       // direction of the first step toward place p on a shortest walk, -1 if p is none,
                              // a wall, off the map, unreachable or my own cell
steps(p)                      // walking distance to p (0 on my own cell), -1 if unreachable or none
nearest()                     // index of the set pin with the fewest steps, -1 if none is reachable
```

- `pick` skips walls. If no listed direction is open, it returns the first one (the move is then a wall no-op).
- `path`, `steps` and `nearest` are free (no beat) and cost ops like any call, not per cell walked. `path` ties go in
  the order N, E, S, W; `nearest` ties go to the lowest pin index. `move(path(p))` is safe: -1 is a negative direction, which is a wait (section 3).
- Dice are seeded per bot: replays are deterministic.

## 8. Allowed syntax

`if/else`, `while`, `for`, `break`, `continue`, `return`, `? :`, `&& || !`, `+ - * / %` (integer division),
`== != < <= > >=`, assignment `= += -= ++ --` (`num[i]`; `pin[i]` takes `=` only), member reads on `trail[i]` and `.x` / `.y` of a place, `function` declarations,
`//` comments. Anything else is a parse error with a hint. Limit: 120 lines.

## 9. Bots

Each bot has a one-line strategy recipe: **hunter** (find, aim, fire), **forager** (scan for coins, walk, wander), **mode switch** (change role on `hp` / `loaded`).

**Rookie (the template):**
Recipe: hunter - scan at random, shoot what is on your line.

```js
function loop() {
  scan(pick(N, E, S, W));
  if (enemy == 3) fire(last);
}
```

**Lighthouse (sweep, dodge with `wall()`, revenge in memory):**
Recipe: hunter with a mode switch - sweep and dodge, hunt the shooter for a few passes after a hit.

```js
// num[0] = sweep direction, num[1] = passes left to hunt the shooter (-1 = sidestep pending), num[2] = shooter direction
function loop() {
  if (num[1] < 0) { num[1] = 0; move(pick(left(num[0]), right(num[0]))); return; }   // retreated last beat: sidestep now
  if (num[1] > 0) { num[1]--; look(num[2]); return; }
  num[0] = right(num[0]);
  look(num[0]);
}
function look(d) {
  scan(d);
  if (bullet == 3) dodge(d);
  else if (enemy == 3 && loaded) fire(d);
  else if (enemy == 2) move(pick(left(d), right(d)));
  else if (coin == 3) move(d);
}
// sidestep off the line d; both sides walls (a corridor): back off along it, sidestep next beat
function dodge(d) {
  if (d % 2 == 0 ? wall(x - 1, y) && wall(x + 1, y) : wall(x, y - 1) && wall(x, y + 1)) {
    num[0] = d; num[1] = -1;
    move(back(d));
  } else move(pick(left(d), right(d)));
}
function onHit(from) {
  num[1] = 0;                                 // drop a stale pending sidestep
  dodge(from);                           // off the line first
  num[2] = from;
  if (num[1] >= 0) num[1] = 4;
}
function onBump(from) {
  if (loaded && chance(60)) fire(from); else move(back(from));
}
```

**Tracker (`wall()` instead of bumping, `num` for scan memory: side guessing with feedback):**
Recipe: hunter - chase the enemy you scan and guess which side it dodges to.

```js
// num[0] = direction I'm tracking, num[1] = sidestep I tried, num[2] = 1 if I just sidestepped from an enemy at 2, 2 = retreated, sidestep next
function loop() {
  if (num[2] == 2) { num[2] = 0; move(pick(left(num[0]), right(num[0]))); return; }   // retreated last beat: sidestep now
  if (blocked(num[0])) num[0] = pick(left(num[0]), right(num[0]));   // wall ahead: turn instead of bumping
  scan(num[0]);
  if (bullet == 3) { dodge(num[0]); return; }
  if (enemy == 3 && loaded) { num[2] = 0; fire(num[0]); return; }
  if (enemy == 2) { num[2] = 1; num[1] = pick(left(num[0]), right(num[0])); move(num[1]); return; }
  if (num[2] == 1 && enemy == 1) {        // it was 2, I stepped, now 1: wrong side, cross over
    num[2] = 0; move(back(num[1])); move(back(num[1]));
    return;
  }
  num[2] = 0;
  if (enemy == 0) num[0] = pick(N, E, S, W); else move(num[0]);
}
// sidestep off the line d; both sides walls (a corridor): back off along it, sidestep next beat
function dodge(d) {
  if (d % 2 == 0 ? wall(x - 1, y) && wall(x + 1, y) : wall(x, y - 1) && wall(x, y + 1)) {
    num[2] = 2;
    move(back(d));
  } else { num[2] = 0; move(pick(left(d), right(d))); }
}
function blocked(d) {
  return wall(x + (d == 1 ? 1 : d == 3 ? -1 : 0), y + (d == 2 ? 1 : d == 0 ? -1 : 0));
}
```

**Forager (scans for coins, wanders to random pinned spots; runs when weak, fights when healthy and loaded):**
Recipe: forager with a mode switch - scan for a coin and head its way, else wander to a pinned spot with `path`; run when weak (`hp`), fight when healthy and `loaded`.

```js
// Forager: scans for coins and walks toward the one it saw, wanders to random spots when it sees none, runs when an enemy is close.
// num[0] = direction of the next look, num[1] = beat counter, num[2] = 1 + the way to a coin I saw (0 = none),
// pin[0] = the spot I am wandering to
function loop() {
  num[1]++;
  if (num[1] % 2 == 0) { look(); return; }       // every other beat: look around
  if (num[2] > 0) { move(num[2] - 1); return; }  // a coin was in that direction: go for it
  if (pin[0] == none || path(pin[0]) < 0) {      // no spot in mind (or reached it): pick one
    pin[0] = at(rand(1, WIDTH - 2), rand(1, HEIGHT - 2));
  }
  move(path(pin[0]));                            // no way there (-1) is a wait
}
function look() {
  if (num[2] > 0) num[0] = num[2] - 1;           // chasing a coin: look its way again
  else num[0] = right(num[0]);
  scan(num[0]);
  num[2] = coin > 0 ? num[0] + 1 : 0;            // still there? keep going, otherwise forget it
  if (enemy == 3 && loaded) fire(num[0]);        // a clear shot: take it
  else if (enemy >= 2 && !(loaded && hp >= 3)) { num[2] = 0; move(pick(left(last), right(last), back(last))); }   // too close and weak: run
}
```

**Hound (follows `heard`, looks before each step, pins the last sighting):**
Recipe: hunter by ear - follow `heard` (a place), look before each step, fire on 3, pin the last sighting.

```js
// Hound: follows the sound of gunfire, looks before it steps, fires at a clear line.
// num[0] = direction of the next look, num[1] = looks done since the last sound I followed,
// num[2] = 1 when the next beat is a look toward the sound, num[3] = beats left to follow it;
// pin[0] = where I last saw an enemy, pin[1] = the sound I am following
function loop() {
  if (heard != pin[1]) { pin[1] = heard; num[3] = 3; }   // a new sound: follow it for a few beats (a repeat from the same cell looks the same)
  if (num[3] > 0 && path(pin[1]) >= 0) {      // creep toward it, a look between steps
    num[3]--;
    num[1] = 0;
    num[2] = 1 - num[2];
    if (num[2] == 1) { aim(path(pin[1])); return; }
    move(path(pin[1]));
    return;
  }
  if (num[1] < 4) { num[1]++; num[0] = right(num[0]); aim(num[0]); return; }   // arrived: look all around
  if (pin[0] != none && path(pin[0]) >= 0) { move(path(pin[0])); return; }     // back to the last sighting
  pin[0] = none;
  move(pick(N, E, S, W));                     // nothing heard: wander
}
function aim(d) {
  scan(d);
  if (enemy == 3) {
    pin[0] = here();                          // remember where I saw one
    if (loaded) fire(d);
  }
}
```

**Walking to a pin (a few lines, not a full bot):**
```js
function loop() {
  if (pin[0] == none) pin[0] = at(WIDTH - 2, 1);   // a far cell, stored once
  move(path(pin[0]));                              // walks round walls; -1 (arrived or cut off) = wait
}
```

## 10. Phone

No tap editor. The phone has: a template gallery, **"Copy rulebook for AI"** (exports this spec, the world rules and the
example bots), and a paste box with red-line errors and fix hints. No live view of the running line, `num` / `pin` cells or
trail on the my-bot page for now.

## 11. TV and memory

The TV shows no memory badge, only a pencil badge when a bot is back from a reboot (and a matrix-rain "rebooting" effect with a progress bar where it left). The current line and the `num` / `pin` cells
are not shown anywhere for now (no live view on the my-bot page). The TV draws no pin markers on the map.

## 12. Implementation (Go)

- **Parser:** `github.com/dop251/goja/parser` (an ES5.1+ parser with a full AST, pure Go). A validator walks the AST and
  rejects anything outside section 8 (node types, names, `num[i]` / `pin[i]`-only assignment, a small number-vs-place type check, no recursion via a call-graph check).
- **Interpreter:** our own small tree-walker over that AST (~600 lines of Go), not the full goja VM. Reasons: it counts
  ops (deterministic budget), clamps integers, and needs no sandbox (nothing exists that isn't whitelisted).
- **One goroutine per bot.** An action builtin sends `{kind, dir}` on the bot's `actions` channel and blocks on its
  `results` channel. The orchestrator ticks on a timer:
  1. collect one queued action per bot (none queued or op budget spent = `IDLE`);
  2. resolve the beat on one snapshot, with a seeded RNG;
  3. send each bot its result (readings, `x, y`, hp, new trail entry, or an interrupt).
- **Ops are the clock for code.** The interpreter pauses the goroutine when its op budget for the tick runs out and
  resumes it on the next tick, so wall-clock speed and goroutine scheduling never change the game: replay = seed +
  programs + edit log.
- **Interrupts:** the blocked action returns an interrupt result; the interpreter unwinds the stack (sentinel panic)
  and calls `onHit`/`onBump`. Hot-swap is the same unwind with a new AST.
- **No shared memory:** each goroutine owns its bot state; the orchestrator only talks through channels.

## 13. Open

1. Trail length 3 (try first, may grow) and op budget 100000: tune after simulations.
