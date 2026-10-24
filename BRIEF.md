# Design brief (shared by all designer agents)

Goal: a programming-battle game for coworkers, shown on ONE shared screen (office TV). Players don't steer; they write
programs for bots that fight each other. People will often ask an AI to write their program - that is welcome and
should make the game better (deeper strategies), not break it.

Hard constraints:
- One screen, readable from across a room. Everybody must be able to follow what's happening.
- Pace: not too fast (people can follow movements and intent), fast enough to feel engagements and tension.
- Allowed rules/actions must be simple to learn, yet expressive enough for interesting fighting logic
  (feints, ambushes, baiting, cooperation, counter-programming).
- Non-programmers must be able to participate; programmers / AI users must find depth.
- Sessions fit an office break or a weekly ritual (rounds of minutes, not hours).

Baseline already drafted: `bot-arena-design.md` (grid, priority IF->THEN rule cards, scan wedge, fire, move,
250ms ticks). You may build on it, challenge it, or replace it.

Deliverable: a design doc (markdown, <= ~1500 words, dense, no fluff) covering:
1. One-line pitch + why it fits the constraints.
2. World model (space, time/tick rate, what's visible on TV).
3. Program model: exactly what a player writes (language/DSL/cards), the full list of sensors and actions, limits
   (CPU/instruction budget, memory). Show 2-3 example programs, incl. one an AI would likely write.
4. Combat & scoring rules, numbers included.
5. Spectator layer: how the TV makes tension readable (camera, overlays, replays, commentary, slow-mo, etc).
6. Session format: setup, rounds, editing windows, tournament/ladder, teams.
7. How AI-written programs are handled (allowed, encouraged, kept fair; any anti-degenerate-strategy measures).
8. Risks / degenerate strategies and how the design counters them.
9. Implementation sketch (tech stack, deterministic sim, how programs are submitted + sandboxed) - brief.
Use only short dashes "-", never long dashes.
