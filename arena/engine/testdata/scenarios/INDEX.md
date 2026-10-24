# Scenario index

Rule -> scenario(s). A rule with no scenario counts as not implemented.
Scenario format: see `arena/PLAN.md` section 3. Regenerate goldens with
`cd engine && go test ./scenario -update`.
Optional `bullets:` block puts bullets in flight before beat 1: one per line, `x,y,DIR` or `x,y,DIR,OWNER` (floor
cell without a bot, OWNER = a bot on the map). Frames draw bullets as `*` (a bot wins the cell if both would be drawn).
Optional `start:` block sets a bot's initial state: `<bot>: hp=2 bonus=1 cooldown=3` (cooldown = beats until the
regular shot is ready; bonus is capped by `bonus_max`). Also `shield=N` (beats of shield left). Expectations can read `<bot>.hp|bonus|cooldown|shield|loaded|alive|x|y|hit|heard_set|heard_x|heard_y` (`heard_*` = the place: `_set` 1/0, `_x` / `_y` the cell or -1 for none; `alive` = 1/0; a dead bot reads hp 0, bonus 0, x/y where it fell) (`hit` = direction the last beat's hit came from, N=0 E=1 S=2 W=3, or -1).

Bots are `script <actions>` or `program <what>`: `program rookie` (a house bot from `lang/bots`: rookie, lighthouse,
tracker, forager, hound), `program file.js` (relative to this directory) or an inline block (`program |` and the source indented below).
Program errors are reported at the scenario line (inline body) or the declaration line (file). A Record carries the
program text, never a file name. Optional `edits:` block = hot-swaps, `beat N: <bot> program <what>` (same forms; `beat N: <bot> reboot K program <what>` is a reboot of K beats, readable as `<bot>.rebooting` = beats left), applied at
the start of beat N in file order; the bot must be a program bot; they go to `Record.EditLog`. Expectations can also read a
program bot's `<bot>.num_0..num_7`, `<bot>.pin_0_set|x|y..pin_7_set|x|y` (x and y are -1 for an empty pin) and `<bot>.act` (kind of its latest action: SCAN 0, MOVE 1, FIRE 2, WAIT 3, IDLE 4).

Determinism suite: every scenario here is also run twice and once through `app.Replay(Scenario.Record())` (after a JSON
round trip); the three event-log hashes (SHA-256 of the canonical JSON lines) must be equal (`scenario/determinism_test.go`).
`make determinism` compares the hashes under GOMAXPROCS=1, 8 and `-race`. `FuzzMatch` (`app/fuzz_test.go`, corpus in
`engine/app/testdata/fuzz`) does the same for random matches and checks the invariants in `app.CheckInvariants`.

| Rule | Scenario(s) |
|---|---|
| A bot that waits stays where it is (no `Moved` events, HP unchanged) | `wait_stays_put.scn` |
| A move into a free cell succeeds (`Moved`) | `move_free_cell.scn` |
| A move into a wall is a no-op: no event, position unchanged (`ok=false` shown to the bot's next Decide) | `move_into_wall.scn` |
| A move into a bot fails: the mover gets `Bumped <dir>` (dir = the direction it tried), the other bot nothing | `move_bump_into_bot.scn` |
| Two bots into the same free cell: both fail, both `Bumped` (each with its own move direction) | `move_same_cell_conflict.scn` |
| Two adjacent bots swapping: both fail, both `Bumped` | `move_swap_conflict.scn` |
| Snapshot rule: occupancy is judged at the start of the beat, so a follower bumps even if the occupant moves away this beat (no chains) | `move_chain_snapshot.scn` |
| Perpendicular claim: two bots from right angles into one free cell both bump, each with its own direction | `move_perpendicular_same_cell_claim.scn` |
| Scan cone: cells with perpendicular offset <= distance along the scan direction (90 degree wedge); a scan emits `Scanned <bot> <dir> <enemy> <coin> <bullet>` with the cone cells and sightings in the JSON payload | `scan_enemy_level3.scn` (and every `scan_*`) |
| Enemy level 3 = on my line | `scan_enemy_level3.scn` |
| Enemy level 2 = one lane off | `scan_enemy_level2.scn` |
| Enemy level 1 = elsewhere in the cone | `scan_enemy_level1.scn` |
| Enemy level 0 = nothing in the cone (behind, beside, wall-side scan) | `scan_enemy_level0.scn` |
| Coin levels 3 / 2 / 1 / 0, same rule as enemies | `scan_coin_level3.scn`, `scan_coin_level2.scn`, `scan_coin_level1.scn`, `scan_coin_level0.scn` |
| Per kind the best-aligned object sets the level; kinds are independent | `scan_best_aligned_wins.scn` |
| Scans run after moves and read post-move positions | `scan_after_moves.scn`, `scan_target_steps_off_line.scn` |
| Walls block the view (line of sight between cell centres; grazing a wall corner sees past it); wall cells hold nothing | `scan_walls_block_the_view.scn`, `scan_pillar_casts_a_shadow.scn` |
| A bot never reads itself; only the scanned direction counts | `scan_ignores_self_and_other_directions.scn` |
| Bullet levels (only bullets flying toward the scanner count; the scan sees bullets where they stand after the beat's flight) | `scan_bullet_level3.scn`, `scan_bullet_level2.scn`, `scan_bullet_level1.scn`, `scan_bullet_level0.scn` (flying away); more cases in `TestBulletLevels` (`domain/sim/scan_test.go`) |
| Bots start loaded; a shot reloads for `reload` beats (3): fire on beat t, loaded again on beat t+3 | `fire_reload_click_sequence.scn` |
| `fire` with nothing loaded = `Clicked <dir>`, no bullet, beat wasted; the reload keeps counting | `fire_click_with_nothing_loaded.scn`, `fire_reload_click_sequence.scn` |
| Each bot has its own gun and reload | `fire_reload_is_per_bot.scn` |
| Reload length comes from the rules | `fire_reload_rule_is_tunable.scn` |
| Bonus shots fire while reloading and leave the reload untouched | `bonus_fires_during_reload.scn` |
| With the gun loaded the regular shot goes first, the bonus is kept | `bonus_kept_while_loaded.scn` |
| Bonus is capped at `bonus_max` (2): 1 regular + 2 bonus shots in a long reload, then clicks | `bonus_cap.scn` (start cap: `TestParseErrors`; pickup cap: `TestAddBonusIsCapped`, `coin_pickup_at_bonus_cap.scn`) |
| `loaded` = reloaded or bonus stored | `bonus_makes_gun_loaded.scn` |
| Bonus shots are lost on death (and do not come back with the respawn) | `death_bonus_lost.scn` |
| Beat order moves -> shots -> bullets fly -> scans -> hits: a scan reads a bullet fired this beat that is still in the air, where it stopped | `scan_sees_bullet_still_flying.scn` |
| A bullet read on the shot beat is dodged by a sidestep next beat | `scan_bullet_seen_then_sidestep_dodges.scn` |
| A bullet that struck a wall or another bot this beat is gone and not read | `scan_does_not_see_bullet_that_hit_a_wall.scn`, `scan_does_not_see_bullet_that_hit_a_bot.scn` |
| A scanner hit on its scan beat is still hit (and interrupted); the bullet that hit it is not read | `scan_scanner_hit_on_scan_beat_still_hit.scn`, `program_lighthouse_hit_sidesteps_and_hunts.scn` |
| Bullets travel `bullet_speed` (3) cells per beat, starting at the neighbour of the shooter, the same beat they are fired; they fly on across beats | `bullet_travels_three_cells_per_beat.scn`, `bullet_arrives_on_a_later_beat.scn`, `bullet_speed_is_a_rule.scn` |
| A bullet is removed silently at a wall (no event) | `bullet_stops_at_wall.scn`, `bullet_travels_three_cells_per_beat.scn` |
| A bullet hits the first bot on its path: `Hit <victim> <from>`, damage 1, bullet gone, bots behind are safe | `bullet_hits_stationary_target.scn`, `bullet_hits_only_the_first_bot.scn` |
| A bullet never hits its owner at spawn; an adjacent bot is hit at once | `bullet_hits_adjacent_target.scn` |
| Hit direction = where the bullet came from (flying E = from W) | `bullet_hit_direction_from_west.scn`, `_from_east.scn`, `_from_north.scn`, `_from_south.scn` |
| Target that moves on the beat you fire wins (moves run before bullets) | `bullet_target_that_moves_on_the_beat_you_fire_wins.scn` (the section 3 example), `bullet_close_target_that_moves_wins.scn` |
| Bullets are not blockers: bullet meets bullet = pass through | `bullet_passes_through_bullet.scn` |
| Path is checked cell by cell: a bot stepping into a bullet's cell is hit, not skipped | `bullet_hits_bot_that_walks_into_it.scn` |
| Distant dodge: a far target reads the bullet (scan level 3) on the shot beat and steps aside next beat; one within bullet_speed cells is hit on the shot beat | `bullet_distant_dodge.scn`, `bullet_close_target_cannot_dodge.scn` |
| Two bullets on one bot in the same beat = 2 damage, two `Hit` events; the bot's `hit` state is the first one (shooter-ID order) | `bullet_two_hits_in_one_beat.scn` |
| HP never drops below 0; a bullet reaching a bot that already died this beat is spent with no `Hit` (no hits on a corpse), so exactly one `Killed` | `bullet_hp_never_below_zero.scn` |
| The victim's next Decide sees `Last.Hit`/`HitDir` for one beat only (a hit beats a bump) | `bullet_hit_state_is_none_without_a_hit.scn`; unit tests in `domain/sim/bullet_test.go` |
| HP 0 = death: `Hit`, then `Killed <killer> <victim>` (killer = shooter of the lethal bullet); the victim leaves the grid | `death_killed_event.scn`, `death_from_full_hp.scn` (4 hits at default HP) |
| Self-kill (walking into your own bullet): `Killed B B`; the scoring layer decides what it is worth | `death_self_kill.scn` |
| A dead bot is not blocking (cell is free), not hittable (bullets fly on), not scannable | `death_dead_bot_does_not_block.scn`, `death_dead_bot_is_not_hittable.scn`, `death_dead_bot_is_not_scannable.scn` |
| Respawn after `respawn_beats` (5): killed on beat t, `Respawned <bot> <x> <y>` at the end of beat t+5, acts from t+6 | `respawn_after_five_beats.scn` |
| Respawned bot: full HP, no bonus, loaded gun, `shield_beats` (2) of shield | `respawn_restores_hp_and_gun.scn` |
| Respawn spot = free cell (no bot, coin or bullet) with the max distance (Manhattan) to the nearest bot | `respawn_spot_max_min_distance.scn`, `respawn_spot_ignores_coins.scn` |
| Ties for the spot are broken by the seeded RNG (same seed = same cell, other seed may differ) | `respawn_tie_seed_1.scn` (x=1), `respawn_tie_seed_2.scn` (x=5) |
| Shield: bullets that reach a shielded bot are spent, no damage, no `Hit`, no interrupt; it lasts `shield_beats` beats after the respawn | `shield_blocks_hits.scn`, `shield_expires.scn`, `shield_after_respawn.scn`; length is a rule: `shield_beats_is_a_rule.scn` (3) |
| A shield does not stop the bot from moving, scanning or firing | `shield_does_not_stop_acting.scn` |
| Coin pickup: a bot that ends its move on a coin gets `CoinTaken <bot> <x> <y>` and +1 bonus shot; a bot that does not arrive on it does not | `coin_pickup_gives_bonus.scn`, `coin_pickup_needs_a_step.scn`, `coin_pickup_bump_does_not_take.scn` |
| A coin heals `coin_heal` (1) HP, never above max HP (`hp`, 4) | `coin_pickup_heals.scn`, `coin_pickup_heal_capped_at_max_hp.scn`, `coin_heal_is_a_rule.scn` |
| Beat order hits -> coins: a bot killed on the beat it steps onto a coin does not take it, the coin stays | `coin_pickup_dead_bot_does_not_take.scn` |
| Pickup at `bonus_max`: the coin is still consumed (it still scores), the bonus stays capped | `coin_pickup_at_bonus_cap.scn` |
| Coin spawn (seeded): every `coin_every` (50) beats one coin on a random free cell while fewer than `coin_max` (5) lie on the map (`CoinSpawned <x> <y>`) | `coin_spawn_seeded.scn`, `coin_spawn_other_seed.scn` |
| Map coins count towards `coin_max`; `coin_every=0` switches spawning off; a pickup frees room for the next spawn | `coin_spawn_counts_map_coins.scn`, `coin_spawn_off.scn`, `coin_spawn_refills_after_pickup.scn` |
| House bots: Rookie scans a random open direction and fires at level 3 (it ignores `loaded`, so it clicks into a reload) | `program_rookie_scans_then_fires.scn`; level 2 is ignored: `program_rookie_ignores_enemy_off_its_line.scn` |
| Lighthouse sees a bullet at level 3 and steps off the line before it arrives | `program_lighthouse_dodges_a_level3_bullet.scn` |
| Lighthouse fires at an enemy on its line when loaded | `program_lighthouse_fires_at_level3.scn` |
| Lighthouse `onHit`: sidestep first, then `num[2]` = shooter, `num[1]` = 4 hunting passes that scan `num[2]` | `program_lighthouse_hit_sidesteps_and_hunts.scn` |
| Tracker: enemy at level 2 -> sidestep; wrong side (reads 1) -> cross over with two moves, then fire | `program_tracker_crosses_over_after_a_wrong_sidestep.scn` |
| Tracker: right side (reads 3) -> fires at once, no crossing | `program_tracker_right_sidestep_fires.scn` |
| An endless loop = frozen bot: no action within the op budget, trail act IDLE every beat, the code keeps running (`num[0]` hits the 9999 clamp) | `program_infinite_loop_is_a_frozen_bot.scn` (program from `program_frozen_by_infinite_loop.js`) |
| Hot-swap: new code starts at a fresh `loop()`; memory, HP, reload and bonus are kept | `program_hot_swap_keeps_memory_hp_and_reload.scn` |
| Reboot (a saved program in a running game): the bot is off the map for K beats and returns on the respawn spot with a shield; HP, reload, bonus and memory are kept and the new program starts at a fresh `loop()` | `reboot_takes_the_bot_off_the_map_and_keeps_its_state.scn` (sim: `TestRebootEndsAfterNBeatsKeepingHPGunAndBonus`) |
| A rebooting bot cannot be hit or scanned and does not bump | `reboot_bot_is_not_hit_scanned_or_bumped.scn` |
| A rebooting bot's cell is free: another bot walks in without Bumped or Hit | `reboot_bot_walks_into_the_cell_the_rebooting_bot_left.scn` |
| A save during a reboot replaces the program and does not restart the countdown | `reboot_latest_save_wins_with_the_original_countdown.scn` |
| A dead bot's reboot ends at the later of its respawn and the reboot end, with full HP | `reboot_of_a_dead_bot_waits_for_the_later_of_respawn_and_reboot_end.scn`, `TestRebootOfADeadBotWaitsForTheLaterOfRespawnAndReboot` (sim) |
| No free cell = the bot waits off the map |  `TestRebootingBotWithNoFreeCellStaysOffTheMap` (sim) |
| Leave, kick or a new game cancels a reboot; detached bots and stopped games just store the program; house bots never reboot; the stay count runs on | `app/reboot_arena_test.go` |
| Memory: `num[0..7]` are eight independent cells; a computed index outside 0..7 reads 0 and writes nothing | `program_memory_has_eight_cells.scn` |
| A respawn starts a fresh `loop()` but keeps the memory | `program_respawn_keeps_memory.scn` |
| Places: `here()` after a move, `pin[i] == here()` compares cells, a bot returns to a pinned cell | `program_pin_here_returns_to_the_pinned_cell.scn` |
| Pins are kept through death and a hot-swap | `program_pins_survive_respawn_and_hot_swap.scn` |
| Navigation: `steps(pin)` counts the walk round a wall, `path(pin)` ties E before S, a bot reaches its pin in exactly `steps` beats | `program_path_walks_round_a_pillar_to_a_pin.scn` |
| `move` / `scan` / `fire` with a negative direction spend the beat as `wait()` (act WAIT); `move(path(p))` toward an unreachable pin stands still | `program_negative_direction_is_a_wait.scn` |
| `steps` / `path` are -1 for a sealed-off pin, a wall, my own cell (`path`) and `none`; `nearest()` skips unreachable pins | `program_path_is_minus_one_when_the_pin_is_sealed_off.scn` |
| Hot-swap keeps the scan readings (`enemy`, `last`) and the trail for the new code | `program_hot_swap_keeps_readings_and_trail.scn` |
| Forager: scans, sees a coin on its line, heads for it, keeps looking its way, takes it, then forgets the heading | `program_forager_scans_walks_and_takes_a_coin.scn` |
| Forager: an enemy one lane off while hp < 3 -> runs; healthy and loaded -> keeps walking | `program_forager_runs_from_a_close_enemy_when_weak.scn`, `program_forager_keeps_walking_when_healthy_and_loaded.scn` |
| Forager fires at level 3 when loaded | `program_forager_fires_on_a_clear_line_when_loaded.scn` |
| Hound: a heard shot -> alternates a look and a step toward it; without news it goes back to the all-round look | `program_hound_creeps_toward_a_heard_shot.scn` |
| Hound: level 3 -> pins the cell it stands on and fires | `program_hound_fires_on_level3_and_pins_the_sighting.scn` |
| Hound: after a sighting pin and a heard shot it creeps off, then with no news walks `path(pin[0])` back to the pin and resets `pin[0] = none` on arrival | `program_hound_walks_back_to_the_pinned_sighting.scn` |
| An edit at beat N acts on beat N; edits on one beat apply in file order | `program_hot_swap_takes_effect_at_its_beat.scn` |
| Long goldens (~300 beats: 2 bots 5x5, 4 bots 9x9 with a hot-swap, Rookie vs Lighthouse vs Tracker): Record JSON + pinned event-log hash in `engine/testdata/long`; `go test ./app -update` regenerates | `app/long_test.go` (`TestLongGoldenReplay`, part of `make determinism`) |

Tuned defaults (Step 19; evidence in `arena/TUNING.md`). Each pins its `Rules` default with `rules: default`, so changing the
default breaks exactly one scenario. Beat length (500 ms) and the arena (19x13) are server settings pinned in `cmd/arena` tests.

| Rule | Scenario(s) |
|---|---|
| Default `bullet_speed` is 3: a bullet fired at 6 cells is at +3 after beat 1 and hits on beat 2 | `defaults_bullet_speed.scn` |
| Default `reload` is 3: fire, click, click, fire | `defaults_reload.scn` |
| Default `cone_slope` is 100 (90 degree wedge): 2 along and 2 off reads level 1, 2 along and 3 off is outside | `defaults_cone.scn`, `defaults_cone_far.scn` (9 along, 10 off); the width is a rule: `scan_cone_slope_is_a_rule.scn` (50) |
| Default `coin_every` is 50 and `coin_max` is 5: coins on beats 50, 100, 150, 200 and 250, none between and none on 300 (the cap) | `defaults_coins.scn` |
| `wall(x, y)` is free and true for walls and cells outside the arena; in a walled 1-wide corridor a bot that sees `bullet == 3` retreats to a side opening and sidesteps the next beat, unhit | `program_wall_corridor_retreat_then_sidestep.scn` |
| The same retreat with the bullet one beat too close: the bot is still hit | `program_wall_corridor_retreat_too_late.scn` |
| `wall()` is terrain only: a floor cell with a bot on it is not a wall, a wall and an out-of-arena cell are | `program_wall_ignores_bots.scn` |
| Lighthouse/Tracker `wall()` dodge: sidestep to the open side; in a walled corridor back off along the line (`num[1] = -1` / `num[2] = 2` = sidestep pending) and sidestep next beat, unhit; too close is still a hit | `program_lighthouse_sidesteps_to_the_open_side.scn`, `program_lighthouse_corridor_retreats_then_sidesteps.scn`, `program_lighthouse_corridor_retreat_too_late.scn`, `program_tracker_corridor_retreats_then_sidesteps.scn` |
| Tracker turns before walking into a wall (`wall()` ahead), no bump | `program_tracker_turns_from_a_wall_without_bumping.scn` |
| Hearing: a real shot (loaded or bonus) by another bot within `hear_range` (Manhattan) is heard; `heard` is the cell of the shooter, from each side | `hear_from_north.scn`, `hear_from_east.scn`, `hear_from_south.scn`, `hear_from_west.scn` |
| Default `hear_range` is 6: 6 cells is heard, 7 is not (`heard` is none); the range is a rule, 0 = deaf world | `defaults_hear_range.scn`, `hear_range_is_a_rule.scn` (2), `hear_deaf_world.scn` |
| Walls do not block sound | `hear_through_a_wall.scn` |
| My own shots are not heard; a click is silent; a bonus shot is heard | `hear_own_shot_is_not_heard.scn`, `hear_click_is_silent.scn`, `hear_bonus_shot_is_heard.scn` |
| Two shooters: the nearest wins, equal distance goes to the lowest bot id | `hear_nearest_shooter_wins.scn`, `hear_equal_distance_goes_to_the_lowest_id.scn` |
| The heard cell uses positions after moves | `hear_uses_positions_after_moves.scn` |
| `heard` is kept on silent beats until a newer sound is heard, which replaces it | `hear_is_kept_on_silent_beats.scn`, `hear_newer_shot_replaces_the_old_one.scn` |
| Respawn resets `heard` to none | `hear_resets_on_respawn.scn` |
| A program that pins `heard` and walks `path(pin[0])` goes toward gunfire, then holds next to the shooter | `program_heard_walks_toward_a_duel.scn` |
