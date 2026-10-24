# L1 - RULEBOOK (priority rules, perfected)

## 1. Idea
A bot is **an ordered list of `when <chips> do <action>` rules, read top to bottom; the first rule that matches AND can act wins**. The last row is always `always do ...`. Memory is one extra concept, **marks**: named sticky notes with an optional countdown and an optional remembered direction. Marks are set as a rule's side effect (`then mark @bait 3 at it`) and read back as chips (`@bait`). No tabs, no GOTO, no loops, so the program stays one flat list and every beat costs at most 12 x 3 chip checks. Overheat is not needed.

## 2. Grammar (text form = the language)

```
program  := 'bot' STRING NL rule{0,11} always
rule     := 'when' chip ('and' chip){0,2} 'do' action effect? note? NL
always   := 'always' 'do' action effect? note? NL
chip     := 'not'? (sensor | '@'mark)
effect   := 'then' ( 'mark' '@'mark beats? ('at' dir)? | 'clear' '@'mark | 'say' dir )
note     := '#' text               # kept verbatim, shown as a caption under the rule
beats    := 1..20                  # omitted = until cleared
mark     := [a-z]{1,8}             # max 4 distinct marks per bot
```

**Sensors** (defaults in bold, used when a chip is created by tap):

| Group | Chips |
|---|---|
| DANGER | `BULLET dir`, `HIT dir`, `SCANNED dir`, `ARMED dir` |
| TARGET | `ON_LINE dir`, `STEP_IN dir`, `ENEMY dir >= faint/**near**/locked`, `LOCKED <= n` (ping distance, **4**) |
| ME | `LOADED`, `BONUS >= **1**`, `HP <= **1**`, `BLOCKED dir` |
| WORLD | `HEARD dir`, `COIN dir`, `ALLY_NEAR`, `FRIEND dir`, `ALLY_SAID dir`, `CHANCE 1/**2**` (1/2, 1/3, 1/4) |
| MARKS | `@name` (active), `@name <= n` (beats left) |

Every chip `dir` defaults to **any**.

**Actions:** `MOVE dir`, `SIDESTEP`, `SWEEP dir`, `PING dir`, `FIRE dir`, `LEAD`, `WAIT`, `DECOY dir`.

**Directions:** `N E S W any cw ccw toward away heard hit coin target it @mark`.
- **`it`** is the direction bound by the first positive directional chip in the rule. With `any` and several matches it takes the nearest, ties broken N E S W. This one word removes most direction picking: `when ON_LINE any do FIRE it`.
- `target` = the last SWEEP/PING hit, `toward` = the nearest known enemy, `away` = opposite of the nearest threat, `@mark` = the direction stored in a mark.

**Implicit feasibility (the key smart default).** A rule matches only if its action can happen right now:
- `FIRE` needs a loaded gun or a bonus shot.
- `MOVE` needs an open cell.
- `it`/`target`/`coin`/`@mark` must resolve to a direction.
- `LEAD` needs a STEP_IN.
- `DECOY` needs a charge.

So `when ON_LINE any do FIRE it` waits for the reload by itself, and the newcomer never has to learn `AND LOADED`. If the `always` action is infeasible, the bot does WAIT.

```
bot "Lamp"
when BULLET any              do SIDESTEP
when ON_LINE any             do FIRE it
when ENEMY any >= near       do PING it
always                       do SWEEP cw
```

**Round trip.** Parse -> AST -> canonical print. Canonical form is: lowercase keywords, UPPER sensors, `do` aligned, notes kept. Import shows "reformatted" if only the spacing changed. Errors become red rule cards with a fix, e.g. `rule 3: "it" needs a direction chip like ON_LINE any`.

## 3. The tap editor

```
+--------------------------------+
| Toaster  [LIVE ●]   hp ▮▮▮ 2/3 |  live: firing rule glows, marks shown as tags
|--------------------------------|
| 1 when [BULLET any]  do [SIDESTEP]      x41 |
| 2 when [ON_LINE any] do [FIRE it]       x12 |
|        ┆ skipped 9x: gun empty               |  dashed = matched but infeasible
| 3 when [ENEMY any≥near] do [PING it] ⚠never |  shadowed-rule warning
|   [+ and]                    [then...]       |  shown on the selected rule only
|--------------------------------|
| A always do [SWEEP cw]                       |  pinned, cannot be deleted
|        [ + RULE ]   [AI ⇅]                   |
+--------------------------------+
```

- **+ RULE** opens the chip sheet straight away. It is one scrolling page: 5 colored groups, a 3-column grid of big buttons, a **NOT** toggle at the top, and a **Recipes** row first ("Dodge", "Shoot on sight", "Grab coins", "Ambush").
- Picking a sensor creates the chip with its defaults and flips the sheet to the **action sheet**. The first button there is the suggested action for that sensor:

  | Sensor | Suggested action |
  |---|---|
  | BULLET, HIT, ARMED | SIDESTEP |
  | ON_LINE, LOCKED, SCANNED, HEARD | FIRE it |
  | STEP_IN | LEAD |
  | ENEMY | PING it |
  | COIN | MOVE it |
  | HP | MOVE away |
  | BLOCKED | MOVE cw |

  Directional actions default to `it` when the rule has a binding chip, otherwise to `cw`.
- **Smart insertion.** A new rule lands above the first rule of a lower urgency class (DANGER > TARGET > ME/WORLD > marks-only), so a dodge rule added last still lands at #1.
- **Edit a chip:** tap it to open a small row of values (dir `any N E S W`, level, number), plus `NOT` and trash.
- **Reorder:** drag the handle, or long-press and use the up/down arrows.
- **Delete a rule:** swipe left, with an undo toast.
- **Invalid is unreachable:**
  - Pickers only offer typed, legal values.
  - `it` appears only when a binding chip exists. If you delete that chip, the action falls back to `cw` and flashes.
  - `+ and` hides at 3 chips and `+ RULE` hides at 12 rules.
  - Mark names come from a palette (`@hunt @bait @flee @hide @mad @plan @go @back`) or from marks already used.
- **Live counters** (`x41`) and the **skip line** turn the phone into a debugger. The TV shows behavior, and the phone shows why.

**Tap counts.** A tap = one touch on a target; drags count 1.

| Task | Taps |
|---|---|
| Join with an auto name, starter bot fighting | **1** (PLAY) |
| Add "dodge bullets" via Recipe | 2 |
| Add any rule whose suggested action is right | **3** (+RULE, sensor, action) |
| Same with a non-default dir (`MOVE away`) | 4 |
| Add `and not LOADED` to a rule | 3 (+and, NOT, LOADED) |
| Change a chip dir any -> E | 2 |
| Add `then mark @bait 3 at it` | 4 (then..., mark, @bait, 3; `at it` is the default when bindable) |
| Reorder / delete | 1 / 1 |

## 4. Execution per beat
1. Snapshot the sensors (scan memory lasts 2 beats).
2. Tick the marks: timed marks count down by 1, and a mark at 0 is removed.
3. Walk the rules from 1 to 11. A rule fires if all its chips are true and its action is feasible. If none fires, `always` runs.
4. Lock the action and apply the rule's effect right away (it is visible to next beat's chips). `say` goes out with the beat.

Cost is bounded: at most 36 constant-time chip checks per beat, with no loops.

**TV badge:** `#2 FIRE E`, with active marks as colored tags beside it (`@bait3`). `always` shows as `#A SWEEP cw`. The kill-cam quotes the rule text.

**Hot-swap.**
- A complete edit (the picker closes) auto-pushes. It applies at the **next beat boundary**, atomically.
- Rate limit: 1 swap per 3 beats. The phone queues only the latest version. A **HOLD** toggle batches several edits into one push.
- On swap:
  - Marks whose names still exist survive, with their timers. Removed names are dropped.
  - Scan memory, reload and HP are untouched.
  - Rules renumber.
- The TV shows a pencil on the badge for 3 beats, so the room sees "Bo just patched".
- Piloting by editing costs at least 3 taps, which is too slow to beat a real rule. The rate limit closes what is left.

## 5. Learning curve

**10 s, newcomer.** PLAY (1 tap) runs the starter. It deliberately has no dodge:
```
bot "Toaster"
when ON_LINE any   do FIRE it
always             do SWEEP cw
```
It gets shot. The kill-cam says `#A SWEEP cw - never dodged`, and the phone shows "You were shot from E". The fix is the Dodge recipe (2 taps), or +RULE / BULLET / SIDESTEP (3 taps). The new rule lands at #1 and the next life visibly survives. That is programming, debugged in under 30 s and under 5 taps.

**2 min, regular.** About 17 taps from the starter:
```
bot "Magpie"
when BULLET any                 do SIDESTEP
when STEP_IN any                do LEAD
when ON_LINE any                do FIRE it
when ARMED any and not LOADED   do MOVE away    # kite while reloading
when COIN any                   do MOVE it
always                          do SWEEP cw
```
Tap count:

| Rule | Taps |
|---|---|
| Dodge | 2 |
| LEAD | 3 |
| Kite | 4 + 3 |
| COIN | 3 |
| Reorder | 1 |
| **Total** | **16-17** |

**1 prompt, AI** (Prompt Pack = this grammar plus the beat order):
```
bot "Wolf"
when BULLET any                          do SIDESTEP
when HP <= 1                             do MOVE away then mark @flee 8
when @flee and HEARD any                 do MOVE away
when STEP_IN any                         do LEAD
when ON_LINE any and not ARMED it        do FIRE it          # punish whoever just fired
when @bait and ON_LINE @bait             do FIRE it          # they walked into the lure
when @bait <= 1                          do PING @bait then clear @bait
when @bait                               do DECOY away       # footsteps the other way
when ENEMY any >= near and not @bait     do WAIT then mark @bait 4 at it
when COIN any and BONUS <= 1             do MOVE it
when CHANCE 1/3                          do SWEEP ccw        # unreadable scan pattern
always                                   do SWEEP cw
```

## 6. Depth

| Tactic | How it is written |
|---|---|
| Ambush | `when SCANNED any do FIRE it` + `always do WAIT` (goes dark) |
| Lead shot | `when STEP_IN any do LEAD` |
| Kite | `when ARMED any and not LOADED do MOVE away` |
| Bait | A timed mark with a direction (Wolf above): sense, mark the dir, decoy, punish whoever walks into `@bait` |
| Feint | `when ARMED any and not @fake do MOVE it then mark @fake 1`, then `when @fake do SIDESTEP`: step into their line, slide out on the beat they fire |
| Cooldowns / anti-predictability | `not @fired` ... `then mark @fired 5`, plus `CHANCE` |
| Modes (state machine) | A mark used as a gate on each rule (`@flee and ...`). Hysteresis via `until cleared` marks |
| Team play | `then say it` on the shot, and the ally runs `when ALLY_SAID any and ON_LINE it do FIRE it`. Focus fire and crossfire herding |
| Coin double-tap | `BONUS >= 1` plus feasibility: FIRE stays feasible while reloading if a bonus shot is stored |

The ceiling is real: 12 rules x 3 chips x 4 timed, directional marks is a small but complete finite-state controller with timers and one direction register per mark.

## 7. Weaknesses (honest)
- **State machines are verbose.** Every rule in a "mode" repeats `@flee and`, and 12 rules run out quickly for 3-mode bots. Tabbed states (VISION Tier 2) are cleaner for experts. I trade that for one flat, TV-readable list.
- **No arithmetic or counting.** "Shot 3 times" needs chained marks. AI programs will hit this wall.
- **Implicit feasibility hides causes.** An expert may not realize a FIRE rule is being skipped. The skip line helps, but only on the phone and not on the TV.
- **`it` with `any`** picks the nearest match. That is deterministic, but sometimes not what the author meant, and spelling out a direction costs 2 more taps.
- **First-match-wins shadowing** is the #1 bug class (a broad rule eating the ones below). Static `⚠never` warnings catch the obvious cases only.
- **Hot-swap** still allows slow human steering (for example, flipping a mark gate). The rate limit reduces this but does not remove it.
- Mark names from a palette mean no custom words by tap. AI text can use any name, which the tap editor then can't recreate.
