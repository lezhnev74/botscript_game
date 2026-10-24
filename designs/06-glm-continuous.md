# 06 - BEARING: continuous-space bot duels

## 1. Pitch

Robocode for the office TV: each player submits a ~10-line pilot program from their phone; their tank has a body, a turret and a radar that turn independently, and fights read as dogfights, not chess.

Why it fits: slow speeds plus trails make motion legible across a room; the three-part tank makes intent visible (body points where you flee, turret where you threaten, radar where you are blind); a 4-verb API is learnable at the coffee machine, yet lead-the-target aiming and orbit movement give real depth - exactly the depth LLMs are known to write well.

## 2. Continuous vs grid - the honest argument

Where the grid wins: per-frame legibility. Cells snap, "same row" is a visible fact, 4-direction cards need no math, and implementations are trivially deterministic. Continuous space risks mush: two dots drifting is hard to parse at 3 meters.

Where continuous wins: narrative legibility and skill ceiling.

- Intent becomes geometry. Trails draw the feint, the orbit, the ambush arc. Spectators stop tracking positions and start reading shapes.
- Aim is a skill, not a lottery. On a grid, sharing a line is a discrete accident. Here, leading a mover, strafing circles and radar-shadow ambushes are continuous skills with unlimited refinement - and Robocode's 20-year public corpus (virtual guns, guess-factor targeting, anti-gravity movement) means LLMs reproduce strong versions instantly. The AI-authored ceiling is far higher, which the brief explicitly wants.

Verdict: the grid is easier to follow in round 1; continuous is more watchable from round 3, when viewers watch shapes, not cells. We earn it with: slow speeds (arena crossing ~14 s), big sprites, 2 s fading trails, a cosmetic background reference grid (spatial anchors, no gameplay effect), and predicted-path dots. Fallback if prototypes look mushy: snap rendering to a coarse overlay grid while physics stay continuous.

## 3. World model

- Arena 1000 x 625 units (16:10), continuous coordinates. Sim: fixed 10 ticks/s; render 60 fps with interpolation.
- Tank = body (heading, throttle), turret (own heading), radar (own heading, 50 deg wedge). Turn rates: body 90 deg/s, turret 180 deg/s, radar 300 deg/s (full sweep 1.2 s).
- Max speed 70 u/s (long crossing 14 s, a dodge ~1 s). Typical engagement range 150-350 u.
- Obstacles: 6 round pillars + 2 low walls. They block movement and bullets and cast radar shadows (ambush space). Kept few for readability.
- Bullets: 250 u/s, tracers ~40 u, max flight 2.2 s (range ~550 u).
- HP 100; respawn 4 s at arena edge.
- On TV: colored tanks with distinct silhouettes, faint radar wedge, gun barrel with a 30 u aim tick, 2 s motion trails, bullet tracers, hit flashes, HP as 4 chunky pips.

## 4. Program model

One language, two frontends:

- Cards (phone): priority IF->THEN rules like the baseline, with bearings instead of compass directions ("strongest blip", "hit-from side"). Compiles to the same program as text.
- Pilot text DSL (~10-20 lines): event handlers plus per-tick rules. Programmers and AIs write this directly.

Sensors (complete list):

- `blips`: results of the last radar sweep - kind (bot / cell / bullet / wall), bearing, distance; bots also report heading (8 dirs) and speed (10 u/s steps).
- Events: `hit_from(bearing)`, `i_hit(target)`, `bump(kind, bearing)`, `spawned`.
- Self: `x, y, heading, speed, hp, gun_heat, clock, score`; 32 numeric variables (persist across ticks); blackboard of the last 5 sweeps.

Actions (4 verbs, intents that persist until changed):

- `drive(heading, throttle)` - body turns toward heading, then moves.
- `aim(bearing | blip)` - turret.
- `radar(sweep | hold | track)` - radar mode.
- `fire()` - if gun heat is 0, shoots along current turret heading.

Limits: 500 instructions per tick, 2 ms wall-clock cap, no file/network/clock access, one seeded RNG shared by the arena. Same code + same seed = identical match.

Example 1 - Brawler (card form):

```
1. IF hit from side B          -> drive B+90 full throttle
2. IF blip bot closer than 250 -> aim blip (lead 0.4 s); FIRE
3. OTHERWISE                   -> radar sweep; drive toward nearest cell
```

Example 2 - Ambusher: `IF nothing seen -> park behind nearest pillar, radar hold; IF blip bot < 200 -> aim + fire, drive tangent; IF hp < 40 -> flee edge to edge`. Still bots draw no trails, so hiding is emergent, not a rule.

Example 3 - Intercept gunner (the kind an LLM writes):

```
var ex ey evx evy = 0
on tick:
  b = latest_bot_blip()
  if b: ex ey = b.x b.y; evx evy = b.vx b.vy
  if seen(ex):
    t = dist(self, ex ey) / 250          # bullet flight time
    aim atan2(ey + evy*t, ex + evx*t)    # intercept point
    if gun_error < 4 and gun_heat == 0: fire()
    drive tangent(ex, ey) at 60          # orbit
  else:
    radar sweep; drive toward last_seen
on hit_from b: drive b+90 at 100; after 8 ticks: radar sweep
```

## 5. Combat & scoring

- Bullet 250 u/s, damage 26 (4 hits to kill), reload 1.5 s shown as a gun pip. Friendly fire on.
- Cells (pickups): +1 point and +15 HP; 6 on the field, respawn 20 s - bait material.
- Kill +3, pickup +1, death -2.
- Stalemate tax: if no damage lands arena-wide for 30 s, everyone loses 1 point per 10 s.
- Last 45 s: a shrinking "storm" ring deals 5 HP/s outside it - forces the endgame.
- Round = 3 min. Match = 5 rounds, cumulative score.

## 6. Session format

- Big screen shows the arena; phones edit via QR + 4-letter join code. Submit up to 10 s before round start.
- Rhythm: 3 min round, 90 s edit window (continuous code needs a bit more than cards). One match ~22 min; a single 4 min round works as a coffee-break variant.
- Newcomers fork a house bot (Brawler / Ambusher / Runner provided) and tweak one line in round 1.
- Teams: 2v2 or 3v3. Ally blips are always visible (no radar needed) and teams share a sweep blackboard, so cooperation is cheap: crossing fire, bait-and-pivot, one scout feeding two gunners. Card: `IF ally hit from B -> FIRE toward B`.
- Weekly ladder: Elo per player-bot; champion stays, challenger's code stays hidden until the match ends, then both are revealed on screen - a spectator moment and the meta engine.

## 7. Spectator layer

- Fixed full-arena camera, no scrolling (one-screen rule). Kills trigger a corner instant replay at half speed (3 s), then 0.5 s of 0.3x slow-mo on the live feed.
- Overlays: name + 4 HP pips; predicted-path dot 1.5 s ahead of any tank holding a steady heading; radar wedges as faint arcs; a gun-lock flash when a turret aligns, so the room braces before the shot.
- Sound: per-bot radar ping pitch, reload click, hit thunk - the room learns to hear the game.
- Between rounds: 10 s "top play" replay with trails burned in; after the final round, the code reveal.

## 8. AI-written programs

- Allowed and encouraged: the DSL spec plus 5 sample bots is a complete LLM prompt; the phone editor has a paste box. Depth is the point - intercept math, feints, cooperative kiting are all writable.
- Fairness: the interpreter has no capabilities beyond sense/act, an instruction budget, and deterministic seeded replay, so disputes are checkable and nothing can touch the host.
- Anti-degenerate: opponents' code hidden until match end (copying only after reveals, which is a healthy meta); budget kills gimmick exploits; arena-side counters below.

## 9. Risks / degenerate strategies

- Readability mush -> countered in section 2; validate on a real TV at 3 m before anything else.
- Perpetual dodger (no one ever lands): dodging at range is intended, but bullets at 250 u/s versus bots at 70 u/s punish it up close; storm + stalemate tax + cell bait force engagement. If still stale: add 1 deg seeded shot spread.
- Radar-spin camper: campers can't score; storm and cell spawns are pulled toward the center.
- Corner hugging: pillars placed away from corners; storm ring.
- Orbit-lock stalemate (two bots circling forever): 1.5 s reload means orbits still convert to hits; stalemate tax otherwise.
- Runaway leader: leader must keep picking cells to stay ahead; storm caps running in round endgame.
- AI code bombs / host escapes: sandboxed VM, op budget, no host APIs.
- Skill gap crushing novices: house-bot forking plus card frontend keeps non-programmers playing the same game.

## 10. Implementation sketch

- TypeScript monorepo. `sim`: deterministic core, fixed-point math, seeded RNG, 10 Hz; a match replay = code versions + seed. `render`: Canvas 2D at 60 fps with interpolation, trails as fading polylines. `server`: WebSocket broadcasting snapshots; phones are thin web pages.
- Pilot DSL parses to an AST interpreted by a capability-free VM (500 ops/tick, 32 slots, sensor/action calls only). Cards compile to the same AST.
- Match log JSON feeds replays and the ladder site.
- First milestone: arena + 3 house bots + trails + kill-cam PiP; tune speed, bullet, storm and radar rates on a real TV before inviting the office.
