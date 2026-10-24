# Tuning (Step 19)

Evidence for the defaults of `sim.DefaultRules()` and the `cmd/arena` settings. Goals come from VISION section 1 and 12:
slow enough to follow, fast enough for tension, a distant target has a good chance to step aside, skill beats noise,
and "up to 10 bots" on one TV.

## Tool

`cmd/sim` runs matches headless (parallel, deterministic: same flags, same output).

```
cd engine && go build -o /tmp/sim ./cmd/sim
/tmp/sim -map testdata/maps/19x13.map -bots rookie,lighthouse,tracker,rookie,lighthouse,tracker \
         -matches 400 -beats 300 -sweep bullet_speed=1..5 -sweep reload=2,3,4
```

- `-bots` is a count (random action scripts) or a list: `rookie,lighthouse,tracker,random`, `name:count` repeats. A map
  file without bots gets them placed at random per match (`testdata/maps/{11x9,15x11,19x13,23x15}.map`, pillars in the corners).
- `-program A=tracker`, `-script B=wait`, `-rule k=v` override single bots/rules. Without `-sweep` it prints totals and a
  per-program table; with `-sweep key=lo..hi|a,b,c` (repeatable) one row per combination.
- Columns: `kill/100b` kills per 100 beats (whole arena); `self%` self-kills of deaths; `acc` hits per shot; `1st-hit` mean
  beat of the first hit; `life` alive beats per death; `idle%` alive beats without an action; `dodge` share of
  level-3 bullet readings not followed by a hit from that side within 5 beats, over the dodging programs only (`-dodgers`,
  default lighthouse and tracker; rookie never reacts to bullets, so it would only dilute the number); `coins/m` coins per match; `smart%` kills by
  house programs (not `random`). Per-program table adds `share` (of kills), `line` (mean beat of the first level-3 enemy
  reading), `clicks` (fires with nothing loaded).
- All numbers below: 400 matches x 300 beats, seed 1, house bots cycling rookie, lighthouse, tracker.

## Targets used

| Goal | Band |
|---|---|
| Kill pace on the TV (6..10 bots) | one kill every 2..7 s |
| Life of a bot | 15..40 s (long enough to read its program, short enough to be exciting) |
| Dodge rate of the dodging bots (lighthouse, tracker) at level 3 | 30..50% ("a good chance", not a sure thing) |
| Skill matters | house bots take well over their share of kills against random scripts; ladder tracker > lighthouse > rookie stable |
| Self-kills, idle beats | about 0 |

## Results and decisions

### Bullet speed: keep 3
19x13, house bots cycling rookie, lighthouse, tracker; dodge = lighthouse and tracker only:

| speed | kill/100b | acc | life | dodge | | kill/100b | acc | life | dodge |
|---|---|---|---|---|---|---|---|---|---|
| | 6 bots | | | | | 10 bots | | | |
| 1 | 4.97 | 0.41 | 115 | 0.73 | | 15.6 | 0.48 | 59 | 0.71 |
| 2 | 6.22 | 0.47 | 92 | 0.60 | | 18.8 | 0.55 | 48 | 0.58 |
| **3** | **7.32** | **0.54** | **77** | **0.49** | | **21.4** | **0.63** | **42** | **0.45** |
| 4 | 7.82 | 0.58 | 72 | 0.38 | | 23.1 | 0.68 | 38 | 0.31 |
| 5 | 8.25 | 0.61 | 68 | 0.26 | | 24.0 | 0.71 | 37 | 0.21 |

Speeds 3 and 4 are both inside the 30..50% band (3 at its upper edge, 4 at its middle); 2 and below make a bullet easy to
sidestep, 5 makes it almost a hitscan. This is a judgement call between 3 and 4. 3 is kept because it is the VISION default,
the bullet stays visible for more beats, and the house dodgers are basic (sidestep on the first reading): bots written
to dodge well will score above these numbers, which argues against going faster. Timeline: a scan costs a beat and the sidestep
the next, so a bullet is dodgeable only when fired from farther than 2 x speed cells (7 at speed 3, 9 at speed 4).
Balance between programs does not depend on the speed; duel kill shares (19x13, 600 matches x 300 beats, speed 2 / 3 / 4):

| duel (`-bots`) | winner share |
|---|---|
| `lighthouse,tracker` | tracker 63 / 63 / 64% |
| `rookie,tracker` | tracker 70 / 70 / 69% |
| `rookie,lighthouse` | lighthouse 65 / 59 / 58% |

Reproduce: `/tmp/sim -map testdata/maps/19x13.map -bots rookie,lighthouse -matches 600 -beats 300 -rule bullet_speed=3`
(per-program `share` column). VISION 12.3 (fire on 3 vs dodge on bullet 3): against a pure shooter the dodger wins about 58-65% here, so
dodging pays without being decisive.

### Reload: keep 3
Lighthouse, tracker, rookie + 3 random scripts, 19x13:

| reload | kill/100b | acc | smart% |
|---|---|---|---|
| 1 | 6.69 | 0.21 | 73 |
| 2 | 6.22 | 0.22 | 77 |
| **3** | **5.73** | **0.26** | **78** |
| 4 | 5.55 | 0.28 | 79 |
| 6 | 4.80 | 0.31 | 81 |

With house bots only (6 bots) reload 1 and 2 give identical results: a scan-then-fire loop needs two beats anyway, so reload
binds from 3. Below 3 only spam (random blind fire) gains, and skill share drops; above 3 each step buys 1-2 points of skill share for
about 5% of pace. 3 is the smallest value that rewards aiming, and the kill pace stays inside the band.

### Arena: 19x13 (was 15x11)
Defaults otherwise, dodge of lighthouse and tracker, 6 bots / 10 bots:

| size | kill/100b | life | dodge | | kill/100b | life | dodge |
|---|---|---|---|---|---|---|---|
| 11x9 | 11.1 | 49 | 0.08 | | 29.8 | 29 | 0.09 |
| 15x11 | 9.6 | 57 | 0.34 | | 25.5 | 34 | 0.28 |
| **19x13** | **7.3** | **77** | **0.49** | | **21.4** | **42** | **0.45** |
| 23x15 | 5.4 | 107 | 0.60 | | 17.5 | 52 | 0.55 |

11x9 is out (dodge under 10%, a life of 29 beats with 10 bots). 23x15 leaves 6 bots a kill every 19 beats (9 s at 500 ms) and
dodging is too easy. 15x11 versus 19x13 is close: 15x11 is in or near the dodge band (0.34 / 0.28), keeps 6-bot pace at 5 s per kill, but a
10-bot life is 34 beats (17 s at 500 ms), near the lower edge of the band. 19x13 has dodging at the upper end of the band
(0.49 / 0.45), a 10-bot life of 21 s and 6-bot pace of 6.8 s per kill, still inside the band, and more room for a bullet to be seen and dodged.
It was chosen on that margin; 15x11 would also be defensible. It is the built-in map of `cmd/arena` (`defaultMap` equals
`testdata/maps/19x13.map`, pinned by a test).

### Beat length: 500 ms (was 400)
Beats per kill is 13.7 (6 bots) and 4.7 (10 bots) on 19x13; a life is 77 / 42 beats.

| beat | s per kill (6 / 10 bots) | life (6 / 10 bots) | bullet over 19 cells |
|---|---|---|---|
| 400 ms | 5.5 / 1.9 | 31 / 17 s | 2.5 s |
| **500 ms** | **6.8 / 2.3** | **38 / 21 s** | **3.2 s** |
| 600 ms | 8.2 / 2.8 | 46 / 25 s | 3.8 s |

Every scan is an x-ray (cone, silhouettes, level bars) plus a badge with the running line; at 10 bots that is several scans
a second. 400 ms puts a kill every 1.9 s with a full crowd, which cannot be read; 600 ms makes a 6-bot arena slow (8 s between kills).
500 ms keeps both ends inside the 2..7 s band and a bullet visible for about a second per 6 cells. It is a server flag
(`-beat`, `make run BEAT=`), so a venue can still override it. Time scales linearly, so the table extends to any value.

**Playtest override: 1000 ms.** At 500 ms the TV could not be followed live. The default is now 1000 ms, bots glide
600 ms between cells and scans are no longer drawn. A kill comes every 13.7 s with 6 bots (outside the band, accepted for now).

### House-bot seats: `-min-bots` 5 (was 6, 3 before)
With 3 bots a kill comes every 95 beats (about 50 s at 500 ms) - nothing happens on the TV. At 6 it is every 14 beats. Seats above
6 are filled by players; `balance` removes house bots as players join.

**Step 31 override: 5.** The arena starts with 5 house bots; each joining human replaces the oldest house bot, a leaver is replaced again.

### Cone: keep `cone_slope` 100 (new rule, percent)
The cone shape was code; it is now `Rules.ConeSlope` (most perpendicular offset per cell along, in percent; 100 = the 90 degree wedge,
the PLAN section 5 default). 19x13, 6 bots:

| cone_slope | kill/100b | acc | life |
|---|---|---|---|
| 0 (line only) | 0.90 | 0.38 | 660 |
| 50 | 7.41 | 0.54 | 76 |
| **100** | **7.32** | **0.54** | **77** |
| 150 | 6.97 | 0.53 | 81 |
| 200 | 6.96 | 0.53 | 81 |

From 50 up the house bots are insensitive; only a line-only cone kills the game (level 2 and 1 are what makes tracking possible).
100 is the one value where four scans (N, E, S, W) tile the arena exactly: below it the diagonals are never seen, above it the
cones overlap and scanning gets redundant (VISION 5: "4 scans cover the whole arena"). Kept, and pinned by `defaults_cone.scn`.

### Coins: `coin_max` 5 (was 3), `coin_every` 10
19x13, 6 bots:

| coin_max | coins/match | kill/100b |
|---|---|---|
| 0 | 0 | 6.78 |
| 3 | 4.7 | 7.07 |
| **5** | **7.4** | **7.32** |
| 8 | 10.7 | 7.43 |

`coin_every` 5 vs 10 changes nothing once `coin_max` is 4+ (the map is full either way). With 3 coins and a 19x13 room only 4.7 coins
are taken in 300 beats, about 7% of all points; 5 gives 7.4 (about 10%) and a bonus-shot pull that raises kill pace by 4% without
changing dodge or life. More than 5 clutters the TV for little. House bots collect weakly (rookie never), so players who aim for
coins gain more than these numbers show.

### House bots with `wall()` (Step 21)
Lighthouse and Tracker now dodge with `wall()`: sidestep to the open side, and in a walled corridor retreat along the line (slot `b = -1` / `c = 2`
= sidestep pending) and sidestep next beat; Tracker turns before walking into a wall instead of bumping. Rookie is unchanged. Re-run on the
current engine (hp 4, line of sight), 19x13, 400 matches x 300 beats, seed 1, defaults, house bots cycling rookie, lighthouse, tracker:

| bots | bullet_speed | kill/100b | beats per kill | acc | life | dodge (lighthouse / tracker) |
|---|---|---|---|---|---|---|
| 6 | 2 | 4.79 | 21 | 0.65 | 121 | 0.63 |
| 6 | **3** | **5.71** | **18** | 0.77 | 100 | **0.44** |
| 6 | 4 | 6.21 | 16 | 0.86 | 91 | 0.31 |
| 10 | 3 | 18.0 | 5.6 | 0.80 | 47-58 | 0.37 / 0.42 |

At the default speed 3 the dodge rate is 0.44 (6 bots), inside the 30..50% band; kill pace is a kill every 18 beats with 6 bots and 5.6 with 10
(9 s / 2.8 s at 500 ms; 18 s / 5.6 s at the 1000 ms default). Ladder by duel kill share (19x13, 600 matches x 300 beats): `lighthouse,tracker` tracker 60%;
`rookie,tracker` tracker 87%; `rookie,lighthouse` lighthouse 70%. tracker > lighthouse > rookie holds. The earlier tables in this file were measured with the old
dodge and before the hp 4 and line-of-sight changes; they were not re-run, only this section is current.

### Forager, Hound, audible pings and rarer coins (Step 28, balance round)
New house bots: Forager (ping, pin, path; looks every other beat; runs from `enemy >= 2` unless loaded and `hp >= 3`) and Hound (follows
`heard`, alternates a look and a step, looks all round on arrival, fires on 3, pins the last sighting). Lighthouse and Tracker keep their
`num`-only code. Round 1 (pings silent, `coin_every` 10) failed the pass criterion: the Forager topped 88-99 of 100 seeds at every `hear_range` and
coin value (the Hound at most 2). Round 2 changes: a ping is audible like a shot (hearers cannot tell them apart, so hunters can track foragers),
a negative direction is a wait (no behaviour change for the bots; `move(path(p))` is now safe), and `coin_every` was swept.

Setup: 19x13 (`testdata/maps/19x13.map`), lineup forager, hound, lighthouse, tracker, rookie, seed 1, other rules default, audible pings.
Score per bot summed over 200 matches x 300 beats, then who topped the score table in 100 single-match seeds (winners). This table supersedes the
round 1 one (silent pings, `coin_every` 10: forager 88-99, rest at most 8 each).

| coin_every | coin pts | hear_range | forager | hound | lighthouse | tracker | rookie | winners (of 100) |
|---|---|---|---|---|---|---|---|---|
| 10 | 1 | 4 | 3794 | 1172 | 987 | 1509 | 653 | forager 95, tracker 4, hound 1 |
| 10 | 1 | 6 | 3757 | 1320 | 1039 | 1462 | 677 | forager 85, tracker 8, hound 5, lighthouse 2 |
| 10 | 1 | 8 | 3864 | 1373 | 1051 | 1474 | 650 | forager 95, tracker 2, hound 2, lighthouse 1 |
| 10 | 2 | 4 | 6834 | 1654 | 1385 | 1760 | 653 | forager 97, hound 2, tracker 1 |
| 10 | 2 | 6 | 6732 | 1724 | 1492 | 1704 | 677 | forager 97, hound 2, lighthouse 1 |
| 10 | 2 | 8 | 6939 | 1748 | 1488 | 1713 | 650 | forager 99, hound 1 |
| 20 | 1 | 4 | 2694 | 987 | 808 | 1396 | 761 | forager 81, tracker 12, hound 3, rookie 2, lighthouse 2 |
| 20 | 1 | 6 | 2718 | 1163 | 753 | 1432 | 681 | forager 79, hound 10, tracker 9, lighthouse 2 |
| 20 | 1 | 8 | 2656 | 1254 | 845 | 1324 | 645 | forager 74, tracker 12, hound 11, lighthouse 2, rookie 1 |
| 20 | 2 | 4 | 4655 | 1215 | 972 | 1522 | 761 | forager 95, tracker 3, hound 2 |
| 20 | 2 | 6 | 4662 | 1365 | 928 | 1555 | 681 | forager 98, tracker 1, hound 1 |
| 20 | 2 | 8 | 4602 | 1429 | 1045 | 1445 | 645 | forager 94, tracker 4, hound 2 |
| 30 | 1 | 4 | 2098 | 930 | 724 | 1331 | 766 | forager 67, tracker 18, hound 9, rookie 3, lighthouse 3 |
| 30 | 1 | 6 | 2019 | 1103 | 788 | 1369 | 737 | forager 59, tracker 22, hound 11, lighthouse 7, rookie 1 |
| 30 | 1 | 8 | 1979 | 1318 | 706 | 1375 | 685 | forager 54, tracker 22, hound 16, rookie 4, lighthouse 4 |
| 30 | 2 | 4 | 3462 | 1065 | 825 | 1404 | 766 | forager 93, tracker 3, lighthouse 2, hound 2 |
| 30 | 2 | 6 | 3382 | 1216 | 890 | 1446 | 737 | forager 88, tracker 6, lighthouse 3, hound 2, rookie 1 |
| 30 | 2 | 8 | 3343 | 1419 | 809 | 1442 | 685 | forager 85, hound 7, tracker 4, lighthouse 3, rookie 1 |
| 50 | 1 | 4 | 1424 | 914 | 653 | 1421 | 779 | tracker 46, forager 36, hound 7, rookie 6, lighthouse 5 |
| **50** | **1** | **6** | 1294 | 1119 | 721 | 1377 | 763 | tracker 42, forager 21, hound 18, lighthouse 10, rookie 9 |
| 50 | 1 | 8 | 1266 | 1285 | 708 | 1408 | 738 | tracker 34, hound 31, forager 22, lighthouse 8, rookie 5 |
| 50 | 2 | 4 | 2257 | 971 | 697 | 1462 | 779 | forager 70, tracker 24, hound 4, lighthouse 2 |
| 50 | 2 | 6 | 2107 | 1176 | 777 | 1415 | 763 | forager 54, tracker 30, hound 10, lighthouse 4, rookie 2 |
| 50 | 2 | 8 | 2072 | 1333 | 763 | 1443 | 738 | forager 56, tracker 18, hound 18, lighthouse 6, rookie 2 |

Kills and coins per beat at the chosen setting (200 x 300 = 60000 match-beats): 2499 kills (0.042 per beat), 976 coins (0.016 per beat, 4.9 per
match; at `coin_every` 10 it was about 21 per match). Ladder at the chosen setting: tracker 1377, forager 1294, hound 1119, rookie 763, lighthouse 721.
Both Forager and Hound beat Rookie in every row of the table. Coin points 2 hands the Forager the table again at every `coin_every` (54-99%);
a longer `hear_range` helps the Hound (at 8: tracker 34, hound 31, forager 22) but 6 already spreads well.

**Chosen: `coin_every` 50 (was 10), coin points 1, `hear_range` 6 (unchanged).** Pass criteria hold: no single bot tops more than 42% of seeds
(tracker 42, forager 21, hound 18, lighthouse 10, rookie 9), and Forager and Hound both beat Rookie. Pinned by `defaults_coins.scn`
(coins on beats 50 and 100 only), `TestDefaultRules`, and `app.DefaultPoints()` (coin 1). The change altered `duel_5x5` (its seed moved from 4 to 34
to stay a real fight, 3+ kills), the long goldens and `hashes.txt`. Record versioning: a record carries every rule, so the new default never alters an
existing record (see the `RecordVersion` comment); audible pings and negative-direction waits belong to the not yet released version 4.

### No coin radar (Step 37)
`ping()` and the audible ping are gone. Forager is rewritten: it scans (every other beat), heads toward a coin it saw (heading kept in `num[2]`, re-looked each
look, forgotten when the coin leaves the scan), otherwise wanders to random pinned spots with `path`; the flee rule is unchanged. Rerun at the defaults
(`coin_every` 50, coin points 1, `hear_range` 6; 19x13, lineup forager, hound, lighthouse, tracker, rookie, seed 1, 200 matches x 300 beats):
kills 2422 (0.040 per beat), coins 704 (0.012 per beat, 3.5 per match). Ladder: tracker 1373, forager 1140, hound 1012, rookie 740, lighthouse 668.
Winners over 100 single-match seeds: tracker 41, forager 31, hound 17, lighthouse 6, rookie 5. Pass criteria hold (Forager and Hound beat Rookie; no bot
tops every seed), so `coin_every` stays 50.

### Scan timing (Step 40)
Scans now resolve after bullets fly (moves, shots, bullets, scans, hits): a scan reads only bullets still in the air, a bullet fired this beat is counted where it
stopped. Same lineup and settings as above (forager, hound, lighthouse, tracker, rookie, seed 1, 200 matches x 300 beats):
kills 2243 (0.037 per beat), coins 714 (3.6 per match), accuracy 0.72 (hits 10384 of 14432 shots). Ladder: tracker 1290, forager 1082, hound 952, rookie 780, lighthouse 662.
Winners over 100 single-match seeds: tracker 39, forager 20, hound 19, rookie 13, lighthouse 9. Pass criteria hold (Forager and Hound beat Rookie; no bot tops every
seed). Dodgers (lighthouse, tracker) now react a beat earlier, so shots fall a little and the `four_9x9` long golden moved to seed 49 to keep 3+ kills.

### Unchanged on purpose
`hp` 3, `bonus_max` 2, `respawn_beats` 5, `shield_beats` 2 were not in scope. With the final defaults the per-program view
(19x13, lighthouse, tracker, rookie + 3 random scripts): tracker 33% of kills, rookie 24%, lighthouse 20%, the three random scripts together 22%
(a random bot gets 1/3.5 of a house bot's kills; it still lands some, so a newcomer sees it work). Self-kills 0, idle 1% for house bots.

## Pinned by scenario

`defaults_bullet_speed.scn`, `defaults_reload.scn`, `defaults_cone.scn`, `defaults_coins.scn` (all `rules: default`),
`defaults_cone_far.scn` (a cell 9 along and 10 off, catches slopes of 112 and up; `TestDefaultConeNeverAdmitsAnOffsetBeyondItsDistance` covers 101 and up), `scan_cone_slope_is_a_rule.scn`; `cmd/arena` tests pin `-beat` 500, `-min-bots` 5 and the 19x13 map.
Changing `coin_max` 3 -> 5 is the only default change: it altered the long goldens (`hashes.txt`) and two web fixtures
(`duel_5x5`, `four_9x9`); replaying the old records with `coin_max=3` reproduces the old hashes exactly.

## Format changes
`Rules.Set` (scenarios, `-rule`, `-sweep`) now rejects values below a rule's minimum (`bullet_speed`, `reload`, `hp`, `respawn_beats` at least 1;
the rest at least 0). `app.RecordVersion` is 2 (rules gained `cone_slope`); `DecodeRecord` still reads version 1 documents without
`cone_slope`, as a 100 wedge, and returns them as version 2.

## Caveats
- The population is three simple house bots; a field of AI-written bots will shift kill pace and dodge rates. Re-run the sweeps with real bots
  (`-program`, or extend `lang/bots`) before the first event.
- `dodge` is an estimate from scan and hit events (5 beat window, per direction), not a replay of the bot's intent.
- Sample noise: 400 matches x 300 beats; differences below about 3% in `kill/100b` are noise.
