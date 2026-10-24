# Same beat

## 1. Pitch

Every bot locks one action on the same beat, blind to the other locks. A beat is 800 ms on one TV. Move, dash, fire, shield, scan, and charge spend one bar. A step costs the same 1 you just gained, so constant walking never funds a shot. Charge pays +2 and glows the whole beat, which is both the fast reload and the tell. A round is 3 minutes, 225 beats. The program is eight IF-THEN rules.

## 2. World model

Grid 18 by 10, four directions, four mirrored cover blocks, seed in the corner. Cover stops steps, shots, and lines. Bots and the TV share cells, facing, HP, energy, last badge, bullets, and coins. Rule text and registers open when the round ends. Locks stay hidden until the beat resolves. Programs see the previous beat plus this beat's passive energy.

Up to 12 bots. HP 3. Energy is an integer, cap 6. Three coins. Pickup is +1 score and +1 energy, then the coin is gone for 8 beats and returns at least 4 cells from any bot.

The server steps on its own 800 ms clock. The page tweens. Phases, all bots together:

1. Passive +1 energy, cap 6. Programs run. One lock each.
2. Pay. A lock you cannot afford becomes WAIT. You keep the passive point.
3. CHARGE adds +2, cap 6.
4. MOVE and DASH. A shared end cell bounces both bots home. No refund.
5. Shields arm. The bubble eats one hit this beat, then pops.
6. New shots, then every bullet moves 5 cells. First bot on the post-move segment takes it. Cover and the edge stop the shot. Max travel 15. Friendly fire is on.
7. Coins, damage, deaths.
8. Cones write next beat's scan sensors.

Hits use post-move cells. CHARGE, FIRE, SHIELD, and LEAD are rooted. A blind sidestep still beats a shot.

## 3. Program model

Phone cards and pasted text are one list. Eight rules, first match, OTHERWISE last. Conditions join with AND and NOT. A matched rule may SET up to three registers, then take one action. SETs happen before payment. A B C D store 0-9 or a direction.

Sensors, all legal in a condition:

- ENERGY >= n, HP <= n. Energy is post-passive, pre-spend.
- ON_LINE dir, dist <= n. Clear line to an enemy.
- GLOWED dir. Their last badge was CHARGE.
- LAST dir = a badge name.
- SCANNED dir. A cone covered me last beat.
- STEP_IN dir. Last move would land them on my line there. They are off it now.
- BULLET dir, dist <= n. Incoming.
- HIT dir. I took damage from there last beat.
- FRIEND dir. An ally is the first body on the line.
- COIN dir, dist <= n.
- RAND < n. One draw per bot per beat, 0-99, always taken.
- Register tests.

Directions: N E S W, CW, TOWARD, AWAY, SIDESTEP, COIN, HIT, D. SIDESTEP leaves the nearest threat line toward open cells.

| Action | Cost | Effect |
|---|---|---|
| WAIT | 0 | Hold. |
| MOVE | 1 | One cell. |
| SCAN dir | 1 | Paint a quarter cone this beat. |
| DASH | 2 | Two cells, stop at cover. |
| SHIELD | 2 | Rooted. One hit stopped. |
| FIRE dir | 3 | Rooted. 1 damage, 5 cells this beat. |
| LEAD | 3 | FIRE toward a STEP_IN cell. If none exists, this lock becomes WAIT at cost 0. |
| CHARGE | 0 | Rooted, glow, +2 on top of the passive. |

Follow a SCAN with FIRE or LEAD on that facing and the shot costs 2. Any other follow-up skips the next +1. From 0, one CHARGE is fire-ready next beat. Three WAITs match it with no glow. From 4, CHARGE ends at 6, FIRE leaves 3, the next passive makes 4, and a second FIRE leaves 1: glow, two flashes, empty bar.

**Lamp**, the default.

```
1. BULLET any dist <= 5    -> MOVE SIDESTEP
2. ON_LINE any dist <= 6
   AND ENERGY >= 3         -> FIRE TOWARD
3. ENERGY <= 2             -> CHARGE
4. OTHERWISE               -> WAIT
```

**Bait.** Stands on a line when the bubble is affordable.

```
1. BULLET any dist <= 5          -> MOVE SIDESTEP
2. FRIEND TOWARD                 -> MOVE SIDESTEP
3. ON_LINE any AND ENERGY >= 2
   AND HP >= 2                   -> SHIELD
4. HIT any AND ENERGY >= 3       -> FIRE HIT
5. OTHERWISE                     -> WAIT
```

**Ledger**, a typical model paste: shoot the glow, lead a step-in, and sometimes leave the line.

```
1. BULLET any dist <= 6                 -> MOVE SIDESTEP
2. FRIEND TOWARD                        -> WAIT
3. ON_LINE any AND GLOWED TOWARD
   AND ENERGY >= 3                      -> FIRE TOWARD
4. STEP_IN any AND ENERGY >= 3
   AND RAND < 50                        -> LEAD
5. ON_LINE any AND ENERGY >= 3
   AND RAND < 30                        -> MOVE SIDESTEP
6. ENERGY <= 2 AND NOT ON_LINE any      -> CHARGE
7. COIN any dist <= 6                   -> MOVE COIN
8. OTHERWISE                            -> SCAN CW
```

## 4. Combat and scoring

Damage is 1. At 0 HP, respawn after 4 beats on a random edge, at least 6 from the nearest enemy, with HP 3, energy 2, and one beat of spawn shield. Mutual kills on one beat both pay.

Coin +1. First kill on that victim this round +5, second +3, later kills +1. Victim score -1, floor 0. Friendly fire pays 0. Team score is the sum.

Match tie-break: score, then fewer FIRE locks, then more coins.

Shield, wait, shield is the block ceiling: cost 2, income 1, rooted while up. The last 24 beats, the outer ring deals 1 a beat. If playtests show no dodges, drop bullet speed from 5 to 4. Keep fire at 3, charge at +2, beat at 800 ms.

## 5. Spectator layer

Whole arena. Name, 3 HP ticks, 6 pips, pips at 3 and above drawn brighter.

Badge from 0 to 150 ms, then step, charge ring, cone, and tracer. Impacts in the last 100 ms. A row with both a bullet and a live bot gets a dim wash. A miss is a tracer through a cell badged MOVE.

Deaths 10 s apart replay in a corner at half speed. Live play continues. The ticker speaks on a paid beat only: glow on a line, LEAD into a step, shot on a shield. Between rounds, 20 s on the three busiest beats.

## 6. Session format

Join on the phone. Lamp is loaded. Five rounds: 60 s of edits on the frozen frame, then 3 minutes. Late pastes wait. Texts go public at the round end and can be forked.

Teams of 2 to 4. Under 5 players, free-for-all. A missing chair in round 1 is a house bot on Lamp, Bait, or Ledger.

Weekly ladder, one saved program, Elo from placement, top three texts left posted. Talk during the edit minute. Live beats have no extra chat. FRIEND is already drawn on the lane.

## 7. AI-written programs

The paste format is the API. The join screen states the costs and the lock rule in one line: you commit before their move, charge glows, fire is 3, income is 1.

Same eight rules and the same RAND for every author. Enemy source and the next beat are unavailable. A loop or a second action fails at a line number.

Text stays private during the round and opens after. The TV prints badges only. Eight rules cannot fund a shot every beat. A one-line ask, "stay off lines, shoot glows, take coins", comes back as cards. Reordering them is the human part.

## 8. Risks

Text opens between rounds so the edit minute has a list to answer. A sweep can be sealed until round 5. RAND and STEP_IN keep a leaked text from playing itself.

CHARGE at 4 or more sets up the double-tap. CHARGE then DASH lies about the next lock. The glow beat itself is rooted. Firing every GLOWED donates 3. Ignoring glow eats both shots.

A scan you abandon skips the next +1. Repeat kills pay +1, respawn sits far with a shield beat, and short cover makes a bounce a wasted spend. The death inset replays the seed, the texts, and the draws.

## 9. Implementation sketch

Integer TypeScript. `step(state, locks)` returns the next state. One Mulberry32 draw per bot per beat, before rules. The TV is a canvas. It receives beat, state, and locks, and tweens. Phones POST rules during the edit window and show the live hash.

The importer is the sandbox: unknown sensors, a 9th rule, a 4th SET, or two actions fail the paste. No user JavaScript. Run Lamp, Bait, and Ledger headless for 50 rounds before drawing a badge. The bar has to read from the back of the room.
