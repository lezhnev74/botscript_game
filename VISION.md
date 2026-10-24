# BOT ARENA - Game Vision (current)

One TV, up to 10 coworkers' bots, programmed in a tiny JS, mostly written by AI. Bots **scan, think, act**.
The TV x-rays every scan, so the room reads each program by watching it play.

Language spec: `LANGUAGE.md` (BOTSCRIPT, a tiny JS).

Status tags: **[agreed]** = owner decision; **[default]** = accepted for now, may change after simulations;
**[later]** = decided to defer.

---

## 1. Goal [agreed]

- One screen for everyone (office TV). Slow enough to follow every move, fast enough for engagements and tension.
- Simple rules and actions, but enough to express interesting fighting logic.
- People will ask AI to write programs, and that should improve the game.
- A newcomer must have a working (bad) bot in seconds, watch it fail, and fix it.

## 2. Mode [agreed]

- **Kills** are the main mode. Territory (PAINTLINE) is **[later]**.
- **Free-for-all only**, no teams **[agreed]**. Empty seats are filled by house bots.

## 3. World

| Rule | Status |
|---|---|
| Grid, 4 directions, walls around the arena | agreed |
| **No facing.** Direction is an attribute of each action (`move N`, `scan E`, `fire W`) | agreed |
| One action per beat; all bots decide on the same snapshot (simultaneous lock) | agreed |
| Beat length: decided **after simulations** | later |
| Moving into a wall = **no-op** (beat spent, nothing happens) | agreed |
| Moving into a bot fails, the mover gets `on bump`; **the other bot notices nothing** | agreed |
| Beat order: moves -> new shots -> bullets travel -> scans -> hits -> coins (a scan sees only bullets still in the air); **a target that moves on the beat you fire wins** (bullet hits air) | agreed |
| Bullets are **faster than bots but not instant**: a distant target that sees it has a good chance to step aside | agreed |
| Bullet speed 3 cells/beat, flies until a bot or a wall | default |
| Blind fire (firing without a scan) is allowed | agreed |
| **Gunfire is audible** [default]: a real shot (loaded or bonus; a click is silent) is heard by every other bot within 6 cells (Manhattan, `hear_range`, walls do not block). A bot reads only the cell of the nearest shooter as a place (`heard`, read-only, `none` before any, kept until a newer sound) | default |
| HP 4, shown as a health bar [playtest] | default |
| No cover blocks in v1; walls hide what is behind them from scans | default |
| Arena size, coin count, respawn rules: tune in simulation | later |

## 4. Weapons [agreed]

- **No energy.** Firing has a **reload** cooldown (start at 3 beats [default], tune by simulation).
- **Coins give extra**: +1 score, +1 **HP** (up to full) [playtest] and +1 **bonus shot** (max 2 [default]), fires even while reloading, lost on death.
- `fire` with nothing loaded wastes the beat, "click" on TV [default].

## 5. Scan-n-act [agreed]

- The core loop is **scan -> analyze -> act**. A scan costs **1 beat**.
- A scan looks at the **cone** in one direction, **from the bot to the walls**.
- **Walls block the view** [agreed, playtest]: a cell is seen only with a straight line between cell centres that
  enters no wall (grazing a corner sees). Pillars cast shadows to hide behind. Bots, coins and bullets do not block.
  Scans are **silent** [silent: default].
- **No coin radar** [agreed]: coins are found with `scan` (the `coin` level) and the bot's own guessing.
- A scan returns 3 signals, **level 0-3 per kind**: `enemy`, `coin`, `bullet`. **No side, no distance.**
- Level = **how lined up**: 3 = on my line (a shot this way hits if it doesn't move), 2 = one lane off, 1 = elsewhere in
  the cone, 0 = nothing. `bullet` counts only bullets flying toward me [default].
- **Memory [agreed]:** 8 integer cells `num[0..7]` + 8 place cells `pin[0..7]` (a map cell or `none`, set with `here()` / `at(x, y)`) + a read-only **trail** of the last 3 beats (position, action, result;
  no scan readings, memory only). The bot knows its exact `x, y`.
- **Urgent events only** interrupt the script: `onHit(from)` and `onBump(from)`. After a handler, `loop()` starts fresh.
- Bump into a wall does not trigger `onBump` [default].

## 6. Randomness [agreed]

- Randomizing a bot's actions is a core feature: players tune the random level, and also add intelligence on top.
- `pick(N,N,S)` weighted random direction, **skips walls**; `chance(30)`, `rand(lo,hi)`; `left/right/back(d)` turning.
- Dice are seeded per bot, so replays are deterministic.

## 7. Programming [agreed]

- **A tiny JavaScript**, Turing complete in spirit. The game is built around **AI writing the scripts** [agreed].
- **Thinking is free**; only actions block until the next beat. Slow code (over 100000 ops per beat, deterministic)
  misses beats [agreed].
- **No tap editor** [agreed]: templates, "Copy rulebook for AI", paste box with error hints.
- **Editing is allowed any time** in v1, but a saved program **reboots the bot**: it leaves the map for 10 seconds (the
  reboot, 10 beats by default) and comes back running the new code, keeping HP, gun state, memory and score. This forbids
  puppeteering a bot by live edits (an AI pushing a new program every second); bots must run on their algorithm. Saves
  during a reboot replace the program without restarting the countdown. Leaving and returning is no way around it: a bot
  that left by its own Leave may return no earlier than the reboot time (10 beats by default) after it left. The TV shows a matrix-rain effect with a progress
  bar where the bot left and a pencil badge when it is back.
- Spec: `LANGUAGE.md`.

## 8. TV

- **Scan x-ray [agreed]**: every scan is drawn as exactly the area the bot scanned (its cone tinted in the bot's color,
  to the walls) and exactly what it learned (silhouettes of reported enemies/coins/bullets with 0-3 level bars).
  Objects the bot missed stay plain. The sim emits a **scan record** per scan; the TV draws it, never recomputes it.
- **[default]** from the first synthesis:
  - A pencil badge over a bot right after it is back from a reboot; a matrix-rain effect with a progress bar at the cell of a rebooting bot. No memory badge on the TV; no live view of the line running or the `num` / `pin` cells on the my-bot page for now. No pin markers on the map.
  - A reload ring, a "loaded" muzzle glow, and gold bonus-shot pips.
  - Bullets drawn as tracers; a miss is drawn through the cell the target just left.
  - A lookahead director: the sim runs ~1.5 s ahead, with short slow-mo on kills.
  - A kill-cam inset that replays both bots' last scans.
  - A sidebar with names, HP, kills, deaths, coins and score.
  - Clutter control: one scan per bot per beat; outline-only x-ray for unfocused bots above 6 bots.

## 9. Scoring [default]

| Event | Points |
|---|---|
| Kill | +3 (repeat kills on the same victim +1) |
| Coin | +1 (tuned over 1, 2, 3: higher values let foraging win every match), plus 1 HP and a bonus shot; one spawns every 50 beats (`coin_every`, tuned: rarer coins keep any one role from topping every match) |
| Death | -1 (floor 0), bonus shots lost, respawn after 5 beats far from enemies, 2 beats shielded |

## 10. Session [default]

- **Join:** scan a QR code on the TV to open a phone web app, then get a working template bot.
- **Match:** 3-4 rounds x 3 min, with a 90 s edit window between rounds. *Now:* rounds are admin-timed games: the admin sets
  a timer, when it runs out the game freezes (TV shows "Game over" and the final board, bots stop, the board is kept as
  the "last game" table) and the admin starts the next game by hand (empty board, same players and programs). Edits,
  joins and kicks stay open while the game is over and apply when the next game starts.
- **Reveals:** the top-3 programs are shown on the TV between rounds.
- **AI:** a "Copy rulebook for AI" button exports the grammar, rules and example bots; pasted text imports the bot.
- **Practice:** a "vs house bots" mode.

## 11. Build plan [default]

Go server: a deterministic `sim` + orchestrator (replay = seed + programs + edit log), a `lang` interpreter (goja
parser + whitelist + own op-counting tree-walker), **one goroutine per bot** blocking on actions, WebSocket out.
Web clients: a PixiJS TV and a PWA phone (paste + templates).

**MVP order:**
1. The sim and the language, plus house bots, run headless to tune beat, bullet speed, reload and cone.
2. The TV with the x-ray, tested on the real TV.
3. The phone (paste box, templates, AI prompt export).
4. The director and kill-cam.

## 12. Open

1. Beat length, bullet speed, reload, arena size, coins: run simulations.
2. Revisit the **[default]** rules after simulations.
3. Duel balance: does "fire on 3" vs "dodge on bullet 3" make good mind games? This is the next simulation task.
