# Bot Arena: "Bot Hour" - Spectator & Social Meta Design

## 1. Pitch
A weekly 35-minute office broadcast: coworkers write rule-card bots, the TV airs the fights like a sports show (auto-director, AI casters, live win-probability, fake-money betting), and a season ladder, trophy and hall of fame turn it into a ritual.
Fit: the TV is a TV *show*, not a debug view. Non-programmers get roles (bettor, sponsor, fan, bot-namer) and a kind, funny failure culture. Programmers and AI users get a ladder, versions, replays and a meta to crack.

Key trick: **the match is pre-simulated, the broadcast is a "live" replay with ~25 s delay.** Sim is deterministic and fast, so the server knows the whole match before the TV shows it. That gives the director, caster and odds engine perfect foresight (cuts before the kill, exact slow-mo timing, scripted commentary with zero latency).

## 2. World model
Baseline holds: grid arena, 4 directions, 250 ms ticks, 180 s match (720 ticks), 6-8 bots per arena.
TV layout: arena always fully visible on ~80% of the screen (never zoomed, never panned); right 20% sidebar = leaderboard, kill feed, win-probability bar, odds ticker. Bots: big shape + color (colorblind-safe pairs) + nameplate with owner name. Director adds one picture-in-picture inset max (sec. 5).

Core tweaks needed by the meta layer:
1. **Rule trace**: each tick the sim logs `{bot, rule#, signals, action}`. Feeds bubbles, kill cams, post-mortems, stuck detection.
2. **Spawn shield**: 1 s invulnerability after respawn (visible shimmer). Stops rookie spawn-kill misery.
3. **Bounty system** (catch-up, sec. 4).
4. **Last Call**: at t=150 s a golden coin (+5) drops at arena center and the shrink starts. Guaranteed finale scrum.
5. **Sudden death**: if top two tie at 180 s, 20 s extra, next kill wins, no respawn.
6. **Barks**: each bot has a name (16 chars, filtered), color, motto, and two canned barks (kill bark, death bark) shown as speech bubbles. Cosmetic only, zero engine impact, high comedy value.
7. Crowd input never touches live sim state (betting integrity). The only crowd lever is the **modifier vote** before the edit window (coin shower / pillars / double golden). Everyone programs against the same modifier.

## 3. Program model
Baseline rule cards (IF signal -> THEN action, priority order, 4 slots, coins unlock more, up to 8). Sensors: enemy/coin/bullet signal per direction (wedge scan), hit-from, last-scan memory. Actions: SCAN, FIRE, MOVE, SIDESTEP. One action per tick. Programs are pure data (JSON), no code execution, so no CPU budget needed in the base tier.
Every rule card has an auto-generated plain-English line, reused by bubbles and commentary.

Examples:
```
Sniper "Dot"                       AI-written "Prism v3" (6 slots)
1 bullet any>50 -> SIDESTEP        1 bullet any>30 -> SIDESTEP away
2 enemy N>80 -> FIRE N             2 enemy(last)>85 -> FIRE that dir
3 enemy N>30 -> MOVE to stronger   3 enemy(last)>40 -> MOVE to raise signal
4 else -> SCAN N                   4 coin(last)>60 AND enemy<20 -> MOVE to coin
                                   5 hit -> SCAN toward hit dir
Bait "Decoy"                       6 else -> SCAN next clockwise
1 bullet any>40 -> SIDESTEP
2 enemy(last)>60 -> FIRE that dir  (AI output is solid but predictable:
3 coin(last)>20 -> MOVE to coin     the room learns to bait its thresholds.)
4 else -> SCAN next (parks near coins; hunters walk into its lane)
```

## 4. Combat & scoring
Coin +1, kill +3, death -2, respawn 3 s at random edge, golden coin +5.
Bounty/catch-up:
- Kill the current score leader: +2 bonus. Leader gets a crown icon (visible target).
- 3-kill streak without dying = "On Fire" (flame icon); killing it pays +1 bonus.
- Death costs never go below 0 total.

League points per match (not raw score): placement 5/3/2/1/0. Rating via **OpenSkill (Weng-Lin)**, which handles free-for-all and teams, used for seeding, handicap and ladder display.
**Handicap** (golf-style, shown on TV as "HCP +2"): H = clamp((median rating - rating)/100, 0, 3) coins pre-banked toward slot unlocks. First 3 matches = "Rookie" (rating shown as ?, rookie shield: spawn shield 2 s, +1 free slot).

## 5. Spectator layer
**Pipeline**: headless sim -> event log -> highlight detector -> director script (camera, slow-mo cues) -> commentary script -> bet odds timeline -> one broadcast timeline JSON the TV plays back in sync.

**Highlight scoring** (per 5 s window): kill 3, kill of leader 5, near-miss (bullet passes within 1 cell / sidestep with 1-tick margin) 3, lead change 4, win-prob swing over 25 pts 4, friendly fire 4, golden pickup 3, stuck bot (no useful action 10 s) 2, anything in last 5 s x2. Drives camera, recap reel, clip export and awards.

**Director**: interest map per zone; inset (2x zoom of hot zone, corner nearest it) with 3 s minimum dwell and hysteresis; a glow ring on the main view shows what the inset watches. Cuts 1 s early thanks to foresight. Main arena never moves.

**Dramatic irony overlays** (viewer knows, bot doesn't): dotted bullet path with a pulsing X on the landing cell 3 ticks ahead; for the focused bot a **trigger ring** fills as its signal approaches the rule threshold ("72% / needs 80%") - the room groans together.

**Slow-mo**: tick-stretch, not video: 2x tick duration for up to 4 ticks (sim clock unaffected). Triggers: near-miss, decisive kill in last 15 s, lead-changing kill. 12 s cooldown to avoid fatigue.

**Kill cam** (notable kills, interest >= 3): 3 s inset at 0.5x, plus two text lines: killer "Rule 2: enemy N 91% > 80% -> FIRE", victim "Rule 1 saw bullet at 28%, needed 40%". This is also the teaching tool for newcomers.

**Thought bubbles**: all bots show a rule icon; focused bots show full rule text + signal values.

**Auto-commentary**: two casters (play-by-play "Rex", color "Dot"). One LLM call per match (fast small model, e.g. claude-haiku-5-5) during the reveal/betting window, input = event digest + bot cards (name, owner, motto, rating, form, rivalries, last-week story), output = timestamped lines `{t, speaker, text, refs:[eventIds]}` of at most 25 words. Validator: refs must exist, names must match, content filter, roast rule (roast the program, never the person; per-player opt-out, "Mystery bot" anonymity). Fallback templated lines if the call fails. Subtitles always on, TTS optional (local Piper), "lunch mode" = subtitles + SFX only. A larger-model call writes intros and the end-of-round recap.

**Betting ("Bolts")**:
- Everyone has a bank: 1000 weekly grant, +100 for playing, 20% carryover. Spectators join by QR on phones.
- Odds = 0.92 / p, clamped 1.1x-25x, where p comes from 200 Monte Carlo rollouts (different seeds) at reveal time. House pays, no parimutuel.
- Markets: match winner, exact placement, props (first blood, will anyone get a streak, friendly fire, golden coin winner). Stakes 10-300. Betting closes at t=60 s; win-prob bar reprices every 4 s from 16 rollouts per snapshot (pre-computed).
- Players may bet only FOR their own bot (blocks throwing). Free betting on other lobbies.
- **Sponsorship**: stake bolts on a bot, its nameplate shows "Sponsored by Alice", you earn 10% of its winnings.
- Sinks: cosmetic skins, bark packs, caster nicknames ("The Butcher of Row 7"). Never pay-to-win.
- "Oracle" leaderboard per season.

**Reveal ceremony** (15 s after edit lock): cards flip one per ~2 s: name, owner, version, tier, auto "doctrine" line ("Dodges everything, shoots above 80%"), AI-assisted badge if declared. Full programs browsable on phones. Odds crunch behind it, bets close, countdown.

**Intermission (60 s)**: recap reel (top 3 clips, 6 s each, casters talk over), animated rating changes, scouting report ("tale of the tape": form sparkline, auto style tags Camper / Sniper / Hoarder / Pacifist, head-to-head), next-modifier vote, edit countdown.

**Replays**: replay = `{simVersion, seed, programs, modifier}` (~2 KB) plus keyframes every 40 ticks for resilience to sim upgrades. Scrub, share link, **"what if"**: swap one program and rerun the same seed. Practice mode: fight last week's champion or house bots. Clip export: highlight windows to GIF/MP4 posted to the Slack channel.

## 6. Session format
- Doors open: TV runs Hall of Fame loop + last week's recap; QR join; 5 min warm-up edit vs house bots; modifier vote.
- Qualifiers: 3 cycles, each 3 min match + 60 s edit (+15 s reveal). More than 8 players: heats A/B alternate so heat B edits and bets while heat A fights; waiting players are the live audience.
- Grand Final: top 4 by session points, 2 rounds, double points.
- Awards ceremony (3 min), then trophy photo.
- **Season** = 6 weeks, season points 10/6/4/2 for Cup finish, Season Cup final. Physical trophy (golden rubber duck/robot) passed on, plaque gets the bot name.
- **Overnight ladder**: every night the server runs ghost round-robins of each player's latest saved version (rating weight 0.5) and posts a morning Slack report. Versioned bots (v1, v2...), diff of rules shown on reveal.
- Teams: optional 2-person teams in the Final (ally-ready cards), friendly fire on.
- Matchmaking: snake-seed lobbies by rating for balance.

**Awards (auto-computed)**: Bot of the Week (rating gain), Giant Killer (beat top rating), Bravest Dodger, Pacifist, Hall of Shame (friendly fire), Bug of the Week (stuck detector, opt-in funniest failure), Best Comeback, Prompt of the Week (opt-in shared AI prompt). Winners get an entry in the Hall of Fame (portrait, program, signature clip).

## 7. AI-written programs
Welcome and productized: "Copy bot spec" button gives a prompt with the card schema; AI returns JSON, pasted back and validated. Optional "authored with AI" badge (honor system, pure flavor; one ladder, no segregation). Fairness comes from equal slot caps and data-only programs. Coach button on phone: post-mortem plus one LLM-suggested tweak, not a full rewrite. The AI's output is solid but predictable, so scouting reports and the "what if" replay let humans find counters; the visible threshold style of AI bots is the room's favorite thing to bait.

## 8. Risks
- **Caster bores/hallucinates/offends**: validator, templates, mute, opt-out, roast-the-bot rule.
- **Public failure shame**: bubbles frame bugs as comedy, Mystery bot anonymity, rookie shield.
- **Sandbagging for handicap**: handicap only grants banked coins (max 3 slots-worth), Cup qualification uses session points, not rating.
- **Betting collusion/throwing**: own-bot-only betting, fake currency, weekly reset.
- **Meta stagnation**: weekly modifier vote, season reset with soft rating compression, auto "Meta report" (most common rule 1, top conditions).
- **Slow-mo/overlay fatigue**: cooldowns, one inset, overlays only on focus bots.
- **Leader snowball**: bounty + crown, Last Call golden coin.
- **Foresight leak**: broadcast timeline server-side only, 25 s delay.

## 9. Implementation sketch
TypeScript monorepo. Sim in TS (or Rust->WASM) with integer/fixed-point sensors, seeded xoshiro RNG, no wall clock; same module runs on server and browser. Budget <= 50 us/tick so a full match is ~40 ms and a 16-rollout reprice grid costs about 5 s of CPU. Programs = JSON validated by schema, no code execution (a future script tier would run in QuickJS-WASM with an instruction cap). Node server + WebSocket, SQLite for bots, versions, ratings, bets. TV client = PixiJS playing the broadcast timeline; phone client = PWA (editor, bet slip, post-mortem). Worker pool for rollouts; LLM via API with 10 s timeout and template fallback; Slack webhook for clips and reports.
