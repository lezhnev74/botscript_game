# 08 - SIGNAL: one language, three layers, one VM

## 1. Pitch
**Bots hunt in the dark by sound and flashlight; you program their brain in cards, text or script - and the TV shows exactly which line of that brain is firing.**
Fits the brief: one tiny sensor/action set keeps it learnable; fog + noise make feints, baits and ambushes *mechanically real*; every layer compiles to the same budgeted VM, so an AI-written script has no powers a card player lacks - only better composition.

## 2. World model
- **Arena**: 21x15 grid, a few wall blocks (public map), 4 directions only (keeps "lining up" readable from across the room).
- **Time**: 1 tick = 250 ms, 1 action per bot per tick. Render interpolates; key moments slow to 0.25x.
- **Round**: 2 min (480 ticks). Last 30 s the edges become lava, 1 ring per 8 s.
- **Fog**: bots know the map and themselves only. Enemies, coins, bullets are learned via sensors. The TV shows god view; a "POV" inset shows what the focused bot believes (the dramatic irony: room sees the ambush, victim doesn't).
- **Noise model** (the core of information asymmetry):

| Event | Noise | Who senses it |
|---|---|---|
| MOVE | 1 (radius 4) | `heard` of bots in radius, quadrant only |
| FIRE | 3 (whole arena) | everyone: quadrant + band near/far |
| SCAN | - | target(s) inside wedge get `pinged_from` |
| DECOY beacon | 1 per tick, 6 ticks | like footsteps |
| Stall (over budget) | 2 | radius 8 |

- **Stillness**: a bot that took no MOVE/FIRE for 3+ ticks is "dark" - its scan signal to others is halved. Ambushes exist; a 12-tick heartbeat (noise 1) stops pure camping.

## 3. Program model

### 3.1 The VM (single source of truth)
- Integer-only stack VM, ~30 opcodes, deterministic. Each tick: run from current state's entry until an `ACT` opcode or budget exhaustion.
- **Budget: 300 ops/tick**, no banking. Over budget = stall (WAIT + noise 2 + sparks on TV: "overthinking").
- **Memory**: 4 registers `a b c d`, 4 auto-decrementing timers `t0..t3`, 64-word array `mem`, stack 32. Range -9999..9999, saturating.
- **Size cap**: 2048 bytecode words. A card is ~10 words, so cards never feel it.
- **Seeded `rand(n)`** per bot per round - fair randomness, replayable.

### 3.2 Sensors (identical at every layer; reads cost 1 op)
| Sensor | Meaning |
|---|---|
| `scan.enemy / ally / coin / bullet` | 0-100 signal from last SCAN wedge (closer + more centered = higher) |
| `scan.dir`, `scan.age` | direction of last scan, ticks since |
| `scan.drift` | -1/0/+1: enemy signal moved left/none/right vs previous scan in same dir - the hook for *prediction* |
| `scan.wall` | cells to first wall in scan dir |
| `heard.N/E/S/W` | loudness heard last tick per quadrant; `heard.far` flag for gunshots |
| `pinged_from` | dir of a scanner who lit me (or none) - counter-scan / "they see me" |
| `hit_from`, `hit_age` | where my last damage came from |
| `hp`, `x`, `y`, `can_fire`, `decoys` | self |
| `radio` | last int from a teammate (team-only) |
| `tick`, `time_left` | clock |

### 3.3 Actions (exactly one per tick)
`MOVE d` - `SCAN d` - `FIRE d` - `WAIT` (keeps stillness) - `DECOY d` (throw footstep beacon 4 cells, 2 per life).
Helpers that resolve to a direction: `toward(x)`, `away(x)`, `side(x)` (perpendicular, picks open side), `cw(d)`.
Free side effect: `SAY n` (radio, 2 ops, 1 per tick).

Five actions buy every target tactic: **feint** = DECOY one way, MOVE the other; **bait** = fire to make noise, then go dark beside your own echo; **kiting** = FIRE then `side()`; **ambush** = WAIT in dark, SCAN only when `heard` spikes; **prediction** = `scan.drift` + bullet travel time (2 cells/tick) -> shoot the cell they'll be in.

### 3.4 Layer 0 - cards (phone)
Priority rules, top match wins (the baseline model), plus two upgrades:
- **Tabs = states.** Each tab is a rule list; an action card can end with "-> go to tab". A non-programmer still builds a 2-state bot (Hunt / Flee).
- **"for N ticks" chip** on any rule (compiles to a timer).
Thresholds are sliders; conditions/actions are the sensor/action tables above as icons.

### 3.5 Layer 1 - text (same thing, readable)
Every card is literally one line; the phone has a `</>` flip. Layer 1 adds registers, arithmetic, `enter:` hooks. Anything without `mem`/loops flips back to cards.

```
bot Sniper                         # all Layer-0 expressible
state watch:
  when scan.bullet > 50  -> move side(scan.dir)
  when scan.enemy  > 80  -> fire scan.dir
  when scan.enemy  > 30  -> move side(scan.dir)
  else                   -> scan N
```

```
bot Ghost                          # feint + ambush, uses states/timer
state lurk:
  when hit_from              -> goto flee
  when pinged_from           -> decoy away(pinged_from); goto flee
  when heard.any > 0         -> scan loudest(heard)
  when scan.enemy > 70 and can_fire -> fire scan.dir
  else                       -> wait
state flee:
  enter: t0 = 6
  when t0 == 0               -> goto lurk
  else                       -> move away(hit_from)
```

### 3.6 Layer 2 - script (escape hatch)
Python-flavored subset compiled to the same bytecode: ints, bools, fixed arrays, `if/while/for range`, functions (no recursion > 8, no floats, no imports). `@state` functions keep TV highlighting; `think("text")` sets the bubble.

```python
# Typical AI-written bot: heat-map tracker with lead fire
@state
def hunt():
    decay_map(mem)                         # 64 cells = coarse 8x8 belief map
    for q in DIRS:
        if heard[q] > 0: bump_quadrant(mem, q, heard[q])
    if scan.age == 0 and scan.enemy > 60:
        if can_fire and scan.enemy > 85 and scan.drift == 0:
            think("dead on - fire"); return FIRE(scan.dir)
        if scan.drift != 0:                # target sliding: step ahead of it
            think("cutting off"); return MOVE(side_toward(scan.dir, scan.drift))
        think("closing"); return MOVE(scan.dir)
    tgt = argmax(mem)                      # ~200 ops total, fits budget
    think("checking hotspot"); return SCAN(dir_to(tgt))
```
Budget tuned so a coarse 8x8 belief update fits, a full simulation of everyone doesn't.

## 4. Combat & scoring
- HP 3, bullet dmg 1, speed 2 cells/tick, range to wall, fire cooldown 3 ticks, friendly fire on.
- Scan wedge: 90 deg, range 10. Signal = 100 - 8*dist - 15*offAxis, halved if target dark.
- Simultaneous resolution: moves, then bullets advance, then damage (deterministic order by bot id only for ties in the same cell - bounce both back).
- Respawn 3 s at random edge (seeded), 2 s spawn shield (can't fire, can't be hit).

| Event | Pts |
|---|---|
| Kill | +3 |
| Assist (hit within 8 ticks of kill) | +1 |
| Coin (spawn 1 per 4 s) | +1, golden +3 |
| Death | -1 |
| Team kill | -3 |

## 5. Spectator layer
- **Brain strip**: one column per bot at screen edge showing its rules (cards as icons, L1/L2 as source lines). The firing rule/line glows in the bot's color each tick; state name as a big badge above the bot ("LURK", "FLEE"); timers as rings around the bot; registers as 4 small bars.
- **Thought bubble**: Layer 2 `think()` text, Layer 0/1 auto-generated from rule ("fire N - enemy 84%").
- **Visible information**: scan wedges as flashlight sweeps; sound as ripples sized by noise; pinged bots flash an eye icon; dark bots dim to 40%; decoys look like footsteps to *bots* but are drawn as hollow ghosts for the room.
- **Director camera**: zooms when a bullet is <=2 ticks from a bot or two bots have mutual scan > 60. Slow-mo 0.25x on near-miss/kill.
- **Instant replay** (deterministic re-sim) of each kill, with both brain strips, so the room sees *why*: "Ghost decoyed west, Rex took the bait".
- **State-change ticker**: "REX: HUNT -> FLEE (hit from E)". Stall sparks get a laugh.

## 6. Session format
- Phones join via QR; TV runs host. 4-8 players, solo or 2-4 teams.
- **Match** = 5 rounds x 2 min, 90 s editing between. Round 1 house bots join to fill.
- After each round the winner's program is shown on TV for 15 s (as cards/source) - counter-programming fuel.
- **Weekly ladder**: submit a bot anytime; nightly deterministic round-robin; Friday live final on TV.
- Optional mixed teams rule: each team needs one card-only member.

## 7. AI-written programs
- **Encouraged with an AI pack**: one-page spec (this doc's 3.1-3.3), house bots' source, and a CLI `arena sim mine.bot --vs house/* --seeds 50` so AI can iterate offline on the exact engine.
- **AI helps card players too**: ask an AI "write me a Layer 1 bot", paste text, it appears as cards. AI is an equalizer, not a programmer-only edge.
- **Fairness is structural**: same sensors at all layers, same 300 ops, no wall-clock, no network, no persistence between rounds, no access to opponents' code or identities (enemies are anonymous signals). A script can only be *smarter*, never *better informed*.
- **Readability rule**: a Layer 2 bot without `@state`/`think()` displays as "???" - social pressure to stay watchable.

## 8. Risks / degenerate strategies
| Risk | Counter |
|---|---|
| Camping in the dark | 12-tick heartbeat, coins elsewhere, lava ring |
| Spray fire every tick | cooldown 3, gunshot reveals you arena-wide |
| Scan-spin forever, never commit | scan reveals you via `pinged_from` |
| Decoy spam | 2 per life |
| Over-budget as free WAIT | stall is loud + visible |
| AI scripts crush card players | info ceiling (fog caps advantage), Card League bracket, AI-to-cards path |
| Spawn camping | random seeded edge + 2 s shield |
| Team radio as hidden channel to enemies | radio team-only, 1 int/tick |
| One meta bot dominates | RPS: lurker beats hunter, hunter beats collector, collector-with-decoys beats lurker; weekly map rotation |

## 9. Implementation sketch
- **TypeScript monorepo**: `engine` (pure, integer, seeded xorshift), `compiler` (cards -> L1 AST, L1/L2 parser -> bytecode, own parser, no `eval`), `vm` (op-counted interpreter), `tv` (PixiJS), `phone` (web app), `cli` (same engine in Node).
- Host laptop runs engine authoritatively; phones send source over WebSocket; compile errors returned instantly, shown inline.
- Sandbox is inherent: bytecode only touches its own memory, op counter enforces budget.
- Replay = map seed + programs + edit log; any kill re-simulates exactly for slow-mo and ladder disputes.
- Bytecode carries a source map (op -> card/line) so the TV highlight is free.
