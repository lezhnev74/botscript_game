# L6 - REFLEX (events + default loop)

## 1. Idea

A program is a **body** and **reflexes**. The body is one short `loop:` of steps, played one per beat when the world is calm. The reflexes are prioritized `on <event>` handlers checked every beat, top down, first match wins; the winner's action replaces that beat's loop step. Scratch "when hat" blocks with Robocode priorities, tap-built. The two degenerate cases bracket the whole game: a newcomer ships a 2-step loop with zero reflexes; VISION's card bots are 8 reflexes over a 1-step `otherwise` loop. Same interpreter, no new powers.

## 2. Grammar

```
program := botline, loopline, handler*
botline := bot "Name" [tier 2]
loopline:= loop: step [, step] ...            1..6 steps
handler := on test -> step                    reflex: 1 beat
         | on test => step [, step] ...       job (tier 2): 1..3 steps, then done
test    := ev [+ ev] [- ev]                   1 required + at most one AND, one NOT
step    := MOVE d | SWEEP d | PING d | FIRE d | LEAD | SIDESTEP | WAIT | DECOY d (t2)
d       := N | E | S | W | CW | TARGET | TOWARD | AWAY | HIT | HEARD | COIN | SCANNED | SAID (t2)

ev (tier 1): bullet  hit  heard  scanned  line  step_in  armed
             enemy faint|near|locked   coin   loaded   bonus n   hp low
             stuck   ally near   friend   rand n
ev (tier 2): said d
```

Events are nouns, not booleans: `bullet` means a round reaches me within 2 beats, `hit` means damage last beat, `line` a clear fire line now, `enemy near` from scan memory (2-beat fresh), `stuck` my last move bounced, `friend` an ally sits on my fire line, `rand n` a 1-in-n coin flip re-rolled each beat. Loop steps are **actions only** - all the "when" thinking lives in handlers. That split is the teachable core: body = what I do, reflexes = what interrupts me. Caps: tier 1 = 8 handlers + 6 loop steps; tier 2 = 20 handlers, jobs, `SAID`. No GOTO, no while: the interpreter cannot loop, so VISION's overheat rule is dropped as dead weight.

```
bot "Lamp"
loop: SWEEP CW , MOVE N , SWEEP CW , MOVE S
on bullet -> SIDESTEP
on line + loaded -> FIRE TARGET
on enemy near -> PING TARGET
```

## 3. Tap editor

Layout (portrait): top = bot name + Fight button; middle = stack of handler cards in priority order (colored event icon, one line each, drag handle + trash); a labeled seam "**when nothing happens**"; then the loop as a **filmstrip** of step chips with a pulsing "next" chip; bottom = big **+ reflex** button. All pickers are full-screen sheets of 8-12 thumb tiles. No keyboard except the name.

Defaults make 0 taps playable: joining creates a valid bot `loop: SWEEP CW , WAIT`, and one tap on a template chip (Lamp / Magpie / Possum / Hunter) installs a full archetype.

First bot, counted (newcomer path, ~10 s):
1. See default bot sweeping on TV, doing nothing (0 taps, 5 s of shame).
2. Tap **+** (1), tap tile **bullet** (2), tap tile **SIDESTEP** (3). Card closes itself. The bot now dodges.
3. Tap the **WAIT** chip in the filmstrip (4), tap **MOVE** (5), tap **N** (6). It wanders and sweeps.

6 taps, no typing, working bot. A reflex costs 3 taps + 1 per modifier chip + 1 if the action needs a direction (`SIDESTEP`, `LEAD`, `WAIT` need none; `FIRE TARGET` costs 5 total).

Edit: tap any card or chip - it opens in the same pickers pre-filled; tap a tile to replace that slot. Reorder: long-press + drag (priority = order); filmstrip drag reorders steps. Delete: trash on long-press. Invalid programs are impossible: every card ships template-complete with legal defaults, sheets only offer legal tiles, tiers hide illegal events, and a half-edited card still parses (its old value stays until a tile is tapped). The only free text is the AI import, which validates into red cards with plain-language fixes, per VISION.

## 4. Semantics per beat + hot-swap

Lock phase, on the shared snapshot: evaluate handlers top down (max 20 cheap sensor lookups, all precomputed by the sim - no cost, no side effects). First true handler fires: a reflex (`->`) runs its one step, a job (`=>`) starts and its steps run on subsequent beats. While a job is active, handlers are still checked each beat; a true handler preempts and cancels the job, **except** the handler that spawned it. If nothing is true: the job's next step, else the loop's next step (pausing, not resetting, when interrupted; wrap at the end). Then the beat resolves in VISION's published order. Cost per beat is O(handlers), deterministic, replay = seed + program.

TV badge: loop beats show `L3 SWEEP CW` in the bot's calm tint; a firing handler flashes `!2 SIDESTEP` with a bright rim; a job shows `J coin 2/3`. The room reads temperament at a glance: looping bots look placid, twitchy programs sparkle, and everyone sees the exact line that fired.

Hot-swap (any time, v1): edits build a new AST on the phone; Save sends it; the server swaps **atomically at the next beat lock**, and that lock is spent as a WAIT with a visible reboot flicker - the one-beat pause is the whole cost of mid-round editing, so no twitch-edit cheese. Program state (loop index, active job) resets to fresh; world state (HP, reload, coins) is untouched. Dead bots take the swap at respawn. AI paste takes the same path after validation.

## 5. Learning curve

10 seconds (6 taps, above):
```
bot "Bo"
loop: SWEEP CW , MOVE N
on bullet -> SIDESTEP
```

2 minutes (regular: dodge, snipe, lead, grab coins, hunt):
```
bot "Bo"
loop: SWEEP CW , MOVE N , SWEEP CW , MOVE S
on bullet -> SIDESTEP
on step_in + loaded -> LEAD
on line + loaded - friend -> FIRE TARGET
on coin => MOVE COIN , MOVE COIN
on enemy near -> PING TARGET
```

One AI prompt (tier 2: kite, bait, team):
```
bot "Wolf" tier 2
loop: SWEEP CW , WAIT , SWEEP CW , MOVE TOWARD
on bullet => SIDESTEP , MOVE HEARD          # juke twice, not a predictable perpendicular
on hp low => MOVE AWAY , MOVE AWAY , SWEEP CW
on step_in + loaded -> LEAD
on line + loaded - friend -> FIRE TARGET
on said N => MOVE TOWARD , MOVE TOWARD      # ally called contact north
on enemy locked => DECOY AWAY , WAIT , WAIT # loud steps one way, body stays dark
on scanned + loaded -> FIRE SCANNED
on coin -> MOVE COIN
```

## 6. Depth

- **Ambush (Possum):** dark loop `WAIT, WAIT, SWEEP CW`; `on heard + loaded -> FIRE HEARD`. The loop keeps you still; only reflexes spend beats.
- **Kite:** `on enemy near => MOVE AWAY , MOVE AWAY` plus `on line + loaded -> FIRE TARGET`; loop just sweeps. Range-keeping with zero math.
- **Lead shot:** native: `on step_in + loaded -> LEAD`, else free WAIT.
- **Bait / feint:** jobs are temporal lies: `on enemy locked => DECOY AWAY , WAIT , WAIT` sells footsteps while the body camps; `on bullet => SIDESTEP , MOVE HEARD` beats opponents who LEAD your sidestep, and `rand n` variants make the juke unreadable even to yourself.
- **Team play:** `on said N => MOVE TOWARD , MOVE TOWARD` converges on an ally's ping; `- friend` on every FIRE card is polite crossfire; `on ally near + hp low -> WAIT` bodyguards.
- **Economy aggression (Magpie):** `on coin => MOVE COIN , MOVE COIN , MOVE COIN` then `on bonus >= 1 + line -> FIRE TARGET` double-taps through reload.
- **Counter-play is structural:** a feint-heavy Wolf dies to `on armed -> SIDESTEP` spammers; predictable loops die to LEAD. The RPS archetypes are all 3-6 line programs.

## 7. Weaknesses, honestly

- **Loop starvation:** one always-true handler (`on enemy faint`) reduces REFLEX to a flat rules list; the rhythm pitch silently dies and the language's identity with it. No clean fix - the loop is only as alive as the player's restraint.
- **Blunt preemption:** any stray reflex (`heard`) cancels a 3-step job, so long plans are fragile; a "pause jobs" flag would add back complexity I cut on purpose.
- **No states:** bait-then-switch phases need jobs to fake what GOTO does natively; the AI depth ceiling sits below a real state machine unless jobs grow (and then they become states with extra steps).
- **Edge vs level confusion:** `on hit` is last-beat, `on enemy near` is 2-beat memory, `on line` is now. Players will misread all three; badges help, wording will not fully.
- **Drag-to-repriority on a phone is fumbly** mid-match, and a botched reorder is a silent tactic change.
- **Stale directions:** `TOWARD/COIN` come from scan memory; a 3-step chase walks blind for 2 beats and eats a LEAD.
