# L2 - TAPCODE: real structured code, edited by taps

## 1. Idea
A tiny Python-looking imperative language. The program is a **tick function**: every beat it runs top to bottom from scratch, and the **first action it reaches ends the tick** (like `return`). Falling off the end = `wait`. Variables persist between beats. The phone is a structured code editor: every token is a chip you pick, nothing is typed, and the buffer is always a valid program.

Why a tick function and not a long-running script that yields: hot-swap is trivial (no hidden program counter), the TV badge is just "line N", and the state of a bot is readable (its variables). Sequences, which a tick function does badly, get one dedicated block: `seq`.

## 2. Grammar (the text IS the language)
Canonical form: 2-space indent, one statement per line, lowercase, `#` comments (kept on import, anchored to the next line). Parser output re-printed = input, byte for byte.

```
bot Name
stmt  := if | for | seq | set | action
if    := "if" cond ":" block { "elif" cond ":" block } [ "else:" block ]
for   := "for k in NESW:" block            # unrolled 4x, k is a dir
seq   := "seq:" { action | "until" cond }  # one item per beat, hidden index, wraps
set   := "set" var "=" value               # var: a b c (int 0-9, saturating), m (dir), t (timer)
block := indented stmts (>= 1, max nesting 3)
```
`t` is a timer: it counts down 1 per beat to 0 on its own (`set t = 3` = "for 3 beats").

**Expressions.** `cond` = sensor | `int cmp int` | `cond and cond` | `cond or cond` | `not cond` (max 3 operators). `cmp` = `== != < <= > >=`. Ints: 0-9, `a b c t hp bonus`, `rand(10)` (0-9).

**Directions.** `N E S W`, `any`, `it`, `m`, `k`, `opp(x)`, `cw(x)`, `ccw(x)`.
- `any` in a sensor means "whichever direction matches, nearest first, ties N E S W". It also sets `it` to that direction.
- `it` is valid only inside the body of an `if` whose condition contains an `any` sensor joined by `and`.

**Sensors** (free, passive, from VISION): `bullet(d) hit(d) heard(d) scanned(d) line(d) stepin(d) armed(d) blocked(d) friend(d) coin(d)`, `enemy(d) >= faint|near|locked` (last sweep or ping, fresh for 2 beats), and bare flags/ints `loaded`, `bonus`, `hp`, `allynear`, `dist`.

**Actions** (exactly one runs per beat): `move(d) sidestep sweep(d) ping(d) fire(d) lead wait decoy(d) say(d)`.
`sweep(cw)` means "one step clockwise from my last sweep".

## 3. The tap editor
```
+------------------------------+
| Lamp         [live] [undo]   |  name, live/deploy toggle, undo
|  +                           |  thin "+" gutter row (insert here)
| 1 if bullet(any):            |  line numbers = TV badge numbers
| 2   sidestep                 |
|  +                           |
| 3 if line(any) and loaded:   |  yellow arrow = line firing right now
| 4   fire(it)                 |
| 5 sweep(cw)                  |
+------------------------------+
| keypad: only VALID next tokens|
| [bullet][line][enemy] [loaded]|
+------------------------------+
```
- **"+" gutter**: opens a sheet. Top: 8 **ready-made lines** ranked by context, e.g. `if bullet(any): sidestep`, `if line(any) and loaded: fire(it)`, `if coin(any): move(it)`. Below: raw kinds `if / for / seq / set / action`. Any insert arrives **fully filled with defaults**, so the program never has holes.
- **Tap a token** = keypad shows only type-correct, scope-correct replacements (dirs for a dir slot, `it` only if in scope). Tapping `and`/`or` chips appends a term. Defaults are smart: `fire(` arg defaults to `it` when in scope, else `N`.
- **Tap a line number** = action bar: up, down, indent into the block above, outdent, duplicate, wrap in `if`, unwrap, delete. Long-press drag also reorders. Whole blocks move with their header. Delete is undoable, no confirm.
- **Impossible by construction**: type slots, scope for `it`, non-empty blocks (deleting the last line of a block inserts `wait`), nesting depth 3, 24 lines max (+ greyed out at cap). Lines after an unconditional action are greyed as **dead code** (a real IDE feel, and a hint).
- **Import**: paste text. Errors show on the exact line as red chips with a plain fix ("`it` has no sensor above it - tap to wrap in `if enemy(any)`").

**Default bot (0 taps, already plays):** new bots start as
```
bot Rookie
if line(any):
  fire(it)
sweep(cw)
```
**First 10 s, 5 taps:** tap `line(any)` line (1), `+and` (2), `loaded` (3) -> `if line(any) and loaded:`. Then gutter `+` above line 1 (4), snippet `if bullet(any): sidestep` (5). Bot now dodges and stops wasting shots; the TV shows the effect immediately.
**Full Lamp from the seed: 7 taps** (3 + 2 per extra snippet rule). **Hand-built without snippets:** `+`, `if`, `bullet`, `sidestep` = 4 taps per simple rule, ~6 for a two-term one.

## 4. Execution per beat
1. Snapshot sensors, decrement `t`.
2. Run the tick: visit statements top to bottom. Each statement visited (an `if` test counts once, each unrolled `for` pass counts) costs 1 op. **Cap 16 ops.** Over the cap = **overheat** (wait, steam puff). First action reached ends the tick.
3. `seq`: when execution reaches it, it runs item `i` (an action, or an `until cond` that waits until cond holds) and advances `i`. If execution never reaches the `seq` (an earlier action fired), `i` is frozen. Wraps at the end. Badge shows `seq 2/4`.
4. The chosen action locks with all others (blind simultaneous lock, VISION section 2).

**TV badge:** `L4 fire(E)` in the bot's tint. Kill-cam prints the path: `L3 line(any) and loaded -> L4 fire(E)`. The owner's phone shows the yellow arrow on that line live, plus variable values in a tiny strip (`a=1 m=E t=2`). Debugger-feel for free.

**Hot-swap** (edits any time):
- Default **live**: an edit applies 1.2 s after the last tap (a burst of taps = one patch), effective from the **next beat's lock**. In **manual** mode a Deploy button applies it.
- Variables `a b c m t` keep their values (fixed slots). `seq` indices reset to 0. Reload, bonus shots, HP are bot state and untouched. The TV shows a "patched" blink on the badge.
- Max 1 patch per 2 beats; extras queue, last one wins. Undo is a patch too.

## 5. Learning curve (three programs)
**Newcomer, 10 s** (see section 3):
```
bot Rookie
if bullet(any):
  sidestep
if line(any) and loaded:
  fire(it)
sweep(cw)
```
**Regular, 2 min** (~25 taps: adds ping, pattern, `seq`):
```
bot Lamp2
if bullet(any):
  sidestep
if line(any) and loaded:
  fire(it)
if enemy(any) >= near:
  ping(it)
seq:
  sweep(N)
  sweep(E)
  sweep(S)
  sweep(W)
```
**AI, one prompt** (hunt / bait / flee state machine in variable `a`):
```
bot Wolf
# a: 0 hunt, 1 bait, 2 flee. m: bait direction. t: bait timer
if bullet(any):
  sidestep
if hp <= 1:
  set a = 2
if a == 2:
  if hp >= 2:
    set a = 0
  else:
    move(opp(m))
elif a == 1:
  if t == 0:
    set a = 0
  decoy(m)
else:
  if line(m) and not armed(m) and loaded:
    fire(m)
  if stepin(any) and loaded:
    lead
  if enemy(any) >= near:
    set m = it
    set t = 3
    set a = 1
  sweep(cw)
```
Ops worst case about 11, under the cap. The AI works within the same cap, so depth comes from cleverness, not extra powers.

## 6. Depth
- **Bait / feint:** `decoy(m)` plus state `a` plus timer `t` (Wolf), or `say` that lies.
- **Ambush:** `if scanned(any) and loaded: fire(it)` / `if heard(any) and loaded: fire(it)`, else `wait` (goes dark).
- **Lead shot:** `lead`, or manual: `if stepin(any): fire(it)`.
- **Kite:** `if enemy(any) >= near and not loaded: move(opp(it))` then re-fire when `loaded` (reload-aware).
- **Sweep patterns / anti-predictability:** `seq`, `rand(10) < 3`, `for k in NESW` loops (`for k in NESW: if blocked(k): ...`).
- **Counter-meta memory:** `b` counts times `hit(any)` came from one side, `set m = it` stores a direction across beats.
- **Team:** `say(d)` / `friend(d)`, `allynear`; one `a` slot can double as a shared-signal mode. (Cross-bot read is `ALLY SAID` in VISION; add `said(d)` sensor if wanted, it is a pure sensor.)
- **Ceiling:** 24 lines, nesting 3, 16 ops, 5 variables, one action per beat. Real but finite; an AI can't brute force past the cap.

## 7. Weaknesses (honest)
- **Tap cost scales with expertise.** Two-term conditions are ~6 taps; Wolf is 60-80 taps by hand. Phones are bad at code, so a regular will import AI text for complex bots, which is intended, but hand-editing deep nesting hurts (about 12 lines visible).
- **Tick model hides sequences.** "Do X then Y then Z" needs `seq` or a variable. Newcomers hit this wall around minute 5. `seq` is a patch, not an elegant fix.
- **First-action-wins is invisible until learned.** Dead-code greying helps, but "why doesn't my second `if` fire" is the classic first bug. The yellow arrow helps most.
- **`any` / `it` binding is the subtlest part.** It is implicit state, and `it` scoping rules are non-obvious. The keypad hides `it` where it is invalid, but explaining it takes a sentence in the tutorial.
- **Live hot-swap can wreck a good bot** mid-round with one stray tap. Undo is one tap, and the manual Deploy toggle exists, but the default is risky for the sake of feel.
- **Overheat cap is hard to preview.** Deep `for` loops burn ops silently; the editor should show a live "ops: 9/16" counter (planned, adds UI).
- **Reads as code, so it will scare some non-coders.** The 0-tap seed and snippet chips are the mitigation; a pure card face is the alternative if testing shows bounce.
