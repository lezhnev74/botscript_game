# L3 - MODES

## 1. Idea

A bot is named modes, each a short rule list. The first live rule either acts or `GO`s. Newcomers get one mode, a plain list. More modes are tapped in, never drawn. The TV shows the mode as a huge word over the bot, plus a badge for the rule that fired. Memory is the mode, one latched dir `MARK`, and how long the stay has lasted.

## 2. Grammar

Text is the language. A reprint equals the parse. Lowercase keywords are `bot mode say after`. Sensors, actions, and directions are uppercase, one rule a line. `#` is an error.

```
program = "bot" QUOTED mode+
mode    = "mode" NAME ["say" saydir] ":" rule+
rule    = "OTHERWISE" "->" effect
        | cond ["AND" cond] "->" effect
cond    = ["NOT"] sensor
effect  = action | "GO" NAME ["MARK" dir]
action  = "MOVE" dir | "SIDESTEP" | "SWEEP" dir | "PING" dir
        | "FIRE" dir | "LEAD" | "WAIT" | "DECOY" dir
```

`NAME` is SEARCH, HUNT, FIGHT, BAIT, FLEE, CAMP, KITE, RUSH, FLANK, HIDE, LOOT, PING, AMBUSH, GUARD, SCATTER, or DUCK, for thumbs and for paste. The first mode starts. Caps: 4 modes, 8 rules each counting `OTHERWISE`, 24 total. `OTHERWISE` is last and required.

`saydir` is `N E S W` or `MARK`. On entry the bot says it if it resolves. `ALLY SAID` turns true next beat. An empty latch says nothing.

Sensors. Direction slots default to `ANY`.

- Danger: `BULLET`, `HIT`, `SCANNED`, `ARMED`.
- Seen: `ON_LINE`, `STEP_IN`, `ENEMY dir >= faint/near/locked`, `COIN`, `LOCKED <= 2/4/8`.
- Self: `LOADED`, `BONUS >= 1/2`, `HP <= 1/2`, `BLOCKED`, `after 1/2/3/4/5`.
- World: `HEARD`, `ALLY NEAR`, `FRIEND`, `ALLY SAID`, `MARK`, `RAND < 2/3/4`.

`LOADED` is the muzzle on the TV. Pips are `BONUS` only. Roll 1..4 each beat: `RAND < 2` is 1 in 4, `< 3` is 1 in 2, `< 4` is 3 in 4.

Fresh actions: `SWEEP CW`, `PING CW`, `FIRE THAT` when bound else `TOWARD`, `MOVE AWAY`, `DECOY MARK` if any rule latches else `AWAY`.

Dirs: `N E S W CW CCW THAT TOWARD AWAY HIT HEARD COIN TARGET MARK ANY`. `ANY` is one shared dir, nearest, ties N E S W. `ON_LINE ANY AND NOT FRIEND ANY` is one clean line. `THAT` is that dir, effects only. `TARGET` is the best scan hit, `TOWARD` the nearest known enemy, `AWAY` off the nearest threat. Scans last 2 beats.

`after N` is true after N beats already spent here. Optional one `AND`. Must `GO`. Hidden until a second mode exists.

## 3. The tap editor

One column. One mode hides the strip.

```
+----------------------------------+
| Bo          [SEARCH]    undo     |
| BULLET ANY          SIDESTEP   x |
| ON_LINE ANY         FIRE THAT  x |
|                          [+ and] |
| OTHERWISE           SWEEP CW     |
| [ + rule ]              [ + mode ]|
+----------------------------------+
```

A second mode opens `[HUNT] [BAIT]`, dotted on the start mode. A mode nothing `GO`s to goes dim.

- `+ rule` inserts a finished `BULLET ANY -> SIDESTEP` above `OTHERWISE`. 1 tap.
- WHEN, then a sensor. `ENEMY` arrives as `ENEMY ANY >= near`. A direction is two taps. `THAT` and `MARK` show up only when legal.
- THEN lists actions, every `GO` name, then `+ mode`. Tap `mark`, then a direction, for `GO BAIT MARK THAT`.
- `+ and`, then a sensor, is 2 taps, and it stops at two terms. `NOT` is a sheet toggle. `x` deletes. `OTHERWISE` has no `x`. Up and down shift one slot.
- Rename, and `start`, are 2 taps each. Delete says `delete, 2 rules go to HUNT`, retargets them, keeps `MARK`.
- Undo hot-swaps. Templates sit under `examples`.

Bad paste keeps the old bot and a red card: `line 7: after has to GO to a mode`. Also impossible: empty rules, dangling `GO`, action `ANY`, bare `THAT`, an `after` that shoots, a 5th mode, a 9th rule.

**0 taps.** Facing starts `N`. A directed action turns to its compass dir, so `SWEEP CW` steps once a beat.

```
bot "Bo"
mode SEARCH:
  OTHERWISE -> SWEEP CW
```

**1 tap.** `+ rule` adds `BULLET ANY -> SIDESTEP` above `OTHERWISE`. Dodge, else spin.

**Lamp, 6 taps from that spinner.**

1. `+ rule`.
2. `+ rule` again, above `OTHERWISE`.
3. Tap that WHEN.
4. Tap `ON_LINE`. Stays `ANY`, binds `THAT`.
5. Tap THEN.
6. Tap `FIRE`, labeled `FIRE THAT`.

```
bot "Bo"
mode SEARCH:
  BULLET ANY -> SIDESTEP
  ON_LINE ANY -> FIRE THAT
  OTHERWISE -> SWEEP CW
```

Empty gun and no pip skips that `FIRE`.

**New mode, 3 taps:** THEN, `+ mode`, `STRIKE`. The rule `GO`s, and `STRIKE` is `OTHERWISE -> WAIT`. An existing name is 2 taps.

## 4. Execution and hot-swap

One shared snapshot, then the vision order: moves, scans, new shots, bullets, hits, coins.

From the saved mode, `visited` empty:

- Second visit this beat: `WAIT`, steam, badge `OVERHEAT`.
- `GO` sets mode and maybe `MARK`, queues `say` if the new header resolves, walks that mode now.
- A feasible action locks. `FIRE` and `LEAD` need muzzle or pip. `LEAD` also needs `STEP_IN`. `MOVE` needs a free cell. `DECOY` needs a charge, 2 a life. `MARK` needs a latch. `SIDESTEP` needs a side cell. Else the rule is skipped.
- Nothing locked: `WAIT`.

Respawn shield is not a skip. For 2 beats, `FIRE` and `LEAD` become `WAIT` and do not fall through. `say` publishes after all decisions. Allies hear it next beat.

`stay` counts beats that ended here. Passing through resets it. `after 1` is false on entry. Death loads the start mode, clears `MARK` and `stay`, faces `N`.

Big word: the locking mode. Badge: that rule in compass letters, `#2 FIRE E`, or `OVERHEAT`. Kill-cam adds `HUNT #4 GO STRIKE, STRIKE #2 FIRE E`.

The sim leads the picture by about 1.5 s, so a tap binds the next undecided beat and the word blinks late. Shots already locked still fly. Reload, pips, HP, facing, and bullets stay. The same mode name keeps `MARK` and `stay`. A deleted name restarts you on mode one and clears both.

## 5. Three programs

**10 seconds.** The Lamp above. TV word: `SEARCH`.

**2 minutes.**

```
bot "Bo"
mode HUNT:
  BULLET ANY -> SIDESTEP
  ENEMY ANY >= near -> GO BAIT MARK THAT
  ON_LINE ANY AND NOT FRIEND ANY -> GO STRIKE MARK THAT
  OTHERWISE -> SWEEP CW
mode BAIT:
  after 1 -> GO FLEE
  SCANNED ANY -> GO FLEE
  OTHERWISE -> DECOY MARK
mode FLEE:
  after 2 -> GO HUNT
  OTHERWISE -> MOVE AWAY
mode STRIKE:
  NOT LOADED AND NOT BONUS >= 1 -> GO FLEE
  STEP_IN ANY -> LEAD
  ON_LINE ANY AND NOT FRIEND ANY -> FIRE MARK
  OTHERWISE -> GO HUNT
```

**One prompt** sends this grammar, the skip rule, the one-visit rule, and the caps.

```
bot "Wolf"
mode HUNT:
  BULLET ANY -> SIDESTEP
  HP <= 1 -> GO FLEE
  ALLY SAID ANY -> GO STRIKE MARK THAT
  ON_LINE ANY AND NOT ARMED ANY -> GO STRIKE MARK THAT
  ENEMY ANY >= near -> GO BAIT MARK THAT
  OTHERWISE -> SWEEP CW
mode BAIT say MARK:
  after 3 -> GO STRIKE
  SCANNED ANY -> GO FLEE
  BULLET ANY -> SIDESTEP
  OTHERWISE -> DECOY MARK
mode STRIKE:
  NOT LOADED AND NOT BONUS >= 1 -> GO FLEE
  STEP_IN ANY -> LEAD
  ON_LINE ANY AND NOT FRIEND ANY -> FIRE MARK
  MARK AND NOT FRIEND MARK -> FIRE MARK
  OTHERWISE -> GO FLEE
mode FLEE:
  after 1 AND NOT HP <= 1 -> GO HUNT
  COIN ANY -> MOVE THAT
  OTHERWISE -> MOVE AWAY
```

`BAIT` calls `MARK` as the decoy drops. Allies can shoot that lane a beat later. At 1 HP, `FLEE` ignores `after` and does not steam itself.

## 6. Tactics

**Bait.** `GO BAIT MARK THAT`, `DECOY MARK`, `after 3 -> GO STRIKE`. Name it `HUNT` and the TV lies.

**Feint.** `DECOY MARK`, then next beat `MOVE AWAY`. One action a beat, so the split takes two.

**Ambush.** `HIDE`, `OTHERWISE -> WAIT`, dark after 3 still beats. `HEARD ANY` or `SCANNED ANY` does `FIRE THAT`. A dry gun falls through to `WAIT`.

**Lead.** `STEP_IN ANY -> LEAD` above the fire rule. No stepper, skip.

**Kite.** Next beat, `NOT LOADED AND NOT BONUS >= 1 -> GO FLEE`. A pip stays for the second shot.

**Team.** Entry `say` publishes `MARK`. Next beat, `ALLY SAID ANY -> GO STRIKE MARK THAT`. `NOT FRIEND` is the only block on a team kill.

## 7. Weaknesses

The huge word snitches. `FLEE` reads at 4 meters, until a rout is named `HUNT` and only the badge is true.

A hurt `FLEE` that `GO`s back to a `HUNT` still matching `HP <= 1` revisits itself and steams until death. The editor never flags a cycle no action can leave.

Skipped `FIRE` lets a lower rule run, and only the phone shows it. Four modes, two terms, one `MARK`. Camps stay silent, because `say` is entry-only. No timer before the second mode. Sixteen names cannot say `PATROL`.
