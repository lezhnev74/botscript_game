# CLAUDE.md

Bot arena game. Rules: `VISION.md`, `LANGUAGE.md`. Build plan: `arena/PLAN.md` (steps 1-19). Code lives in `arena/`.
`designs/`, `lang/`, `judging/`, `archive/` are history - read-only, not specs.

## Implementing the plan

The main session orchestrates, it does not write production code itself.

- **One step at a time**, in order. Never run two steps in parallel - each depends on the previous one's files.
- **Implementer: Sonnet subagent** (`model: sonnet`). Give it a self-contained brief: the step text from
  `arena/PLAN.md`, sections 0-5 of the plan as ground rules, pointers to `VISION.md`/`LANGUAGE.md`, and what earlier
  steps already built. It works TDD (red -> green -> refactor) and must finish with `make check` green.
- **Judge: Opus subagent** (`model: opus`), fresh context, after the implementer finishes. It gets the step text and
  the diff, not the implementer's summary. It checks: step scope fully covered, tests exist for every behaviour,
  rules match VISION/LANGUAGE, plan ground rules (determinism, depguard layering, CRAP < 8), `make check` actually
  passes. Verdict: PASS or a list of concrete defects.
- **Loop:** on defects, send them to a new Sonnet implementer round (include the original brief + defects). Max 3
  rounds, then stop and report to the user.
- Opus is used only for judging. Sonnet does all implementation, including fixes.
- After a step passes: mark it `[DONE]` in `arena/PLAN.md`, draft a commit message, and **wait for the user's consent
  before committing**. Never commit on your own.
- Gaps in the spec: use the defaults table in `arena/PLAN.md` section 5; anything not covered there - ask the user.
