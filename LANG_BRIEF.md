# Language design brief (shared by all language-designer agents)

Read first: `VISION.md` (the agreed game: grid, 4 directions, simultaneous beat ~600 ms, reload + coin bonus shots,
SWEEP (quiet fuzzy wedge) / PING (loud exact line), scan x-ray on TV, kills mode, card badge over each bot).
The sensor/action list in VISION.md section 3-4 is a starting point - you may reshape it if your language needs it.

## The problem
Design HOW PEOPLE PROGRAM THEIR BOTS: the language + the phone editor.

Owner's goals (hard):
- It must FEEL LIKE PROGRAMMING with a simple language (not a settings screen, not sliders).
- Editing is a SERIES OF TAPS on a phone: add -> tap to pick type -> tap to pick value, etc. No typing needed.
- A newcomer who just joined must produce a working (possibly bad) bot in SECONDS, watch it behave poorly on the TV,
  and fix it. The first 30 seconds decide whether they stay.
- Editing is allowed ANY TIME in v1, including mid-round (hot-swap). Define what happens on hot-swap.
- An AI can write a much better program: there must be a text form an AI writes and the phone imports, round-trip
  exact (the text IS the language; taps are a structured editor over it). Depth ceiling must be real.
- The running program must be readable on the TV (badge of the line/rule currently firing).

## Deliverable (markdown, <= ~1500 words, dense)
1. Name + one-line idea of the language model (rules list? blocks? state machine? sentences? something else?).
2. Grammar: the full set of statement/expression kinds, sensors, actions, values. Show the exact text syntax.
3. The tap editor: screen layout, step-by-step tap sequence to build the first bot (count the taps!), how to
   edit/reorder/delete, how the editor makes invalid programs impossible, defaults that make a 1-tap bot work.
4. Execution semantics per beat (what runs, how many ops, what "current line" the TV shows) + hot-swap rules.
5. Learning curve: what a newcomer writes in 10 s, a regular in 2 min, an AI in 1 prompt. Show all three programs.
6. Depth: which tactics (bait, feint, ambush, lead shot, kite, team play) are expressible and how.
7. Weaknesses of your own design, honestly.
Use only short dashes "-", never long dashes.
