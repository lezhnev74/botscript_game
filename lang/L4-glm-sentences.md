# DICTA (L4) - the sentence builder

**Idea:** a program is a numbered list of short English sentences the bot obeys top to bottom, first true sentence acts. Every word is a slot; the phone picker only offers words that fit the next slot; the sentence text is the exact language an AI writes and the phone imports 1:1.

## 1. Grammar

Text form: `bot "Name"` header (`deep` flag = tier 2), numbered sentences, one per line, period-terminated. Fixed vocabulary, positional slots. Paste is normalized (case/punctuation) and otherwise rejected with a plain-language fix. Export is the same text: round-trip exact.

**Tier 1 sentence kinds:**
- `If <cond> and <cond>, <verb>.` or `If <cond> but not <cond>, <verb>.` - max 2 clauses (3 in deep), first match wins.
- `Otherwise, <verb>.` - exactly one, pinned last, always pre-seeded.

**Conditions** = subject + predicate. The picker pairs them, so illegal pairs cannot exist. Any condition may take a trailing direction word (north/east/south/west); omitted means "somewhere" (any). Every VISION sensor maps to a word:

| Subject | Predicates | Sensor |
|---|---|---|
| an enemy | is faint / near / locked / in line / stepping in / armed | ENEMY levels, ON_LINE, STEP_IN, ARMED |
| a bullet | is coming | BULLET |
| a coin | is faint / near / locked | COIN |
| my gun | is loaded / is empty | LOADED |
| I | am hit / am scanned / hear a step / hear a shot / have a bonus shot / am hurt (2 HP) / am almost dead (1 HP) | HIT, SCANNED, HEARD, BONUS, HP |
| a friend | is near / is in line | ALLY NEAR, FRIEND |
| the way north/east/south/west | is blocked | BLOCKED |
| a chance | is 1-in-2 / 1-in-3 / 1-in-4 | RAND |

**Verbs:** `fire at X`, `lead it`, `ping X`, `sweep X`, `move X`, `step aside`, `wait`; deep adds `fake steps X`, `mark the spot for N beats` (sets the spot and arms timer 1), `start timer 2 for N beats`, `shout X`, `switch to hunting/hiding/baiting/fleeing/collecting`.

**Direction nouns (X):** compass words; `it` = direction of the subject that made the sentence true; `back` = source of last hit/scan; `the noise`; `the shout`; `the spot`; `the coin`; `away` (move only); `clockwise` (sweep only).

**Deep (tier 2) additions:**
- **Moods:** up to 4 from the fixed list, each a sentence list with its own `Otherwise`, plus an `always:` list evaluated first. Bot starts in its first mood.
- **Then-chains:** `fire at it, then step aside.` Remaining verbs queue and run on following beats before evaluation (queue max 2).
- **Memory:** the spot (one direction, `mark the spot`); **timers:** 2, `If timer 1 rings, ...`.
- **Talk:** `shout east`; `If an ally shouts, ...`; `the shout` as a direction noun.

Caps: tier 1 = 8 sentences; deep = 24 sentences, 4 moods, 6 ops per beat (1 op per tested condition, 1 per verb). Over cap = overheat: the bot waits, steam puff on TV.

## 2. Tap editor

Portrait phone. Top: bot name + 4 template cards (Sniper, Collector, Ambusher, Hunter). Middle: the numbered sentence list, dim `Otherwise` pinned last, a `+` button. Bottom: a half-screen picker sheet of 6-8 big word chips; the tapped slot gets a caret, and chips are only grammatical continuations, so an invalid program cannot be constructed.

Defaults make 1 tap enough: tap any template and press Join - that is a complete working bot. Every new sentence is born complete with defaults (`If an enemy is near, sweep clockwise.`), so leaving it untouched is still valid. **Auto-collapse:** if a subject has one sensible predicate (`a bullet` -> `is coming`) or a verb one complement (`step` -> `aside`), the next slot auto-fills; auto-fills stay tappable.

**First bot: 1 tap** (template; name field pre-focused).
**One custom sentence from scratch** ("If a bullet is coming, step aside."):
1. tap `+` (sentence appears, defaulted), 2. tap subject, 3. tap `a bullet` (predicate auto-fills), 4. tap verb, 5. tap `step` (complement auto-fills). **= 5 taps.**
Add a clause: tap the small `and`/`but not` chip after the condition (+2-3 taps). A full custom 4-sentence tier-1 bot from blank: **~15-18 taps, under 30 s.**

Edit: tap any word to reopen its picker (2 taps per change). Reorder: tap the sentence number -> up/down/trash row. Delete: trash or swipe. `Otherwise` is pinned and cannot be deleted, only re-worded. A "deep" toggle unlocks mood tabs and the tier-2 words. The paste box imports AI text; errors like `Sentence 3: I don't know "banana" - did you mean "the noise"?` highlight the bad slot after import.

## 3. Execution per beat

Same snapshot for everyone. Evaluate `always:` then the current mood's list top-down; each tested condition costs 1 op; the first true sentence fires its verb (1 op). TV badge: `#4 fire at it` - sentence number + verb phrase, tinted like the bot's scan. Kill-cam shows the full sentence that killed.

`it` resolves to the direction of the subject of the first true clause; if a verb has no resolvable target (or no ammo), it degrades to WAIT and clears the queue - never a crash. Then-chains run their queued verbs first on later beats, badge `#4 then: step aside`. Over 6 ops = overheat (wait + steam). Tier 1 cannot overheat structurally.

**Hot-swap:** edit any time. Commit ("Done") validates fully, then takes effect at the next beat boundary; the current beat finishes on the old program. Runtime resets: first mood, empty queue, cleared timers and spot. TV flashes a small "reprogrammed" chip. Between rounds all programs stay public per the reveal rule.

## 4. Learning curve

**Newcomer, 10 s, 1 tap (Sniper template):**
```
bot "Lamp"
1. If a bullet is coming, step aside.
2. If an enemy is in line and my gun is loaded, fire at it.
3. If an enemy is near, ping it.
4. Otherwise, sweep clockwise.
```

**Regular, 2 min (moods, but-not, mark, then-chain):**
```
bot "Doris" deep
always:
1. If a bullet is coming, step aside.
2. If I am hurt, switch to fleeing.
in mood hunting:
3. If a friend is in line, wait.
4. If an enemy is in line but not armed, and my gun is loaded, fire at it.
5. If an enemy is near, mark the spot for 6 beats, then switch to baiting.
6. Otherwise, sweep clockwise.
in mood baiting:
7. If timer 1 rings, switch to hunting.
8. Otherwise, fake steps to the spot.
in mood fleeing:
9. If I am not hurt, switch to hunting.
10. Otherwise, move away.
```

**AI, 1 prompt (deep, everything):**
```
bot "Wolf" deep
always:
1. If a bullet is coming, step aside.
2. If I am almost dead and a chance is 1-in-2, shout back.
in mood hunting:
3. If a friend is in line, wait.
4. If an enemy is in line but not armed, and my gun is loaded, fire at it, then step aside.
5. If an enemy is stepping in and my gun is loaded, lead it.
6. If an enemy is near but not locked, ping it.
7. If an enemy is locked and I have a bonus shot, fire at it, then fire at it.
8. Otherwise, sweep clockwise.
in mood baiting:
9. If timer 1 rings, switch to hunting.
10. If I am scanned, switch to hunting.
11. Otherwise, fake steps to the spot.
in mood fleeing:
12. If I am not hurt, switch to hunting.
13. If an ally shouts, move to the shout.
14. Otherwise, move away, then step aside.
```

## 5. Depth

- **Bait/feint:** mark the spot on a near enemy, bait with `fake steps to the spot` while the body is elsewhere, timer walks you back to hunting; `If I am scanned, switch to hunting` is the discovered-bait escape.
- **Ambush:** hiding mood: `wait` goes dark, then `If I hear a step and my gun is loaded, fire at the noise.`
- **Lead shot:** `If an enemy is stepping in, lead it.` beats the sidestep prediction game.
- **Kite:** fleeing `move away, then step aside` while hurt; `but not armed` clause resumes the duel safely.
- **Team play:** `shout` on lock + `If an ally shouts, move to the shout.`; `If a friend is in line, wait.` prevents team kills.
- **Coin aggression:** bonus double-tap (`fire at it, then fire at it`) makes collectors dangerous and worth hunting.
- **Anti-AI noise:** `If a chance is 1-in-3, step aside.` breaks deterministic predators.

## 6. Weaknesses

- The English shape invites English expectations: "if an enemy is behind cover" feels legal but isn't. The phone picker hides this; AI paste hits it constantly and needs excellent error copy. Synonyms (shoot, blast, northward) must alias or reject.
- `it`, `back`, `the spot` are hidden registers; a mis-resolved `it` fires the wrong way and the badge alone won't explain it. Needs a phone "why did this fire?" trace (one tap on the badge).
- Wordiness: a 24-sentence deep bot wraps to 2-3 lines per sentence on phone and TV; scanning the list is slower than a symbol language like `ON_LINE any -> FIRE`.
- Tap count scales with sentence length; a 3-clause condition costs ~8 taps, so casual players realistically stay at template + 2 sentences.
- English-only grammar does not localize cleanly; an i18n pass may need a symbol skin over the same AST.
- Clause cap (3 in deep) forces mood splits for 4-factor logic (loaded + armed + in line + no friend), which is more roundabout than a gated card list.
