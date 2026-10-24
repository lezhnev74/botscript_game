# Judge verdict (Opus)

Criteria: (a) TV readability & pace, (b) simple to learn, (c) depth / counter-programming, (d) AI improves not breaks, (e) office fit & social fun, (f) v1 buildable in a few weeks.

## 1. Scores

| # | Design | a | b | c | d | e | f | Sum | Verdict |
|---|---|---|---|---|---|---|---|---|---|
| 01 | Refined baseline (Gambit Grid) | 8 | 8 | 8 | 8 | 8 | 8 | **48** | The safest, most complete base: fixes the baseline's real flaws (pace, spin-scan, no state) without breaking what works. |
| 08 | SIGNAL (one language, one VM) | 8 | 7 | 10 | 10 | 7 | 5 | **47** | Best mechanics for feints and ambushes (noise, dark, decoy) and the cleanest AI story. Too much compiler for v1, 250 ms too fast. |
| 04 | PAINTLINE (territory) | 10 | 8 | 5 | 5 | 8 | 8 | **44** | The most readable TV of all, but the program model is too shallow and AI hits the ceiling on day one. Strong second mode. |
| 02 | CODEBOTS (JS) | 7 | 4 | 9 | 9 | 7 | 6 | **42** | Max depth and AI fit, but non-programmers become slider users, and `threats()/safe()` hands out perfect dodging for free. |
| 03 | Same beat (energy, simultaneous) | 7 | 5 | 8 | 7 | 6 | 8 | **41** | Glow-as-tell is real yomi, but the energy rules (scan discount, skipped income) are hard to learn, and the doc is cryptic. |
| 06 | BEARING (continuous) | 6 | 4 | 9 | 9 | 6 | 5 | **39** | Robocode on a TV. The AI ceiling is high, but so is the mush risk, and aim math shuts out non-coders. |
| 07 | Bot Hour (spectator meta) | 9 | 7 | 4 | 5 | 9 | 3 | **37** | Great TV-show and ritual ideas bolted onto the unchanged baseline game. The layer for v2, not the game. |
| 05 | Squad radio (roles) | 5 | 3 | 8 | 6 | 5 | 3 | **30** | 4 chassis x 3 modes x radio codes x captain taps. A balancing and readability trap. |

## 2. Best ideas (mix freely)

1. **Firing-rule badge on every bot** (01 card badge, 08 brain strip + state badge, 07 rule trace). This is the single feature that makes "players don't steer" watchable: the room reads the program by watching it run.
2. **One program model, many front-ends** (01 BotCard text <-> cards, 08 layers -> one VM, 02 cards compile to `think`). AI pastes text and gets cards back. Nobody gets powers others lack.
3. **Quiet vs loud sensing** (01 SWEEP vs PING, 08 `pinged_from`). Exact information costs you exposure. This kills spin-scan-fire and creates baiting.
4. **Noise model + stillness "dark" + heartbeat** (08, 01 HEARD). Moving and firing make sound, still bots get harder to see. Ambushes are real mechanics, not hopes, and the heartbeat prevents pure camping.
5. **DECOY, 2 per life** (08). Turns "feint" from a buzzword into one card. The TV draws it as a hollow ghost, so the room is in on the joke.
6. **3-level signals instead of percentages** (01). Bars read from across the room, and the thresholds stay simple for non-coders.
7. **States/tabs + timers** (01 Tier 2, 08 "for N ticks" chip). The minimum memory that lets bait-then-flank exist. A non-coder can build Hunt/Flee.
8. **Visible overheat** (01 steam, 08 sparks, 02 BRAIN FRY). Budget enforcement becomes comedy and an anti-loop rule at once.
9. **Lookahead director** (01 1.5 s render delay, 07 pre-simulated broadcast). The deterministic sim knows the kill before the TV shows it, so slow-mo lands exactly. Cheap and huge.
10. **Kill-cam with both rules** (07 "Rule 2: enemy 91% > 80% -> FIRE" / victim's missed rule, 02 victim-POV fog). This is the teaching tool and the punchline in one.
11. **Program reveal in edit windows** (01 top-3, 03 fork public texts). This is what drives counter-programming. Without it, the edit window is guessing.
12. **Prompt pack + copy-AI-prompt button + offline runner** (01, 02, 08 CLI `--seeds 50`). AI iteration becomes better strategy, and everyone gets the same compute.
13. **Bubbles/mood, lies allowed** (02 `say`, 08 `think`, 07 auto plain-English from the rule). Intent stays visible, and bluffing is comedy.
14. **Endgame forcing: storm ring + golden coin "Last Call"** (01, 07). Every round ends in a scrum.
15. **Hall-of-fame bosses + nightly round-robin** (02, 08, 07). Last week's champion becomes this week's house bot, and the ritual sustains itself.

Honorable: diminishing kill points per same victim (03) against farming; the "restless" idle reveal (01); roast the program, never the person (07); territory paint as a mode (04).

## 3. Reject

- **Real JS/QuickJS as the main model (02).** It needs a sandbox, sliders are a second-class game for non-coders, and helpers that compute safe cells make dodging trivial, so "kills come from crossfire" really means stalemates.
- **Continuous space (06).** Fixed-point trig, aim math, and the risk of readability mush on a TV. Grid "same row" is a visible fact, and that is gold.
- **Asymmetric chassis, multiple modes, radio codes, captain taps (05).** Months of balancing, unreadable digits, and every session needs exactly 2-4 per team.
- **Energy economy with scan discounts and skipped income (03).** Too many interacting numbers for "learn at the coffee machine". Keep only the visible-tell idea.
- **Betting, odds from Monte Carlo, sponsorship, LLM casters (07) in v1.** Huge scope. Betting at work is an HR conversation. Template ticker first.
- **Coin-unlocked rule slots and handicap coins (baseline, 07, 04 season slots).** These snowball and punish rookies. Use equal caps.
- **250 ms ticks (04, 05, 07, 08).** Too fast with 8+ bots. 01's own diagnosis is correct.
- **Raw percentage formulas on TV (05, baseline).** No one can read "72% of what".
- **Mid-round live edits (01, 04).** Turns programming into slow steering and favors the person standing closest to the TV. Edit between rounds only.
- **Hunger tax that drains HP (02).** It deletes the ambusher archetype. Prefer 01's "restless" reveal.
- **Underdog spray boost (04).** Visible rubber-banding feels like cheating. A bounty on the leader is cleaner.
- **Python-subset Layer 2 (08) for v1.** It means writing a compiler. Tier 2 cards cover 90% of the depth.

## 4. Recommended combined design (~400 words)

**Core loop.** Program, then watch, then read the kill-cams and the revealed programs, then counter-program in 90 s, then watch again.

**World.** 21x13 grid with mirrored seeded cover and 4 directions. 1 tick = 400 ms (tunable from 350 to 500). A round is 3 min (450 ticks). 6-10 bots, 3 HP, bullets travel 2 cells per tick, reload 3. Point-blank (<= 2 cells) always lands. Respawn after 8 ticks at the edge farthest from enemies, with a no-fire shield.

**Program model.** Ordered IF -> THEN cards with a 1:1 text form.
- **Tier 1:** 6 cards, 1 condition each.
- **Tier 2:** up to 4 states plus an ALWAYS list, 2 timers, registers A-C plus D (direction), AND/NOT, a 24-card cap, 6 instant ops per tick, then overheat.
- Same caps for everyone, no unlocks.

**Sensors.**
- Passive: BULLET dir, HIT dir, HEARD quadrant (moves within 4 cells, fire heard arena-wide), SCANNED dir, BLOCKED, HP, RELOADED, ALLY NEAR, ALLY SAID.
- From scans: ENEMY/COIN dir faint/near/locked (from SWEEP), LOCKED + distance (from PING).

**Actions.** MOVE, SIDESTEP, SWEEP (quiet, fuzzy), PING (loud, exact, warns the target), FIRE, WAIT, DECOY (2 per life). Direction words: N E S W, CW, TARGET, HIT, HEARD, AWAY, COIN. A bot still for 3+ ticks is dark (sweeps read it one level lower). After 20 idle ticks it becomes restless (visible to all).

**Scoring.** Kill +3, assist +1, coin +1, death -1, team kill -3, +2 bounty on the leader (crown). The last 30 s bring a shrinking storm and a +5 golden coin at center.

**Session.**
- Break mode: 3 rounds x 3 min, with 90 s edits and the top-3 programs revealed.
- Weekly mode: Rookie Heat (Tier 1), Open Heat, then a final.
- House bots fill empty seats. Teams of 2 are optional with ALLY SAID.

**Spectator.**
- Card and state badge on every bot.
- Brain strip for the 2 most interesting bots.
- PING drawn as a red laser, SWEEP as a faint cone, sound as ripples, decoys as ghosts.
- Lookahead director with slow-mo (max 1 per 10 s).
- Kill-cam inset with both bots' rules.
- Auto plain-English bubbles and a template ticker.

**AI.** A Prompt Pack plus a copy-prompt button. Pasted text is validated into cards, and errors come back in plain language. An offline runner (same engine, 8x speed) is available to everyone.

**MVP (3-4 weeks).**
- Integer deterministic sim.
- Text parser/validator.
- Tier 1 + Tier 2.
- Phone text editor with a card preview.
- PixiJS TV with badges, scans, ripples and the director.
- Kill-cam, 3 house bots, between-round reveal.
- SQLite for programs and replays.

**Later.** Drag-and-drop card editor, Elo ladder, nightly round-robin, hall-of-fame bosses, territory mode (04), POV fog inset, awards, LLM caster, maybe script Layer 2.

## 5. Unresolved decisions for the owner

1. **Tick speed:** 400 ms vs 800 ms (03). Only a real TV at 4 m answers this. Prototype with a slider.
2. **Depth ceiling:** is Tier 2 cards enough for the AI crowd, or does a script layer (08 L2 / 02 JS) come later? It decides whether you build a compiler.
3. **Reveal policy:** reveal the top 3 every window (fast meta, copying), all programs after the match (06), or keep one hidden rule (05)?
4. **FFA vs teams as the default.** Teams add cooperation, but they need even headcounts and more UI.
5. **Friendly fire on?** It's more tactical but can feel like griefing. Default on with -3, off in Rookie Heat?
6. **Is phone card-UI needed in v1,** or does text + AI paste + templates carry the non-coders for the first sessions?
7. **Persistence:** a session-only game vs a season ladder. That decides how much backend (SQLite, Elo, nightly runs) v1 needs.
8. **Betting / fake currency:** fun or an HR risk in your office? Decide before anyone builds it.
9. **Kill scoring:** a bounty on the leader vs diminishing points per repeat victim (03). Pick one, not both.
10. **Fog on the TV:** god view always (dramatic irony) vs an optional POV inset. Recommend god view plus a POV inset only in kill-cams.
