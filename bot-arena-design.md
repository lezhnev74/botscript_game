# Bot Arena: Game Design Summary

A real-time office game where players don't control their avatar directly. Each player writes a small **program** for a bot, using simple rule cards, and watches it play on a shared big screen.

---

## 1. Core concept

- A 2D **grid arena**, shown on a big screen.
- Each player owns one bot and edits its program from their **phone**.
- Bots act on their own, following their program.
- Goal: score the most points by collecting coins and eliminating enemies.
- Audience: non-technical office colleagues. Think "school turtle programming", not real code.

## 2. Programming model: priority rules

A program is a short, ordered list of **IF condition → THEN action** rules.

- Every tick, the bot checks the rules from top to bottom and performs the **first one that matches**.
- Players build rules by tapping cards and reorder them by dragging.
- **Order is the strategy.** Swapping two rules can turn a collector into a hunter.

Why this model:
- Step-by-step sequences ("forward, forward, left") break in real time, because the world changes while they run. Rules react to the world automatically.
- Anyone can read five rules. No variables, loops or syntax.
- Mistakes are visible and funny, and everyone learns by watching.

Inspirations: *Gladiabots*, the *Final Fantasy XII* Gambits system. RoboRally could make a turn-based warm-up mode.

## 3. Time budget

- One tick = **250 ms**, so a bot gets **4 actions per second**.
- Every action costs 1 tick.
- Scanning all four directions takes a full second, so information has a real cost.

## 4. Directions

- Only **4 directions**: north, east, south, west.
- The grid and 4 directions create **lines**: bullets travel along rows and columns.
- The game is about **lining up** with an enemy, or **stepping off their line**.

## 5. The three actions

### Scan: a wedge with a probability signal
- Scanning in one direction covers a **wedge**: a triangle widening outward, a quarter of the arena.
- It returns three signal strengths, for example:

  ```
  SCAN NORTH →  enemy 72%   coin 20%   incoming bullet 0%
  ```

- The signal is stronger when an object is **closer** and **nearer the wedge's center line**.
- This works like hot-and-cold: step sideways, scan again, and when the signal peaks, you are lined up. That's the moment to fire.
- Detecting **incoming bullets** lets bots dodge before impact.

### Fire: into the wild
- The bullet travels in a straight line along the row or column.
- The **first object in its path** takes the damage, **teammates included**. If nothing is hit, it disappears at the arena edge.
- Bullets **travel over time** (a few cells per tick) rather than hitting instantly:
  - close shots almost always land,
  - long shots can be dodged.
- Bullets pass **through coins**, so loot can't be destroyed.

### Move
- One cell in one of the 4 directions.
- Used to **chase** (enemies, coins) or to **dodge**.
- Because bullets fly along lines, a **sidestep** (perpendicular to the threat) always clears you.

## 6. Information and mind games

- **Getting hit reveals the shooter's direction**, but not the distance. Firing gives away your position.
- Bots remember the **last scan result** and **where they were last hit from**.
- Each scan gives only a short, fuzzy, partial picture, so players must decide where to look.

## 7. Rule cards

**Conditions**
- `IF enemy signal [direction] > X%`
- `IF coin signal [direction] > X%`
- `IF bullet signal [any / direction] > X%`
- `IF I was hit` (remembers the direction)
- `IF last scan found enemy / coin`
- `OTHERWISE`

Thresholds are set with a **slider**.

**Actions**
- `SCAN` north / east / south / west / next direction clockwise / toward where I was hit
- `FIRE` in a direction / toward the last strong signal / toward where I was hit
- `MOVE` in a direction / toward the strongest signal / sidestep away from a threat

## 8. Example programs

**Sniper**
```
1. IF bullet signal any direction > 50%  → STEP sideways from it
2. IF enemy signal NORTH > 80%           → FIRE NORTH
3. IF enemy signal NORTH > 30%           → STEP east/west toward stronger side
4. OTHERWISE                             → SCAN NORTH
```

**Hunter**
```
1. IF I was hit from X               → FIRE toward X
2. IF enemy signal (last scan) > 80% → FIRE that direction
3. IF enemy signal (last scan) > 40% → MOVE that direction
4. OTHERWISE                         → SCAN next direction clockwise
```

**Collector**
```
1. IF bullet signal any direction > 40% → STEP sideways
2. IF coin signal (last scan) > 50%     → MOVE that direction
3. OTHERWISE                            → SCAN next direction clockwise
```

These archetypes form a rock-paper-scissors loop, so no single program dominates.

## 9. Scoring

| Event | Points |
|---|---|
| Collect a coin | +1 |
| Eliminate an enemy | +3 |
| Get eliminated | −2 (respawn after ~3 s at a random edge) |

## 10. Teams and cooperation

- Play in **2–4 teams**: cooperative inside a team, competitive between teams.
- Friendly fire makes **positioning a team conversation**.
- Teams naturally split into roles: scout, collector, bodyguard.
- Possible cooperation cards: `IF ally is hit → MOVE toward ally`, `FOLLOW ally`, shared team scans.

## 11. Program budget and progression

- Everyone starts with **4 rule slots**.
- Coins can unlock **extra slots** or **advanced cards** (for example, "lead the target"), so collecting has a strategic payoff beyond points.

## 12. Session format

- **Big screen** shows the arena. **Phones** are the rule editors.
- **Rounds of 3 minutes**, with **60 seconds of editing** between rounds.
  - Alternative: live editing during play, with a 3-second "rebooting" freeze as the cost.
- **5 rounds per game.** Round 1 is chaos. By round 3, people are counter-programming each other.

## 13. Spectator appeal

- Draw scan wedges as faint **flashlight sweeps** and bullets as visible **tracers**.
- Near misses and last-second sidesteps are what make the room shout.

## 14. Keeping it fresh

- **Events:** coin showers in one quadrant, golden coins worth 5, a shrinking arena in the last 30 seconds.
- Optional **obstacles or walls** for cover.

## 15. Open questions

- **Honest vs noisy sensors.** Start honest: the percentage is exactly the signal. Later, add an "advanced arena" with random noise for more tension.
- **Tuning by feel:** wedge falloff, bullet speed, arena size and respawn time. Best settled with a playable prototype.

## 16. Next step

Build a one-screen prototype with the grid arena, the card-based rule editor, and the three house bots fighting each other, then tune the numbers before inviting the office.
