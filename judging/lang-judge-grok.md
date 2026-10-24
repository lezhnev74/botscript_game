# Language judge

Ship L1's rule list. Add L3's modes behind a flag, and L2's `it` rule plus finished-line inserts. Reject L4 as the source text and L5 as the runtime.

## Scores

| Design | a feel | b taps | c loop | d AI | e TV | f impl | Sum |
|---|---:|---:|---:|---:|---:|---:|---:|
| L1 rules | 8 | 9 | 8 | 8 | 8 | 9 | 50 |
| L2 code | 10 | 8 | 8 | 9 | 7 | 5 | 47 |
| L3 modes | 7 | 8 | 8 | 7 | 9 | 7 | 46 |
| L6 reflex | 8 | 7 | 7 | 6 | 8 | 8 | 44 |
| L4 sentences | 6 | 8 | 6 | 7 | 6 | 4 | 37 |
| L5 assembly | 9 | 5 | 5 | 7 | 4 | 4 | 34 |

- L1. The one to build. Flat `when/do`, dry-gun fall-through, marks. Kite is miscounted by a tap.
- L2. The best language and a bad first phone. Keep the editor tricks. Leave indent, `for`, and `seq` out of v1.
- L3. The best picture on the TV. The 6-tap Lamp count is real. The 16-name menu and the silent GO cycle are not.
- L6. Honest 6 taps, and the bot still cannot shoot. Jobs are a 3-step script standing where a state machine should be.
- L4. Fastest preset, worst canonical form. People will write English the grammar cannot parse.
- L5. Highest raw ceiling, worst room. Two taps to Duck only if the gap sits above the sweep.

### Tap audit

After join. The join button is not an edit.

- L1. "1 tap" is PLAY. Seed already fires. Dodge recipe is 2. Manual dodge is 3. MOVE away after ARMED is 5, because `it` is the default dir, plus 3 for `and not LOADED`. Kite is 8, not 4+3. Two-minute bot is ~18 taps, not 16-17.
- L2. Lamp at 7 checks out: 3 for `and loaded`, 2 per snippet. A raw rule is 4. Wolf at 60-80 hand taps is plausible.
- L3. Lamp at 6 checks out: two finished rules, then WHEN, ON_LINE, THEN, FIRE THAT as one button. First shot is tap 6. Zero taps only sweeps.
- L4. Custom dodge sentence is 5 with both autofills. Agreed. The 1-tap start is a full preset. Four custom sentences at 15 is the low end.
- L5. Duck is 2 only if the gap is above SWEEP. Bottom `+` twice builds the sleeping test. Moth at 39 is plausible and already past "seconds".
- L6. The 6 taps check out, and that program has no FIRE. First bot that shoots is about 11, or a template.

## Best ideas

1. L1 feasibility. `when ON_LINE any do FIRE it` sits quiet while the gun is dry, and a lower rule runs. The newcomer never learns LOADED. That is the whole first minute.
2. L1 urgency insert. A dodge added at the end still lands at row 1. Kills the classic shadowing bug on the first rule people add.
3. L1 suggested verb. BULLET offers SIDESTEP, STEP_IN offers LEAD, COIN offers MOVE it. Tap 3 is usually the right action.
4. L1 skip line on the phone. `skipped 9x: gun empty` next to `x41`. The TV shows the shot. The phone shows the rule that almost fired.
5. L1 mark. One sticky: on or off, optional countdown, optional dir, written as `then mark @bait 4 at it`. Bait, feint, cooldown, and a crude mode, one concept.
6. L2 runs from the top every beat. No program counter. Hot-swap means the next lock uses the new text. This is the argument against L5's dot.
7. L2 `it` stays in scope only under the test that bound `any`, and the keypad hides it otherwise. Take that over L1's looser `it`.
8. L2 greys lines under an action that always runs. First-match shows up before the first death.
9. L3's huge word. `HUNT` at 4 m, `#2 FIRE E` under it. A lying name is a joke. The badge has to stay true.
10. L3 `after n`, hidden until a second mode exists. Bait length and flee length are the AI use. Newcomers never see the chip.
11. L3 latch on the way out. `GO BAIT MARK THAT` stores the dir once. The decoy list does not re-pick a target every beat.
12. Live decision on the phone: L2's yellow arrow, L6's pulsing chip. This snapshot, this row, before the next tap.
13. L1's pencil on the badge for 3 beats. The room sees the patch. L6's forced WAIT reads as a stun.

## Traps

Reject a program counter that survives the beat (L5). Lines above the dot sleep. Numbered GOTO rots on a drag. Hidden ids die on export, so paste is a different bot. Keeping `A == 2` across a rewrite plays the old feint on the new lines.

Reject English as the stored language (L4). Pastes will say "behind cover" and "shoot". Badge `fire at it` has no compass letter. Hot-swap clears mood, queue, timers, and the spot, so a bait fix restarts the bait.

Reject the reflex loop as the core (L6). One always-true handler and the filmstrip never runs. A stray `heard` cancels a 3-step job. The sample `on said N` then moves `TOWARD`, which is not north. `bonus >= 1` in the sample disagrees with `bonus n` in the grammar.

Reject L3's menu of 16 mode names, `say` only on entry, and GO cycles the editor will not flag. Hurt FLEE hopping back to a HUNT that still matches `HP <= 1` steams until death.

Reject L2 nesting, `for`, and `seq` in v1. Restart-from-the-top stays. The block editor is the 60-tap wall.

Reject an op cap the phone cannot preview. Reject fall-through with no skip reason. Reject a template tap counted as authoring.

## Recommended language

RULES. Ordered `when ... do ...` rows. First row whose tests pass, and whose action can happen, wins. `always` is last. Tier 1 is one list. A `modes` flag unlocks up to 4 named lists and `GO`. Memory is 4 marks. Taps edit chips. The stored string is the AST reprint. A `#` note is kept and ignored.

```
program = "bot" STRING ["modes"] NL body
body    = rule* always
        | mode+
mode    = "mode" NAME ":" NL rule* always
rule    = "when" test ("and" test){0,2} "do" action effect? note? NL
always  = "always" "do" action effect? note? NL
test    = "not"? (sensor | "@" NAME | "@" NAME "<=" INT | "after" INT)
action  = MOVE d | SIDESTEP | SWEEP d | PING d | FIRE d | LEAD | WAIT | DECOY d | GO NAME
effect  = "then" ("mark" "@" NAME INT? ("at" d)? | "clear" "@" NAME | "say" d)
d       = N | E | S | W | any | cw | ccw | toward | away | heard | hit | coin | target | it | @NAME
note    = "#" text
```

`mode`, `GO`, `after`, `DECOY`, and `say` require the flag. NAME is 1-12 letters. Caps: 12 rules a list, 3 tests a rule, 4 marks, 4 modes.

Sensors, dir defaults to `any`: BULLET, HIT, SCANNED, ARMED, ON_LINE, STEP_IN, ENEMY >= faint|near|locked, LOCKED <= n, LOADED, BONUS >= n, HP <= n, BLOCKED, HEARD, COIN, ALLY_NEAR, FRIEND, ALLY_SAID, CHANCE at 1/2, 1/3, or 1/4.

`it` binds to the first positive dir-sensor in that row. `any` is nearest, ties N E S W. No binder, no `it` on the keypad. Delete the binder and `it` snaps to `cw`.

A row that cannot act fails, and a lower row may run. FIRE needs a loaded gun or a bonus pip. LEAD needs STEP_IN. MOVE needs an open cell. DECOY needs a charge. `it`, target, coin, `@mark` must resolve. Dry gun and spawn shield both fall through. The phone shows `skipped: gun empty`.

Join inserts a bot that already fights. 0 taps:

```
bot "Bo"
when ON_LINE any do FIRE it
always do SWEEP cw
```

First authored bot, 2 taps: `+ rule`, recipe Dodge. Urgency puts DANGER above TARGET.

```
bot "Bo"
when BULLET any do SIDESTEP
when ON_LINE any do FIRE it
always do SWEEP cw
```

`+ and` stops at 3 tests. Swipe deletes. `always` stays. A hand-built dodge, with no recipe, is 3 taps.

Each beat: snapshot, tick marks, tick `after`, walk the current list. `then` runs before a hop, so the destination sees the mark. `GO` clears `after` and walks the target once. A second entry this beat is WAIT plus steam. `say` reaches allies next beat. Badge `#2 FIRE E`. With modes, the mode name is large, then the badge, then `@bait 3`. Kill-cam prints the winning row and the hop.

Hot-swap applies at the next lock when the picker closes. One push per 3 beats, newest wins, HOLD batches. Surviving mark names keep timer and dir. The same mode name keeps `after`. Deleting the live mode drops you on mode 1 and clears `after`. HP, reload, pips, and scan memory stay. Pencil for 3 beats.

Newcomer, 10 s: the 2-tap bot above. The interpreter is that walk and one hop flag. No eval, no tier-1 op budget.

Regular, ~2 min, about 18 taps from the seed:

```
bot "Bo"
when BULLET any do SIDESTEP
when STEP_IN any do LEAD
when ON_LINE any and not FRIEND it do FIRE it
when ARMED any and not LOADED do MOVE away
when COIN any do MOVE it
always do SWEEP cw
```

AI, one prompt. Three tests on the fire row is the cap: ON_LINE, not ARMED, not FRIEND.

```
bot "Wolf" modes
mode hunt:
when BULLET any do SIDESTEP
when HP <= 1 do GO flee then mark @flee 8
when STEP_IN any do LEAD
when ON_LINE any and not ARMED it and not FRIEND it do FIRE it
when ENEMY any >= near and not @bait do GO bait then mark @bait 4 at it
always do SWEEP cw
mode bait:
when after 3 do GO hunt then clear @bait
when SCANNED any do GO hunt
always do DECOY away
mode flee:
when after 6 and not HP <= 1 do GO hunt then clear @flee
always do MOVE away
```

## Open questions

1. Rookie heat: refuse the `modes` flag in the parser, or show it and leave it unused?
2. Anti-pilot rate: one push per 3 beats, or L2's patch 1.2 s after the last tap?
3. What lands in week 1: rows only, rows plus marks, or the `modes` flag too?
4. Are Dodge, Shoot, Lead, Coins, Ambush the right five recipes?
5. A revealed bot copied off the TV replaces the whole program, or appends rows?
6. Free mode names, knowing a huge `HUNT` can be a flee?
7. Does `say` ship for the first team night, with DECOY deferred to v1.1 as the vision has it?
8. Confirm dry-gun fall-through. I want the sweep under a fire row to run while you reload.
9. When an AI starts stacking marks just to count to 3, do we add one integer, or hold the line?
10. Vision still allows a text box for the first sessions. This brief does not. I would ship the tap editor in v1 and keep paste for the AI only.
