# Judge notes

Eight docs. Build one game. Mine the rest for parts.

Scores are 1-10. (a) TV readability and pace, (b) simple rules, (c) depth and counter-programming, (d) AI makes it better, (e) office fit and social fun, (f) a v1 in a few weeks.

| Doc | a | b | c | d | e | f | Sum |
|---|---:|---:|---:|---:|---:|---:|---:|
| 01 Gambit Grid | 8 | 7 | 8 | 8 | 8 | 7 | 46 |
| 02 CODEBOTS | 7 | 5 | 9 | 6 | 8 | 4 | 39 |
| 03 Same beat | 9 | 7 | 9 | 8 | 8 | 8 | 49 |
| 04 PAINTLINE | 8 | 6 | 7 | 7 | 8 | 8 | 44 |
| 05 Squad radio | 5 | 3 | 7 | 5 | 6 | 4 | 30 |
| 06 BEARING | 4 | 4 | 8 | 3 | 5 | 5 | 29 |
| 07 Bot Hour | 7 | 6 | 5 | 6 | 9 | 3 | 36 |
| 08 SIGNAL | 7 | 6 | 9 | 7 | 7 | 5 | 41 |

01. Best cards if Tier 1 ships alone. Tier 2 is a second game.
02. Deep and funny. JavaScript and a sandbox miss the room and the calendar.
03. Build this. Pace, tells, and a hard cap on AI fit in one small ruleset.
04. Best picture of who is winning. The duel is thinner and the sensor list is fat.
05. A squad shooter in the brief's clothes. Do not start here.
06. Models will paste twenty years of Robocode. The TV will show mush.
07. A season-2 broadcast. Steal the ritual later. It is not a combat design.
08. Best mind-game sensors. The script VM splits programmers from everyone else.

## Ideas worth keeping

Mix these. Do not ship any doc whole.

1. **Blind lock on one shared beat** (03). Every bot commits, then the beat resolves. Feint and sidestep fall out of the order. This is the fight.
2. **800 ms, full arena, camera fixed** (03, 07). At 250 ms the room sees noise. At 800 ms a sidestep is a sentence. Never pan the board off a bot.
3. **Energy, and a charge that glows** (03). Passive +1, fire costs 3, charge roots and pays +2 while the badge says CHARGE. Models cannot shoot every beat. The tell is the counter.
4. **The matched card painted on the bot** (01). "#2 FIRE" is how non-programmers learn. Without it, correct play still looks random.
5. **Loud ping versus quiet sweep** (01). A fuzzy scan is private. A confirm is a laser the target and the room both see. Levels are faint, near, locked. Never percents.
6. **STEP_IN and LEAD** (03). A card can name "their last move would enter my line." A lead with no such cell becomes a free wait, so the button is a read.
7. **Programs are data** (01, 03, 04). Phone cards and pasted lines are one list. A bad sensor, a ninth rule, or a second action fails on a line number. No user JavaScript in v1.
8. **Diminishing kills and a far, shielded respawn** (03, 01). First kill on a victim pays. Later kills barely pay. They return far away, safe for one beat. Name-farming stops.
9. **The lead must read from the back row** (04). Huge names, 3 HP ticks, 6 energy pips, scores you can sort at a glance. Paint the floor later, as a mode, after dodges already read.
10. **A kill changes the picture for one second** (04). Draw the tracer through the cell they left, or splash paint. A +3 in a column is not a moment.
11. **The room sees more than the bot** (02, 08). God view on the TV, limited sensors in the program. That gap is the joke. Do not hide the board from the people in the room.
12. **One decoy, after v1** (08). Footsteps one way, body the other. A real feint. Skip the noise-radius table for now.
13. **Say and mood that do not spend the beat** (02). Short text, allowed to lie, one change per two seconds. Auto-label silent bots from the action. Cap bubbles or they become the game.
14. **Texts open at the round end** (01, 03). Give 60 to 90 seconds to answer them. That minute is the meta. Ship Lamp, Bait, and Possum so round 1 is never a blank page.
15. **Replay is seed plus programs** (01, 02, 03). One integer `step` on the TV and in tests. A what-if swap (07) can wait until someone asks to rerun Tuesday.

## Reject

**Continuous tanks** (06). Turret, radar, and intercept math do not read at three meters. The Robocode corpus means a model arrives with virtual guns. Card players cannot answer that in an edit window.

**JavaScript as the game** (02). `step` and `threats` hand every model the same strong bot. The sandbox is real work, and the office will not read the source on the TV. Keep the bubbles. Drop the VM.

**Bot Hour as v1** (07). Pre-sim, rollout odds, LLM casters, bolts, OpenSkill, overnight leagues. Combat is still the baseline, percents and coin-bought slots included.

**Chassis, radio codes, three modes** (05). Four bodies, CTF, escort, bomb, captain taps. A new person cannot see why the shot happened. Codes 0-7 break when the teammate, or the model, guessed the codebook wrong. Live captain taps also mean a person is steering.

**Percent wedges and spin-scan** (baseline, still in 04 and 07). Percents do not read from the couch. "Scan clockwise, fire if hot" has no counter except a faster copy. A quiet scan and a loud confirm kill that loop.

**Slots bought with coins** (baseline, 07). The leader buys a bigger brain. Caps are equal, or the AI user hits the cap and stays there.

**Tier 2 and Layer 2 on day one** (01, 08). States, timers, a 64-word heat map, 300 ops, a Python subset. "Same sensors" is true and not enough. That budget is a different game, and six cards do not beat a belief map.

**A stack of closers, and 250 ms.** Storm, hunger, bounty, golden coin, shrink, underdog spray, sudden death. Each doc adds one. Together nobody remembers them. Use the outer ring for the last 24 beats. 02, 04, 05, 07, and 08 also keep a 250 ms clock. Eight bots at four actions a second will not show intent. An underdog spray bonus feels handed to you. Drop it.

## Combined design

Eight to twelve bots lock one action every 800 ms, blind to each other. Eight cards. Charge glows. Fire costs 3.

Grid about 18 by 12, four directions, a few mirrored cover blocks. Cover stops steps and shots. HP 3, energy cap 6. The TV shows cells, facing, HP, energy, the card badge, bullets, coins, and the glow. Rules stay hidden until the round ends. A round is 3 minutes.

Cards run top to bottom, first match, OTHERWISE last. One test, or two with AND or NOT. One action. Sensors: energy, HP, clear line, last badge, glowed, scanned, step-in, bullet, hit-from, friend, coin, one rand per beat. Directions include toward, away, sidestep, coin, and hit. Costs: wait 0, move 1, scan 1, dash 2, shield 2, fire 3, lead 3, charge 0. Charge roots, glows, and adds +2 on the passive +1. Scan then fire on that facing costs 2. Unpaid locks become wait and keep the point. Paste and phone cards are one list. A bad line fails by number.

Beat order: pay, charge, moves, shields, shots, bullet travel, coins, deaths. Damage uses the post-move cell, so a sidestep beats a shot. Friendly fire hurts and scores 0. Coin: +1 score, +1 energy, then gone. Repeat kills on one victim pay +5, then +3, then +1. Death -1, floor 0. Respawn on a far edge after 4 beats, with one safe beat. Last 24 beats, the outer ring hits campers. Ties: score, then fewer fires, then coins.

QR join, Lamp loaded. Four rounds, 75 seconds to edit on a frozen frame. Late pastes wait. Programs go public. The TV holds the three loudest beats for 20 seconds. Under six players, free-for-all. Else teams of two or three, scores summed. The ladder saves one program and a placement. Empty seats are Lamp, Bait, or Possum.

Whole arena, no crop, no caster. Badge, then motion, glow, tracer. A miss draws through the empty cell. Corner replays at half speed do not stop the live beat. Ticker only for a glow on a line, a lead, or a shot on a shield.

People, models, and templates share the cap. No enemy source, no next lock. A short prompt returns cards. The player reorders them. No 50-seed batch tool.

MVP: `step(state, locks)`, three house bots, canvas, phone cards, paste check, QR room. Fifty headless rounds before the first session. A few weeks if the list stops there. Later, only on demand: loud versus quiet scan, one decoy, paint mode, what-if replay, recap, trophy. States only if eight cards feel solved. No script tier unless you are replacing the card game.

## Decisions you still have to make

**Kills or territory.** I recommend kills and a huge scoreboard. Paint makes the lead obvious and muddies the dodge. If the office cares more about "our color is losing" than about a sidestep, let 04 lead and hang 03's clock on it. Those are different games.

**One uncertainty, not two.** Blind locks already hide the enemy action. Fog on top, as in 08, makes the first session harder. Ship a visible board. Add fog only if the games feel solved.

**Beat length, on the actual TV.** Run Lamp against Bait for two minutes at 400, 600, and 800 ms with eight bots. Keep the slowest speed at which people still lean in.

**How many verbs on day one.** Shield, lead, dash, scan, and charge are a lot. If the help screen stalls the room, cut lead and dash before you cut charge. Charge is the tell.

**When text becomes public.** A reveal after every round is the counter-programming the brief wants, and it lets a model copy the winner in 75 seconds. A one-week delay keeps mystery. I would reveal inside the session.

**Practice.** Ship a versus-house-bots button. Refuse a CLI that batches seeds until the live meta is boring.

**Death.** Start soft, as scored above. If nobody fights, raise the first-kill bounty before you add a golden coin.

**Eight bots until badges stay readable.** Every doc says twelve. Twelve is how a good ruleset becomes confetti.
