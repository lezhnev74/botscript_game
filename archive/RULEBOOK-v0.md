# RULEBOOK - the bot language

Synthesis of 6 competing languages (`lang/L1-L6`, written by Opus, Sonnet, Grok and GLM) and 2 independent judges (`judging/lang-judge-opus.md`, `judging/lang-judge-grok.md`).

## Verdict

| # | Language | Opus | Grok | Taken into RULEBOOK |
|---|---|---|---|---|
| L1 | Rulebook (priority rules) | 50 | 50 | **Core**: rules, feasibility, `it`, suggested actions, urgency insert, phone debugger |
| L2 | Tapcode (Python-ish) | 45 | 47 | Code-editor feel: line numbers, live arrow, dead-code greying, `it` scoping, finished-line inserts, byte-exact reprint |
| L3 | Modes (state machine) | 45 | 46 | **Tier 2**: modes, big mode word on TV, `after N`, mark latch on exit, bad paste keeps the live bot |
| L6 | Reflex (loop + events) | 43 | 44 | **Patrol**: the `else` line is a 1-4 step filmstrip |
| L4 | Dicta (English sentences) | 31 | 37 | Rejected: AI writes synonyms the parser can't read, wordy, doesn't localize |
| L5 | Tiny (assembly) | 30 | 34 | Rejected: numeric gotos rot, a persistent PC breaks hot-swap, unreadable on TV |

Both judges independently chose the same base (L1), the same tier 2 (L3 modes) and the same traps.

---

## 1. The idea in one breath

A bot is a **numbered list of rules, checked top to bottom every beat**:

```
when <test> [and <test>] do <action>
```

The **first rule that is true AND can actually act right now** wins. If none does, the `else` line plays its **patrol**, a short loop of 1-4 steps.

It looks and edits like code: line numbers, syntax colors, a live execution arrow. But every token is picked by tap, so the program can never be invalid.

## 2. Grammar (the text IS the language)

```
program := 'bot' NAME NL (rule* else | rule* mode+)        # top rules before modes = global
mode    := 'mode' MODE ':' NL rule* else                   # tier 2, max 4
rule    := 'when' test ('and' test){0,2} 'do' action effect* note? NL
else    := 'else' 'do' step (',' step){0,3} note? NL       # patrol, 1-4 steps
test    := 'not'? ( SENSOR dq? level? | 'after' N | 'mark' )
effect  := 'go' MODE | 'mark' dir | 'say' dir              # applied after the beat locks
step    := action                                          # infeasible step = WAIT
action  := MOVE dir | SIDESTEP | SWEEP dir | PING dir | FIRE dir | LEAD | WAIT | DECOY dir
dq      := any | N | E | S | W | mark                      # sensor direction, default any
dir     := N | E | S | W | cw | ccw | it | toward | away | target | coin | heard | hit | mark | said
MODE    := HUNT | BAIT | FLEE | HIDE | LOOT | GUARD | KITE | RUSH | FLANK | CAMP | PATROL | STRIKE
note    := '#' text                                        # kept verbatim, shown as caption
```

**Canonical form**: lowercase keywords, UPPER sensors and actions, single spaces. The release test is `print(parse(x)) == x`.

**Caps**:

| | Tier 1 | Tier 2 |
|---|---|---|
| Rules | 8 | 24 |
| Modes | - | 4 |
| Tests per rule | 3 | 3 |
| Patrol steps | 4 | 4 |

The patrol line is free and doesn't count toward the rule cap. There are no loops and no jumps within a beat, so there is no op budget and no overheat.

### Sensors (passive, free, default direction `any`)

| Group | Sensors |
|---|---|
| DANGER | `BULLET`, `HIT`, `SCANNED`, `ARMED` (enemy shows loaded muzzle) |
| TARGET | `ON_LINE` (clear line to an enemy), `STEP_IN` (their last move enters my line), `ENEMY >= faint/near/locked` (last scan), `LOCKED <= n` (ping distance) |
| ME | `LOADED`, `BONUS >= n`, `HP <= n`, `BLOCKED` |
| WORLD | `HEARD`, `COIN`, `FRIEND` (ally on my fire line), `ALLY_SAID`, `ALLY_NEAR`, `CHANCE 1/2, 1/3, 1/4` |
| STATE (tier 2) | `after n` (beats spent in this mode), `mark` (a latched direction exists) |

Scan memory stays fresh for 2 beats.

### Three rules that make it newcomer-proof

1. **Feasibility.** A rule only fires if its action can happen now:
   - `FIRE` needs a loaded gun or a bonus shot.
   - `MOVE` needs an open cell.
   - `LEAD` needs a STEP_IN.
   - `it`, `target`, `coin` and `mark` must resolve to a direction.

   So `when ON_LINE any do FIRE it` is silent while reloading, and the rule below it runs. Newcomers never have to learn `and LOADED`.
2. **`it`.** It is the direction of the rule's first positive directional test. With `any`, it picks the nearest match, ties broken N E S W. The keypad offers `it` only when such a test exists, and deleting that test snaps `it` to `cw`. The whole sniper is `when ON_LINE any do FIRE it`.
3. **Effects apply after the lock.** `go`, `mark` and `say` take effect next beat, and there is no same-beat jump. Mode cycles therefore can't hang the bot, and there is no overheat to explain.

## 3. The phone editor (looks like an IDE, works like a menu)

```
+--------------------------------------+
| Toaster        ● LIVE    [HOLD] [AI] |
|--------------------------------------|
| 1 when BULLET any     do SIDESTEP x41|
| 2 when ON_LINE any    do FIRE it  x12|
|   ┆ skipped 9x: gun empty            |  dashed = true but couldn't act
| ▶3 when COIN any      do MOVE it   x3 |  ▶ = the rule running on TV right now
| 4 when ENEMY any≥near do PING it ⚠never|  shadowed by a rule above
|--------------------------------------|
| else do [SWEEP cw] [MOVE toward] [+]  |  patrol filmstrip, next step pulses
|--------------------------------------|
|  [ + RULE ]                          |
+--------------------------------------+
```

- **`+ RULE`** opens the **sensor sheet**: 5 colored groups of big tiles, a NOT toggle, and a **Recipes** row on top (Dodge, Shoot on sight, Lead, Grab coins, Ambush).
- **Sensor tap**: the test is created with its defaults and the **action sheet** opens. Its first tile is the suggested action for that sensor:

  | Sensor | Suggested action |
  |---|---|
  | BULLET, HIT, ARMED | SIDESTEP |
  | ON_LINE, LOCKED, SCANNED, HEARD | FIRE it |
  | STEP_IN | LEAD |
  | ENEMY | PING it |
  | COIN | MOVE it |
  | HP | MOVE away |

- **Urgency insert**: a new rule lands above the first rule of a lower class (DANGER > TARGET > ME/WORLD). A dodge added last still lands at #1.
- **Editing**:
  - Tap any token to get a small row of legal replacements.
  - `+and`, `NOT` and `then...` (go, mark, say) appear on the selected rule only.
  - Drag a line number to reorder, swipe to delete, with an undo toast.
- **Impossible to break**: every insert arrives complete, pickers only offer legal tokens, `else` can't be deleted, and the caps hide `+`.
- **Debugger built in**:
  - Hit counters (`x41`).
  - Skip reasons (`skipped: gun empty`).
  - `⚠never` warnings for rules shadowed by a rule above.
  - Dead-code greying under an always-true rule.
  - The ▶ arrow synced to the TV picture, not to the sim.

  The TV shows what happened, and the phone shows why.
- **`</>` toggle** shows the raw text: read-only, copyable, and the **paste box for AI**. A bad paste keeps the live bot running and shows red cards with plain fixes (`line 3: "it" needs a direction test, e.g. ON_LINE any`).
- **Modes (tier 2)** appear as tabs once a `go` names a new mode. Global rules sit above the tabs.

### Tap counts (verified by both judges)

| Task | Taps |
|---|---|
| Join: a starter bot is already fighting | **0** |
| Add a dodge (recipe) | **2** |
| Add a dodge by hand (`+RULE`, BULLET, SIDESTEP) | **3** |
| Any rule whose suggested action is right | 3 |
| Change a direction (any -> E) | 2 |
| Add `and not LOADED` | 3 |
| Add a patrol step | 3-4 |
| The "regular" bot below, from the starter | ~22 |

## 4. Execution per beat

1. Snapshot the sensors. Tick `after`.
2. Walk the global rules, then the current mode's rules, top down. The first rule that is true and feasible locks its action.
3. If no rule fires, the patrol plays its next step. The patrol index pauses while rules fire and wraps at the end. An infeasible step becomes WAIT.
4. Apply effects: `go` (resets `after`), `mark` (latches a direction), `say` (allies read it next beat).

The cost is at most 24 x 3 lookups. It is deterministic, and a replay = seed + program versions.

**On TV:**
- Rule badge `#2 FIRE E` in the bot's scan tint.
- Patrol badge `~2 SWEEP N`, dimmer, so calm vs. reacting reads instantly.
- With 2+ modes, the **mode word** big above the bot (`BAIT`), plus an arrow for `mark`.
- The kill-cam quotes the winning rule and any mode hop.

## 5. Hot-swap (edit any time)

- Closing a picker = commit. It binds to the first beat the sim hasn't computed yet. The TV shows a **pencil on the badge for 3 beats**, so the room sees "Bo just patched".
- **Rate limit**: 1 swap per 3 beats, and the latest version wins. **HOLD** batches several edits into one push.
- **What survives a swap**:
  - HP, reload, bonus shots and scan memory always survive.
  - Mode, `after` and `mark` survive if the mode name still exists. Otherwise the bot falls back to the first mode.
  - The patrol index resets.
- A dead bot takes the swap at respawn.
- Steering by editing is too slow: at least 3 taps plus the rate limit loses to any real rule.

## 6. Three programs

**Starter, 0 taps.** It is deliberately flawed: it never dodges.

```
bot "Toaster"
when ON_LINE any do FIRE it
else do SWEEP cw
```

It gets shot, and the kill-cam says `#else SWEEP cw - never dodged`. The newcomer taps `+RULE`, `BULLET`, `SIDESTEP`, and the next life visibly survives. That is programming and debugging, under 30 s and under 5 taps.

**Regular, 2 min (~22 taps).**

```
bot "Magpie"
when BULLET any do SIDESTEP
when STEP_IN any do LEAD
when ON_LINE any and not FRIEND it do FIRE it
when ARMED any and not LOADED do MOVE away   # kite while reloading
when COIN any do MOVE it                     # coins = bonus shots
when ENEMY any >= near do PING it
else do SWEEP cw, SWEEP cw, MOVE toward
```

**AI, one prompt (tier 2).**

```
bot "Wolf"
when BULLET any do SIDESTEP
mode HUNT:
  when HP <= 1 do MOVE away go FLEE
  when STEP_IN any do LEAD
  when ON_LINE any and not ARMED it do FIRE it   # punish whoever just fired
  when ENEMY any >= near do DECOY it go BAIT mark it
  when CHANCE 1/3 do SWEEP ccw                   # unreadable scan pattern
  else do SWEEP cw, SWEEP cw, MOVE toward
mode BAIT:
  when ON_LINE mark do FIRE it                   # they walked into the lure
  when SCANNED any do SIDESTEP go HUNT
  when after 3 do PING mark go HUNT
  else do WAIT                                   # stay dark
mode FLEE:
  when ON_LINE any do FIRE it
  when COIN any do MOVE it
  when after 6 do SWEEP cw go HUNT
  else do MOVE away
```

## 7. Depth (what each tactic costs)

| Tactic | How |
|---|---|
| Ambush | `else do WAIT` (goes dark) + `when SCANNED any do FIRE it` / `when HEARD any do FIRE it` |
| Lead shot | `when STEP_IN any do LEAD` |
| Kite | `when ARMED any and not LOADED do MOVE away` |
| Bait / feint | DECOY + `go BAIT mark it` + `after N` + `ON_LINE mark` (Wolf) |
| Coin double-tap | Feasibility keeps FIRE live during reload while `BONUS >= 1` |
| Anti-predictability | `CHANCE`, mixed patrols |
| Team play | `then say it` on a sighting, and the ally runs `when ALLY_SAID any do MOVE said` / `FIRE it`. `not FRIEND it` stops team kills |

The ceiling for AI: 24 rules x 3 tests x 4 modes x timers and a direction latch is a small but complete reactive controller. No integers in v1. If AIs start chaining modes just to count, add one counter later.

## 8. AI path

- A **Prompt Pack** behind a "Copy AI prompt" button: this grammar, the beat order, the numbers, the 4 templates, and the currently revealed top bots.
- The AI returns text, which you paste and see as cards.
- `#` notes let the AI explain its tactics on the phone.
- Same caps as everyone, and same sensors: an AI makes the bot **smarter, never better informed**.

## 9. Known weaknesses (accepted)

- **Shadowing** (a broad rule eating the ones below) is the #1 bug. Urgency insert, `⚠never` and dead-code greying soften it. The kill-cam teaches the rest.
- **Feasibility skips are visible on the phone, not the TV.** That is fine: the TV shows behavior.
- **No arithmetic.** "Count to 3" is awkward. Hold the line until the meta proves it is needed.
- **The lookahead director** means a patch appears on TV ~3 beats after the tap. The pencil badge covers it.
- **Mode names can lie** (a rout named `HUNT`). That's a feature, and the badge stays truthful.

## 10. Open questions

1. **Tier 1 rule cap**: 8 or 12?
2. **Swap timing**: commit on picker close (recommended), or 1.2 s after the last tap (L2)?
3. **Week-1 scope**: rules + patrol only, then modes in week 2? Recommended.
4. **Copying a revealed bot**: replace the whole program, or append its rules?
5. **Rookie heat**: hide tier 2 entirely, or just leave it unused?
