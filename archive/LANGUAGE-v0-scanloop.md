# SCANLOOP - the bot language

**Look, think, act.** A bot is a small imperative script that loops forever. It scans one direction, reads 3 numbers,
decides (free), then moves or fires. It only feels 2 things without looking: **being hit** and **bumping into a bot**.
Dice (`rand`, `chance`) are first-class: players tune how random their bot is, then add intelligence on top.

Supersedes the RULEBOOK draft (`archive/RULEBOOK-v0.md`). Which rules are agreed vs proposed: see `VISION.md`.

---

## 1. World rules the language relies on

- **Grid with walls.** 4 directions. **No facing**: every action names its own direction.
- **Beat = 1 action.** `scan`, `move`, `fire`, `wait` cost 1 beat each. Thinking (`if`, `chance`, reading values) is free.
- **Moving into a wall is a no-op** (the beat is spent, nothing happens, no event).
- **Moving into a bot fails** and triggers `on bump` for the mover only. The other bot notices nothing.
- **Beat order:** moves -> scans -> new shots -> bullets travel -> hits -> coins. Moves resolve first, so **a bot that steps
  away on the beat you fire wins** (the bullet hits air). This applies to bump-then-fire too.
- **Bullets** fly 3 cells per beat (bots move 1), until they hit a bot or a wall. Close shots land before anyone can react;
  long shots can be dodged by a bot that scans the right way in time. Numbers to be tuned by simulation.
- **Reload:** 3 beats after a shot. Coins give a **bonus shot** (max 2) that fires even while reloading.
  `fire` with nothing loaded = a wasted beat ("click" on TV).
- **HP 3** (so `on hit` matters). No teams: free-for-all.

## 2. The scan

`scan <dir>` looks at the **cone** opening in that direction, from the bot **to the walls**. The 4 cones together cover
the whole arena, so **4 scans = a full picture** and **0 = really nobody there**. Scans see through everything and are
silent (the scanned bot learns nothing).

It returns 3 readings, each a **level 0-3 = how lined up** (not how close) the nearest-aligned object of that kind is:

| level | meaning | for `enemy` |
|---|---|---|
| 3 | on my line | `fire` that way hits, unless it moves |
| 2 | one lane off | one sidestep from a shot |
| 1 | elsewhere in the cone | somewhere over there |
| 0 | nothing | - |

- `bullet` counts only bullets **flying toward me**. `bullet == 3` = it will hit me if I stay on this line.
- No side, no distance. "Level 2 - left or right?" is a guess: that is where dice and intelligence come in.
- Readings keep their value until the next scan (they describe the last scan, wherever it looked).
- Scanning toward an adjacent wall returns all 0.

## 3. Grammar (text form = what AI writes; the tap editor builds exactly this)

```
program  := 'bot' NAME NL 'loop:' block handler*
handler  := 'on' ('hit' | 'bump') ':' block
block    := stmt+                                  (indented)
stmt     := action
          | 'if' cond ':' block ('elif' cond ':' block)* ('else:' block)?
          | 'chance:' (PCT ':' block)+ 'rest:' block
          | 'chance' PCT ':' block                 (sugar for: rest: nothing)
cond     := test (('and' | 'or') test)?
test     := 'not'? ( reading OP N | 'loaded' )
reading  := 'enemy' | 'coin' | 'bullet' | 'hp'
OP       := '==' | '>=' | '<='
action   := 'scan' dir | 'move' dir | 'fire' dir | 'wait' | 'nothing'
dir      := 'N' | 'E' | 'S' | 'W'
          | 'last'           direction of my last scan
          | 'hit'            where the last bullet that hit me came from
          | 'bump'           where my last bump happened
          | 'undo'           opposite of my last move
          | 'rand(' dir (',' dir)* ')' | 'rand(any)'
```

- `nothing` = no action, no beat (an explicit "skip" for `chance`/`else` cases).
- `loaded` = the gun is reloaded or a bonus shot is stored.
- Limits (keep it phone-sized): 30 lines, nesting depth 3, up to 5 cases per `chance`.

## 4. Dice

**`rand(...)` - random direction.**
- `rand(N,S)` picks N or S; `rand(any)` picks one of 4.
- **Repeat to weight:** `rand(N,N,S)` = N 2/3 of the time.
- **`rand` skips walls:** it only picks open directions. At the north wall `rand(N,S)` is always S. If none are open
  (e.g. `rand(N,W)` in the NW corner), the move is a no-op - useful for corner campers.

**`chance` - a switch whose cases are probabilities.**
```
chance:
  60%: scan rand(any)
  30%: move rand(N,S)
  rest: fire last
```
- Cases always sum to 100%: the last case is `rest` and absorbs the remainder.
- Any block can hold a `chance`, and any case can hold `if`s: **logic decides when to gamble, `chance` decides how much.**
- Deterministic: each bot has its own seeded dice stream, so replays are identical.
- TV badge shows the rolled case: `30% -> move S`.

## 5. Execution

- The bot runs its script like a normal program with a program counter. It executes free statements until it reaches an
  action, performs it (that is its beat), and continues from the next statement on the next beat. At the end of `loop:` it
  goes back to the top.
- **No free spinning:** if a whole pass through `loop:` produces no action, the bot `wait`s that beat.
- **Interrupts:** if I was hit (or bumped) this beat, the next beat starts the `on hit` (`on bump`) block instead of the
  current line. When the block ends, the bot **restarts `loop:` from the top**. Hit beats bump if both happen. A new hit
  during `on hit` restarts the handler.
- **No handler = no reaction:** the bot just keeps going.
- Defaults: before the first scan, `last` = random; before any hit/bump, `hit`/`bump` = `last`; `undo` before any move =
  random.

## 6. Tap editor (phone)

The screen is the script itself, one big line per statement, indented. The current line glows live while the bot plays.

- **New bot = a working bot:** the template is already `loop: scan rand(any)` + `if enemy == 3: fire last`.
  **0 taps to play.**
- **`+` between lines** opens: `scan  move  fire  wait | if  chance | nothing`.
- **Direction picker:** `N E S W | last hit bump undo | dice`. `dice` opens 4 toggles with a counter each
  (tap a direction again to weight it).
- **`if`** arrives prefilled as `enemy == 3`. Tap a part to cycle it: reading -> `==/>=/<=` -> `0-3` stepper. `+ elif`, `+ else`.
- **`chance`** arrives as `50% / rest`. `+ case`, and a stepper on each case moves 10% from or to `rest`.
- Long-press = drag to reorder or delete. Invalid programs can't be built: every slot always holds a legal value.
- **Debugger counters:** each line shows how many times it ran this round, each `chance` case shows actual vs set %.
- Text form round-trips exactly: "Copy AI prompt" exports, paste imports (errors show as red lines with a fix hint).

Taps for a newcomer's first edit, "dodge first": `+` above `if` (1), `if` (1), cycle reading to `bullet` (1),
`+` inside (1), `move` (1), `dice` -> N, S (3) = **8 taps**.

## 7. Hot-swap (editing any time)

- Saving swaps the program at the next undecided beat. The new script starts at the top of `loop:`.
  Readings, `last`, `hit`, `bump`, the reload and HP are kept.
- Rate limit: 1 swap per 3 beats. A pencil badge on TV marks edited bots for 5 beats.

## 8. Bots

**Newcomer (template, 2 lines):** looks around at random, fires when lined up.
```
bot Rookie
loop:
  scan rand(any)
  if enemy == 3: fire last
```

**Regular (dodge, coins, wander, bump gamble):**
```
bot Brawler
loop:
  scan rand(E,W)
  if bullet == 3:  move rand(N,S)          # leave the E-W line
  elif enemy == 3 and loaded: fire last
  elif coin == 3:  move last
  else:
    chance 40%: move rand(any)
on hit:  move rand(N,S)
on bump:
  chance:
    50%: fire bump                        # he may have stepped away
    rest: move undo
```

**Corner camper (walls as armor):**
```
bot Possum
loop:
  move rand(N,W)                          # walks into the NW corner, then this is a no-op
  scan E
  if bullet == 3: move S
  elif enemy == 3: fire E
  scan S
  if bullet == 3: move E
  elif enemy == 3: fire S
on hit: move rand(E,S)
```

**AI-level (two axes, side guessing, tuned randomness):**
```
bot Wolf
loop:
  scan E
  if bullet == 3:  move rand(N,S)
  elif enemy == 3 and loaded: fire E
  elif enemy == 2: move rand(N,S)         # one lane off: guess a side, rescan next pass
  else:
    scan N
    if bullet == 3:  move rand(E,W)
    elif enemy == 3 and loaded: fire N
    elif enemy == 2: move rand(E,W)
    elif coin >= 2:  move N
    else:
      chance:
        50%: move rand(any)
        30%: scan rand(S,W)
        rest: wait
on hit:
  scan hit
  if enemy == 3 and loaded: fire hit
  else: move rand(any)
on bump: fire bump
```

## 9. Tactics this expresses

- **Snipe / duel:** scan a line, fire on 3. Two bots reading each other at 3 is a mind game: fire, or dodge?
- **Dodge:** `bullet == 3` -> step off the line. Far shots are dodgeable, close ones aren't.
- **Hunt:** level 2 -> sidestep guess -> rescan.
- **Corner camp:** fewer directions to watch, but only one way to dodge.
- **Coin rush:** `coin == 3` -> walk in, bank bonus shots for a double tap.
- **Bump brawl:** bump -> fire point-blank, or bail out. A coin flip on both sides.
- **Unpredictability as a stat:** `chance` and weighted `rand` hide your pattern from bots that learn where you look.

## 10. Weaknesses (honest)

- **Dodge pairing:** there's no "sideways" word, so every scan direction needs its own dodge axis written by hand.
- **No memory:** a bot can't compare "now vs before" (owner decision: no rose/fell). Hunting by side guessing is crude.
- **Level 2 is a 50/50 guess**, which may feel random rather than smart. Simulation will tell.
- **Scan patterns are visible on TV:** predictable bots get punished - good for the game, harsh on newcomers.

## 11. Open

1. HP 3 vs fewer (one hit = `on hit` is useless).
2. Memory per direction (`enemy@E`) in v2?
3. Beat length, bullet speed, reload, cone shape - after simulations.
4. Should `on bump` also fire for a wall? (Currently no.)
