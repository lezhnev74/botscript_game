# TINY

## 1. Name

TINY is a numbered assembly listing and a program counter that survives the beat. Tests, jumps and register writes are free. The first world-action ends the beat. The next beat starts on the next line.

`JMP 1` under an action retests from the top next beat, free. Omit it and the next line runs anyway.

## 2. Grammar

The text is the language. Canonical form is one statement a line, single spaces, numbers `1..n`, no gaps, no comments. The parser repairs spacing only. Anything else rejects the paste and the live bot keeps running. 24 lines, `AGAIN` included.

```
bot "Wolf"
1 IF BULLET ANY GOTO 15
2 IF HP <= 1 GOTO 17
3 IF STEP_IN ANY AND LOADED GOTO 19
4 IF ON_LINE ANY AND LOADED GOTO 10
5 IF DIST <= 2 AND NOT LOADED GOTO 17
6 IF A == 2 GOTO 12
7 IF ENEMY ANY >= near GOTO 21
8 SWEEP CW
9 JMP 1
10 FIRE TOWARD
11 JMP 1
12 MOVE BACK
13 SET A 0
14 JMP 1
15 STEP
16 JMP 1
17 MOVE AWAY
18 JMP 1
19 LEAD
20 JMP 1
21 SET D TARGET
22 SET A 2
23 DECOY D
24 AGAIN
```

`bot "Name"` is 1 to 16 letters, digits or spaces. Default name: Core.

One world-action ends the beat: `MOVE dir`, `STEP`, `SWEEP dir`, `PING dir`, `FIRE dir`, `LEAD`, `WAIT`, `DECOY dir`. `STEP` sidesteps. `DECOY` is the 4-cell beacon, twice a life. Spent, it waits, and the badge still says `DECOY`. A dry `FIRE` waits and skips the reload. `LEAD` with no `STEP_IN` waits.

Instants, then steam on the 13th: `IF cond GOTO n`, `IF cond AND cond GOTO n`, `JMP n`, `SET A 2`, `SET A B`, `ADD A -1`, `SET D dir`, `SAY dir`, `AGAIN`. `A B C` clamp to -9..9. `D` defaults to `N`. `AGAIN` is last, means `JMP 1`, and stays.

Conditions: `BULLET HIT HEARD SCANNED ON_LINE STEP_IN ARMED` with a dir or `ANY`. Also `LOADED`, `BONUS >= n` for 1..2, `HP <= n`, `HP >= n`, `BLOCKED dir`, `ALLY NEAR`, `FRIEND dirq`, `RAND < n`, `ENEMY dirq >= faint|near|locked`, `COIN dirq`, `LOCKED`, `DIST <= n`, `DIST >= n`, `ALLY SAID`. `NOT` covers one sensor. `A B C` compare with `== != < <= > >=`. `D == dir`. `RAND < n` rolls 0..3 from the sim rng, n is 1..4, once per condition.

Dirs: `N E S W CW CCW TARGET TOWARD AWAY HIT HEARD COIN SCAN D BACK SAID`. `ANY` is tests only. `CW` and `CCW` turn facing, or `D` inside `SET D`. `BACK` opposes `D`. `SAID` is the ally dir from last beat. Anything unresolved uses facing. Facing starts at `N`, separate from `D`, and updates when an action picks a dir. `STEP` and `WAIT` leave it. `SET D` stores a cardinal.

A `SWEEP` or `PING` replaces the scan record for that beat and the next. `PING` on a bot sets `LOCKED`, `ENEMY` at `locked`, and `DIST`. `SWEEP` sets the fuzzy level and `COIN`, and clears `DIST`. A `DIST` test with no fresh ping is false. Missing is not zero.

`SAY` publishes one cardinal. Allies see it next beat only. Two speakers: the lower bot id wins the sensor.

## 3. The tap editor

One column. A dot marks the counter. Bottom `+` inserts a world-action above `AGAIN`, default `FIRE TOWARD`. Gaps insert in place. Chips open sheets. The listing takes no typing. The name starts as Core.

Eight sensor presets: bullet, on a line, enemy near, heard, scanned, coin, loaded, low hp. One `AND`. `NOT` sits on a condition. No third condition. A goto lists real lines with their opcodes. Delete is swipe, then confirm, and it is disabled on the last world-action. A deleted target retargets its gotos to the next line and flashes them. The counter stores a hidden line id, so a drag renumbers the screen and the dot stays.

Tap a gap, tap `IF`. That inserts `IF BULLET ANY GOTO` a new `STEP` / `JMP 1` pad, and puts `JMP 1` under the action above it when that action would fall into the pad. No line number is typed. Those jumps make the test run every beat.

| Result | Taps |
|---|---|
| Spinner: `SWEEP CW`, then `AGAIN`. Legal, bad, already joined | 0 |
| Lamp. Counter on line 1, registers cleared | 1 |
| Duck, section 5 | 2 |
| That `STEP` becomes `FIRE` | 2 more |
| Moth, section 5, no missed sheet | 39 |

Lamp, the one-tap template, is Duck plus `IF ON_LINE ANY AND LOADED` aimed at `FIRE TOWARD`, then `JMP 1`. Magpie, Possum and Hunter swap that test and the sweep for coins, `WAIT`, or `PING`.

## 4. The beat, and hot-swap

At lock, each bot runs from its counter on the shared snapshot and stops on one world-action. Then the sim does moves, scans, shots, bullets, hits, coins. `SAY` is invisible to other bots until the next lock.

Badge: `#23 DECOY D`, plus `from #7` when an `IF` jumped. The 13th instant is `#8 STEAM` and the counter stays there. `A` and `D` show on a focused bot only.

Quiet Duck is 1 instant, then `SWEEP`. Moth's acquire beat is 6 instants, then `DECOY`, with `A` set to 2. The next beat is 3 instants, then `MOVE BACK`, unless a bullet steps first and leaves `A` at 2.

Finger-up commits the listing for the next lock. The lock already taken finishes on the old text. A chip edit keeps the counter's line id, the registers and the scan record. Facing stays. Delete the dotted line and the dot moves to the next one. Paste, a template, or death resets the counter and clears registers and scan memory. A shielded `FIRE` is cancelled and struck through on the badge.

## 5. Three programmers

Duck is 2 taps: sweep, and step when the bullet test is the line under the dot.

```
bot "Duck"
1 IF BULLET ANY GOTO 4
2 SWEEP CW
3 JMP 1
4 STEP
5 JMP 1
6 AGAIN
```

Moth is 39 taps with no missed chip, about two minutes after they have already seen one goto still land on `STEP`. The first clean try takes longer.

```
bot "Moth"
1 IF BULLET ANY GOTO 12
2 IF A == 2 GOTO 9
3 IF ON_LINE ANY AND LOADED GOTO 7
4 IF ENEMY ANY >= near GOTO 14
5 SWEEP CW
6 JMP 1
7 FIRE TOWARD
8 JMP 1
9 MOVE BACK
10 SET A 0
11 JMP 1
12 STEP
13 JMP 1
14 SET D TARGET
15 SET A 2
16 DECOY D
17 JMP 1
18 AGAIN
```

Wolf, in section 2, is one paste. The prompt pack is this page.

## 6. Tactics

**Bait.** `SET D TARGET`, `SET A 2`, `DECOY D`, then `MOVE BACK` while `A == 2`. A bullet test above `A` steps, then the feint resumes.

**Feint.** `DECOY E` then `SWEEP W`, no register. Beat two ignores news.

**Ambush.** Three `WAIT`s and no sweep fill the dark timer. A test above them still wakes the bot. `ADD B 1` under the wait and `IF B >= 3` above it ends the camp. Possum is `WAIT`, then `FIRE` on `SCANNED` or `HEARD`.

**Lead.** `IF STEP_IN ANY AND LOADED` goes to `LEAD`, above Wolf's plain `FIRE`.

**Kite.** `IF DIST <= 2 AND NOT LOADED` goes to `MOVE AWAY`. A loaded bot fires at point-blank. An empty gun gives ground. `DIST` is false until a `PING`, and the x-ray shows that ping.

**Team.** The pad Wolf cannot afford. Drop the kite lines, `SAY D` on the decoy beat, and the ally runs `IF ALLY SAID GOTO` a `MOVE SAID` pad, then `JMP 1`, under the same sweep loop as Duck. The ally moves one beat late. Registers stay private.

## 7. Weaknesses

Duck's two taps work because the IF button wrote the jump. A blank line and a mnemonic sheet do not yield a bot in seconds. The button does. The jump stays a mystery.

Bottom `+` twice builds sweep, then fire, looping. On the fire beat the bullet test is not the line under the dot, so it sleeps. The dot is the teacher, and it sits on the phone.

24 lines fits bait, lead, kite and a dodge. Team is the cut.

Gotos are numbers. Hidden ids survive a drag and vanish on export, so a round-tripped paste is brittle again. Retarget-on-delete can send the bot into the wrong pad. A bad goto is legal.

A chip edit keeps `A`. Rewrite the feint while `A` is 2 and the bot plays the old plan on the new lines.

Twelve instants covers Wolf and still steams a long test ladder. Nothing on the phone proves steam ahead of time. `BACK` and `DIST` are the extra words bait and kite need. `CCW` is the dir to drop if the sheet crowds.
