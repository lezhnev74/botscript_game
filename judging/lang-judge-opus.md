# Language judge - Opus

Criteria: (a) feels like programming, (b) newcomer bot in seconds by taps, (c) edit-watch-fix loop + hot-swap, (d) AI text round-trip + depth ceiling, (e) TV readability, (f) implementation simplicity.

## 1. Scores

| # | Design | a | b | c | d | e | f | Sum | Verdict |
|---|---|---|---|---|---|---|---|---|---|
| L1 | Rulebook (rules + marks) | 7 | 9 | 9 | 7 | 9 | 9 | **50** | The most finished design. Feasibility, `it` and the phone debugger are the best ideas in the set. Marks make modes verbose |
| L2 | Tapcode (Python-ish tick) | 10 | 8 | 7 | 9 | 6 | 5 | 45 | Feels most like real code and has the best AI ceiling. Nesting on a phone, scoping rules and `L14 fire(E)` badges cost too much |
| L3 | Modes | 7 | 8 | 7 | 6 | 10 | 7 | 45 | The huge mode word on TV is the best spectator idea. `#` is a syntax error (AI can't explain itself), and the doc is cryptic |
| L6 | Reflex (loop + handlers) | 7 | 6 | 8 | 5 | 9 | 8 | 43 | The body/reflex split reads well on TV. `on line -> FIRE TARGET` aims at scan memory, not at the line - a real bug. No states |
| L4 | Dicta (English sentences) | 4 | 8 | 5 | 4 | 6 | 4 | 31 | Mad Libs, not programming. Invites AI to hallucinate synonyms, "normalized" is not exact, wordy on TV, a reset on every swap |
| L5 | Tiny (assembly + PC) | 9 | 3 | 4 | 6 | 2 | 6 | 30 | TIS-100 charm and a newcomer wall. Numeric gotos, a persistent PC, `#23 DECOY D` means nothing at 4 m |

**Recounted tap claims:**
- L1: dodge fix = 3 (or 2 via recipe), correct. The regular bot is ~18, not 16-17. Smart insertion drops the LEAD rule below ON_LINE as well, so it needs a second reorder.
- L2: the 5-tap fix is correct. "~25 taps" for the regular bot is unverified - the `seq` block alone is ~10.
- L3: Lamp in 6 is correct. The "1-tap bot" is a dodge-spinner that never fires.
- L4: 5 taps per sentence is correct. The "1-tap first bot" is a template pick, not programming.
- L5: Duck in 2 is true, but only because a magic button writes the gotos. Moth is 39 taps.
- L6: 6 taps is correct, but the bot never shoots and `MOVE N` grinds into the north wall.

## 2. Best ideas

1. **Implicit feasibility (L1, L3).** A rule matches only if its action can happen now, so `FIRE` waits for the reload by itself. This removes the `AND LOADED` tax, the newcomer's #1 bug.
2. **`it` binding (L1; L2/L3 variants).** The direction of the first directional chip, nearest first, ties N E S W. `when ON_LINE any do FIRE it` is the whole sniper. It also fixes L6's aiming bug.
3. **Suggested action per sensor (L1).** BULLET->SIDESTEP, ON_LINE->FIRE it, COIN->MOVE it. A sane rule costs 3 taps.
4. **Smart insertion by urgency class (L1).** A dodge rule added last still lands at #1. This silently prevents shadowing bugs.
5. **Phone as debugger (L1 counters `x41`, skip line, `never` warning; L2 yellow arrow, dead-code greying).** The TV shows what happened and the phone shows why. This is the edit-watch-fix loop.
6. **A deliberately flawed starter (L1).** Join -> the bot fires but never dodges -> the kill-cam says `#A SWEEP cw - never dodged` -> a 3-tap fix. A designed first failure is the best onboarding in the set.
7. **Big mode word over the bot (L3).** `BAIT` / `FLEE` reads at 4 m. It makes state machines a spectator sport. Naming a rout `HUNT` is a legit lie.
8. **`after N` per-mode stay timer (L3).** A timer with no register. It covers bait windows and camping limits.
9. **Patrol filmstrip (L6).** The "nothing happening" branch is a 1-4 step cycle, so sweep patterns need no `seq`, vars or marks. The TV can tint patrol beats calm and reflex beats bright.
10. **Bad paste keeps the live bot running (L3, L5)**, and errors land on the exact line with a fix (all designs).
11. **Notes kept verbatim, shown as captions (L1).** The AI explains its own tactics on the phone, and the round-trip stays exact.
12. **Byte-exact canonical reprint (L2).** The parser and printer are tested as `print(parse(x)) == x`.
13. **Hot-swap at the next beat boundary, rate-limited, with a pencil badge (L1).** The room sees "Bo just patched".

## 3. Traps to reject

- **English as syntax (L4).** It invites synonyms, so AI paste breaks constantly. It doesn't localize. Sentences wrap on phone and TV.
- **Numeric gotos / persistent PC (L5).** Brittle on export, unreadable on TV, and a newcomer wall.
- **Indented nesting on a phone (L2).** 60-80 taps for Wolf, `it` scoping rules, an ops cap the player can't preview.
- **Blind multi-beat commitments** (L4 `fire, then fire`, L6 jobs). These bypass the per-beat reaction that makes the dodge game work. Use modes plus `after N`.
- **Hidden state the TV can't show** (L5 registers, L4 `the spot`, L3/L5 "facing"). If the room can't see it, it can't laugh at it.
- **Swap semantics that silently keep stale meaning** (L2/L5 registers survive a rewrite) **or wipe everything** (L4). Keep state only by name.
- **Text that taps can't recreate** (L1 custom mark names). It breaks "taps are an editor over the text". Use a fixed palette both ways.
- **`#` as an error (L3).** AI output without comments is unreviewable.
- **A WAIT tax on hot-swap (L6).** It punishes the newcomer fixing mid-round, and the rate limit already kills piloting.

## 4. Recommendation: RULEBOOK + MODES + PATROL

**Model.** L1's flat rule list is tier 1. L3's modes are tier 2, entered by a `go` side effect, not a same-beat jump, so there are no cycles and no overheat. L6's patrol is the `else` line. One `mark` direction latch. First feasible rule wins.

**Grammar.**
```
program := 'bot' STR NL rule* (else | mode+)
mode    := 'mode' MODE ':' NL rule* else          # tier 2, max 4; top-level rules are global
rule    := 'when' chip ('and' chip){0,2} 'do' action effect* note? NL
else    := 'else' 'do' action (',' action){0,3} note? NL     # patrol, 1-4 steps
chip    := 'not'? ( SENSOR dq? cmp? | 'after' N | 'mark' )
effect  := 'go' MODE | 'mark' dir | 'say' dir
action  := 'MOVE' dir | 'SIDESTEP' | 'SWEEP' dir | 'PING' dir | 'FIRE' dir | 'LEAD' | 'WAIT' | 'DECOY' dir
dq      := 'any' | N | E | S | W | 'it' | 'mark'           # default any
dir     := N|E|S|W|'cw'|'ccw'|'it'|'toward'|'away'|'target'|'coin'|'heard'|'hit'|'mark'|'said'
MODE    := HUNT|BAIT|FLEE|HIDE|LOOT|GUARD|KITE|RUSH|FLANK|CAMP|PATROL|STRIKE   # fixed palette
note    := '#' text                                         # kept verbatim
```
Caps: 8 rules (tier 1), 24 rules and 4 modes (tier 2), 3 chips. Canonical form: lowercase keywords, UPPER sensors and actions, single spaces. The release test is `print(parse(x)) == x`.

**Sensors:**
- Danger: `BULLET HIT SCANNED ARMED` dq.
- Target: `ON_LINE STEP_IN` dq, `ENEMY dq >= faint|near|locked`, `LOCKED <= n`.
- Me: `LOADED`, `BONUS >= n`, `HP <= n`, `BLOCKED` dq.
- World: `HEARD COIN FRIEND ALLY_SAID` dq, `ALLY_NEAR`, `CHANCE 1/2|1/3|1/4`.
- State: `after n`, `mark`.

**Actions:** as in VISION. `DECOY` is v1.1.

**Semantics per beat:**
1. Snapshot (scan memory lasts 2 beats).
2. Global rules, then the current mode's rules, top down. A rule fires if all its chips are true AND its action is feasible (FIRE needs loaded or bonus, MOVE an open cell, `it`/`mark`/`target` must resolve, LEAD needs STEP_IN). `it` = the direction of the first positive directional chip.
3. Effects (`go`, `mark`, `say`) apply after the lock and are visible next beat. `go` resets `after`.
4. If no rule fires, the next patrol step runs. Its index pauses while rules fire and wraps at the end. An infeasible step becomes WAIT.

The cost is at most 24x3 lookups, with no loops and no overheat.

**TV:**
- Rule badge `#2 FIRE E`.
- Patrol badge `~2/3 SWEEP N`, dimmer.
- A big mode word only when the bot has 2+ modes.
- A mark arrow on the bot.

**Hot-swap.** Closing a picker commits the edit. It binds the first beat the sim has not yet computed. The lookahead director means the TV shows it ~3 beats later, with a pencil badge for 3 beats. Rate limit: 1 swap per 3 beats, latest version wins, plus a HOLD toggle. Mode, `after` and `mark` survive if the mode name still exists, otherwise the bot falls back to the first mode. Patrol index resets. HP, reload and bonus are untouched. A dead bot takes the swap at respawn.

**Tap editor.**
- Rule cards with live counters.
- `+ RULE` opens the sensor sheet (with a Recipes row), then the action sheet with the suggested action first. The rule lands by urgency class.
- Tap a chip to edit it. `+and`, NOT, `then...` (go/mark/say) sit on the selected rule.
- Patrol is a filmstrip with `+step`.
- Modes appear as tabs once a `go` names a new one.

**First bot, exact:**
1. **Join** (1 tap). The starter plays at once:
   ```
   when ON_LINE any do FIRE it
   else do SWEEP cw
   ```
2. It gets shot. The kill-cam says "never dodged".
3. `+RULE` (2), `BULLET` (3), `SIDESTEP` (4).

**4 taps in total** to an authored bot that shoots and dodges, and its fix shows on TV within ~5 s.

**Newcomer, 10 s (4 taps):**
```
bot "Toaster"
when BULLET any do SIDESTEP
when ON_LINE any do FIRE it
else do SWEEP cw
```

**Regular, 2 min (~25 taps):**
```
bot "Magpie"
when BULLET any do SIDESTEP
when STEP_IN any do LEAD
when ON_LINE any do FIRE it
when ARMED any and not LOADED do MOVE away   # kite while reloading
when COIN any do MOVE it
when ENEMY any >= near do PING it
else do SWEEP cw, MOVE toward
```
Tap count, starting from the 4-tap bot:

| Step | Taps |
|---|---|
| LEAD | 3 + 1 drag |
| Kite | 4 + 3 + 1 drag |
| COIN | 3 |
| PING | 3 |
| Patrol step | 4 |
| **Total** | **22** |

**AI, one prompt:**
```
bot "Wolf"
when BULLET any do SIDESTEP
mode HUNT:
  when HP <= 1 do MOVE away go FLEE
  when STEP_IN any do LEAD
  when ON_LINE any and not ARMED it do FIRE it      # punish whoever just fired
  when ENEMY any >= near do DECOY it go BAIT mark it
  when CHANCE 1/3 do SWEEP ccw                      # unreadable scan pattern
  else do SWEEP cw, SWEEP cw, MOVE toward
mode BAIT:
  when ON_LINE mark do FIRE it                      # they walked into the lure
  when SCANNED any do SIDESTEP go HUNT
  when after 3 do PING mark go HUNT
  else do WAIT                                      # stay dark
mode FLEE:
  when ON_LINE any do FIRE it
  when COIN any do MOVE it
  when after 6 do SWEEP cw go HUNT
  else do MOVE away
```

## 5. Open questions for the owner

1. **State model:** do tier 2 bots get modes (TV-readable, my pick) or L1's 4 timed marks (more flexible, invisible on TV)? Not both.
2. **Implicit feasibility:** do you accept that a FIRE rule is silently skipped while reloading? The phone shows the skip, the TV does not.
3. **The flawed starter:** do you want a designed failure (no dodge), or a decent starter and slower lessons?
4. **Hot-swap visibility vs lookahead:** a patch appears ~3 beats late on TV. Should the director shrink its lookahead when an edit is pending?
5. **Tier 1 cap:** 8 rules, or 12 (L1)? Is the patrol step in or out of the cap?
6. **Card UI in v1:** this editor is the product. Ship it in v1 instead of text-only (VISION open decision 6)?
