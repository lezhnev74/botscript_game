# BOTSCRIPT - the bot language

**A tiny JavaScript.** Thinking is free; only actions (`scan`, `move`, `fire`, `wait`) block until the next beat.

---

## 1. World rules the language relies on

- **Grid with walls**, 4 directions, **no facing**: every action names its own direction. **HP 4**.
- **Beat = 1 action**, all bots decide on the same snapshot. Order: moves -> new shots -> bullets travel -> scans ->
  hits -> coins. A bot that steps away on the beat you fire wins; a scan sees the world after this beat's flight.
- **Moving into a wall** is a no-op (beat spent, `ok = false`). **Moving into a bot** fails and triggers `onBump` for the mover only.
- **Bullets** fly 3 cells per beat until a bot or a wall. **Reload** 3 beats; `fire` unloaded = a wasted "click".
- **Coins** give a bonus shot (max 2, lost on death) and heal 1 HP. One spawns every 50 beats while fewer than 5 are on
  the map.
- **Gunfire is audible:** a real shot (not a click) is heard by other bots within 6 cells (Manhattan, walls do not
  block); a bot learns only the cell of the nearest shooter (`heard`, section 5).
- **Score** (free-for-all, highest wins): kill +3, or +1 for a repeat kill of the same victim, coin +1, death -1 (never below 0).
- **Death:** respawn after 5 beats on the free cell farthest from the other bots, with a 2-beat shield.

## 2. Program shape

```js
function loop() { ... }          // required: called again each time it returns
function onHit(from) { ... }     // optional: a bullet hit me, `from` = where it came from
function onBump(from) { ... }    // optional: I walked into a bot, `from` = that direction
function anyHelper(d) { ... }    // optional helpers
```

## 3. Execution

- **Slow code misses beats:** the budget is 100000 ops per beat; a bot that has not queued an action within it stands
  still that beat (trail act `IDLE`); its code continues next beat (not restarted), so an endless loop freezes the bot.
- **No action = wait:** if `loop()` returns without an action, that beat is a `wait`.
- **Negative direction = wait:** `move`, `scan` or `fire` with a direction below 0 (such as `path(p)` = -1) is a
  `wait()`. Directions wrap modulo 4: `move(5)` is `move(E)`.
- **Interrupts:** a hit or bump aborts the current call stack; the next beat runs the handler, then `loop()` starts
  fresh. A hit beats a bump on the same beat; a new hit during `onHit` restarts it; a bump during `onHit` is ignored. No handler = no reaction.
- **Saving a program** takes the bot off the map for ~10 s; it returns with the new code, keeping memory, HP, score
  and gun. You cannot steer by live edits.

## 4. Memory

- **`num[0]` .. `num[7]`** are the only writable variables and the only way to remember a scan. Integers from 0,
  clamped to +-9999, kept through interrupts, saves and death. The index may be an expression (`num[num[0]]`); a constant
  index outside 0..7 is a validation error, a computed one reads 0 / `none` and writes nothing.
- **`pin[0]` .. `pin[7]`** hold **places** (a cell of the map, or `none`), start `none`, survive like `num`. A place is
  a second value type, not a number:
  - values: `none`, `here()`, `at(x, y)` (any coordinates; off-map places are unreachable), `pin[i]` and `heard`
    (read-only, section 5); `free() count() path steps nearest` work on places (section 7);
  - store with `pin[i] = place` (`=` only; `pin[i] = 5` is an error); compare with `==` / `!=` (`pin[i] == none` =
    empty); read `.x` / `.y` (-1 on `none`);
  - a place has no arithmetic and cannot be a condition, a return value or an argument of your own functions (pass the
    index, read `pin[i]` inside).
- No `let`/`const`/`var`, user arrays/objects/strings or closures. Parameters are temporaries. Call depth max 3, no recursion.

## 5. Reading the world (read-only)

| Name | Meaning |
|---|---|
| `enemy`, `coin`, `bullet` | 0-3 from the last scan (section 6), overwritten by the next one |
| `last` | direction of the last scan |
| `hp`, `loaded`, `bonus` | health 1-4, gun ready (reloaded or bonus stored), bonus shots 0-2 |
| `x`, `y`, `WIDTH`, `HEIGHT` | my position (`0,0` = north-west corner) and the arena size |
| `heard` | a place: the cell of the nearest shooter heard, `none` before any; kept until a newer sound, reset on respawn. A new sound is `heard != pin[k]` |
| `N E S W` | the numbers 0 1 2 3, so they fit in `num` and support arithmetic |
| `trail[i]` | `i` beats ago, `trail[0]` = the latest beat, up to 3 entries (`trail.length`) |

**Trail entry fields:** `x`, `y` (my position after that beat); `act` (`SCAN` 0, `MOVE` 1, `FIRE` 2, `WAIT` 3, `IDLE` 4 =
missed beat); `dir` (the action's direction, -1 for `WAIT`/`IDLE`); `ok` (move: I moved; fire: a shot left;
scan/wait: true); `hit`, `bump` (-1, or the direction I was hit / bumped from on that beat).

The trail does not store scan readings.

## 6. The scan

`scan(d)` looks at the **cone** opening in direction `d` (a 90 degree wedge). Walls block the view (no straight line
between cell centres = not seen; grazing a wall corner still sees). Scans pass through bots, coins and bullets and are silent. Each reading is a **level 0-3 = how lined up** the best-aligned object of that kind is: 3 = on my line, 2 = one
lane off, 1 = elsewhere in the cone, 0 = nothing. `bullet` counts only bullets flying toward me still in the air after
this beat's flight (one that hit something this beat is gone), so a sidestep next beat always dodges it. No side or distance.

## 7. Built-ins

```js
scan(d)  move(d)  fire(d)  wait()   // actions: block until the next beat

// free (no beat)
left(d) right(d) back(d)      // turning
pick(N, N, S)                 // random direction, weighted by repeats
chance(30)                    // true 30% of the time
rand(lo, hi)                  // random integer, inclusive
wall(x, y)                    // true = wall or outside the arena, false = floor; never reveals bots, bullets, coins
abs(x) min(x, y) max(x, y) dist(x1, y1, x2, y2)   // dist = Manhattan

// places (section 4)
here()  at(x, y)              // the cell I stand on / the cell (x, y), stored as given
free()  count()               // first empty pin index (-1 if all 8 are set) / number of pins set

// navigation (no beat; walking over terrain: walls block, bots and coins do not)
path(p)                       // first step toward place p on a shortest walk; -1 if p is none, a wall, off the
                              // map, unreachable or my own cell
steps(p)                      // walking distance to p (0 on my cell), -1 if unreachable or none
nearest()                     // index of the set pin with the fewest steps, -1 if none is reachable
```

- `pick` skips walls; if none of the listed directions is open it returns the first one.
- Ties: `path` goes in the order N, E, S, W, `nearest` takes the lowest pin index. These cost ops like any call, not per cell walked.

## 8. Allowed syntax

`if/else`, `while`, `for`, `break`, `continue`, `return`, `? :`, `&& || !`, `+ - * / %` (integer division),
`== != < <= > >=`, assignment `= += -= ++ --` (`num[i]`; `pin[i]` takes `=` only), member reads on `trail[i]` and `.x` / `.y` of a place, `function` declarations,
`//` comments. Anything else is a parse error with a hint. Limit: 120 lines.

## 9. Bots

Sources live in `arena/engine/lang/bots/*.js`; each header comment has a one-line strategy recipe (hunter, forager, mode switch).
The bot sources are the single copy; the prompt includes them. Templates: Rookie, Lighthouse, Tracker, Forager, Hound.

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
