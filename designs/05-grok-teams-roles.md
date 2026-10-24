# Squad radio

## 1. Pitch

Teams of 2-4 each program one bot on one office TV. Chassis is scout, tank, sniper, or medic, locked at draft. Allies share cell and HP. Enemy cells stay hidden until a scan or a radio report. A report spends the action: one cell and an integer 0-7, once a second, meaning assigned by the team. Modes are capture-the-flag, escort, or bomb.

Rounds are three minutes. Eight cards can walk a flag home or fire on a ping. AI is stuck with that same packet. Ship flag first.

## 2. World model

Grid 24 by 16. Facings north, east, south, west. Tick 250 ms.

Tick order: snapshot, all programs read it, then moves, shoves, pickups, plants, repairs, radios, new shots, then old bullets. Double-entry into one cell bounces both.

The camera is the full grid. Color is team, silhouette is chassis, flag and bomb are two cells tall, hurt-only HP bars are thick, radio digits are one cell tall.

Free knowledge: own pose and HP, ally cells and HP, objective, ally packets, captain tap. Enemy cells, enemy HP, and source text stay hidden.

## 3. Program model

Up to 8 ordered rules, first match, one action. One test, or two with AND. Chassis-illegal cards stay hidden.

Squad is the paste format: cells, distance, alignment, 40 ops, no loops or extra input. First true branch wins and is the only action. Over cap, reject on the line number.

Memory: last scan, hit facing for 6 ticks, muzzle facing for 4, four registers of 0-7, one ally packet dead at 12 ticks, captain cell and age. Registers outlive a packet. Sensors are exact.

**Scan** spends the tick. Wedge is 90 degrees. Percent is `100 - distance * 6 - off_center_cells * 10`, clamped 0-100. Range: scout 14, medic 8, tank 5. Sniper scan is the facing line only: first bot or cover, exact distance. A scan clears sniper aim.

Passive each snapshot:

- closing bullet on your line, percent `100 - distance * 8`
- `aimed at` from a facing if an enemy sniper has been still 2 ticks with clear line

**Radio** spends the tick, once per 4 ticks. Cell is mine, the objective, the captain tap, or last scan's strongest contact, plus code 0-7. Codes have no built-in meaning.

**Captain tap.** One phone per team may send one cell per 8 seconds. No code. Programs may ignore it, and it is the only phone input during a round.

Conditions: signal above 25, 50, or 75 for enemy, bullet, or objective, one facing or any. Ally, cover, or objective on the fire line. Carrying. Own HP at or below 3. Named ally HP at or below 3 or 5. Hit, muzzle, `aimed at`. Ally code N with age under 8. Ping within 3, or aligned. Radio ready or cooling. Captain age under 8. Register N. Still 2 ticks. Otherwise.

Actions: move facing, toward ally, ping, captain, or objective, or off a line. Scan facing, clockwise, or toward ping or objective. Fire facing, last strong scan, or ping. Radio. Set register. Wait. Shove, which pushes the occupant one more cell if empty. Repair the lowest ally in range. Place cover. Anything illegal stumbles.

Tank, cards. Code 2 is the enemy cell from the scout.

```
1. IF aimed at                         → move off that line
2. IF bullet any > 50                  → move off that line
3. IF medic HP <= 3                    → move toward medic
4. IF scout code == 2 AND age < 8      → move toward scout ping
5. IF I carry the flag                 → move toward own base
6. OTHERWISE                           → move toward the flag
```

Scout, cards. Code 7 means hit. While radio is cooling, a strong contact gets the pistol.

```
1. IF aimed at                         → move off that line
2. IF I was hit                        → radio my cell, code 7
3. IF enemy any > 50 AND radio ready   → radio that cell, code 2
4. IF enemy any > 75                   → fire toward it
5. IF sniper code == 1 AND age < 8     → move off sniper fire line
6. IF I carry the flag                 → move toward own base
7. OTHERWISE                           → scan clockwise
```

Sniper, Squad. An AI writes this from the scout's codes. Scout code 2 is a cell to shoot. Tank code 1 means hold. A mismatched 2 shoots a corner.

```
if aimed_at:
    move off_line(aimed_dir)
if hp <= 3 and radio_ready: radio my_cell 7
if hp <= 3: move dir_to(base)
if ally.scout.code == 2 and age(ally.scout) < 8:
    if aligned(ally.scout.cell) and still >= 2 and not ally_on_fire_line:
        fire dir_to(ally.scout.cell)
    else:
        move strafe_to_align(ally.scout.cell)
if ally.tank.code == 1 and age(ally.tank) < 8:
    wait
scan dir_to(objective)
```

## 4. Combat and scoring

| Chassis | HP | Move | Weapon | Scan |
| Scout | 3 | 1 / tick | 1 dmg, range 8, speed 4, reload 3 | wedge 14 |
| Tank | 9 | 1 / 2 ticks | 2 dmg, next 2 cells, instant, reload 4, first target | wedge 5 |
| Sniper | 3 | 1 / 2 ticks | 4 dmg, whole line, speed 4, reload 8 | exact line |
| Medic | 5 | 1 / tick | 1 dmg, range 6, speed 4, reload 6 | wedge 8 |

Sniper fire before 2 still ticks stumbles. At 2 ticks, `aimed at` is visible, movement happens, then a 4-damage round spawns. Leaving the line saves you and abandons the plant, cart, or flag. Staying kills a scout or sniper, and kills a tank on the third hit. Other new shots travel 4 cells after movement: range 4 hits, range 8 leaves one dodge tick.

Repair: range 3, clear line, +2 HP per 4 ticks, no overheal. Cover: adjacent, 5 HP, blocks move and shots, two per medic. Friendly fire is on. Any hit drops a flag. Hearing is 8 cells, and a sniper shot is a map-wide facing. Respawn is 8 seconds at base, 1 second safe, pickup after 2. Score is the objective only.

**Capture the flag**, default, 3:00. Touch to take. Carrier rate halves, so the tank steps once per 4 ticks. Score 1 at your own base. First to 2, else the lead. Tie: fewer deaths, then one cap with respawn off. After 45 seconds an untouched flag drifts toward center at 1 cell per 4 ticks.

**Escort.** A 16-cell track, 3 wide, two corners. In the cart's 3x3, more attackers than defenders advances 1 cell per 2 ticks. Defenders alone reverse 1 per 4. Score is cells, then sides swap.

**Bomb.** Pickup on contact. Plant 8 ticks, defuse 12. Leaving pauses. A hit flinches 2 ticks and keeps the bar. Fuse 20 seconds. Round 2:30, best of three, sides swap. Explosion wins for attack. Defuse, or no plant, wins for defense.

## 5. Spectator layer

Live speed stays 250 ms. A corner replays 3 seconds at half speed after a drop, plant, defuse, or tank death. Flares show the digit for 3 seconds. Enemy bots learn nothing from them. Eligible snipers draw their line, with allies outlined. The ticker is events, including unmatched codes. Phones highlight the fired rule, and the TV does if that player pins it.

## 6. Session format

QR join, snake draft, each chassis once per team, one mode for the session. Learn on 2v2. An extra person is captain and may co-edit one bot between rounds. Dummies for 45 seconds, then phones lock. Edit window 90 seconds, programs public on the TV for the first 20, one rule allowed to read "???". First to two rounds, bases swap. The ladder stores the result and chassis, and at most two of the top half draft together.

## 7. AI programs

Paste is allowed, stored on the ladder, shown next window minus the hidden rule. Caps: 8 rules or 40 ops, one cell and 3 bits a second, no enemy source, no network, registers cleared each round. Three shared-model generations per window from chassis, mode, and teammate programs. Personal models are unlimited. Over budget rejects. The codes still need a teammate who will not read the prompt.

## 8. Risks

**Heal pile.** The medic has to stand on the 4-damage line. If a 9 HP tank still will not die, repair drops to +1.

**Fat packets.** Coordinates fit only in the cell. Codes stay 0-7.

**Captain.** 8 seconds, cell only, name on the flare, bots may ignore it.

**Base camp.** Drift at 45 seconds, and any hit drops the flag.

**Bad codes.** The ticker names codes nobody reads, with 90 seconds to fix them.

**Clutter and team kills.** Drop captain flares before bot flares. The aim line and the program are both on the TV, friendly fire included.

**Fuzzy math.** Keep percents exact so the wedge on screen is the number in the rule.

## 9. Implementation sketch

Node, 250 ms tick, websocket clients, TV browser on the display machine. Clients draw an integer sim. Replay is seed, sources, and taps. Cards are JSON. Squad becomes at most 40 opcodes, one sensor struct in and one action out, no `eval`, overrun is Wait. Late uploads drop. SQLite keeps the ladder. First build is headless: three squads, one flag match, a log of caps and expired pings. Tune reload there.
