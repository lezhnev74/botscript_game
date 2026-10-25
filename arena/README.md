# Arena

## Docker

Build (from `arena/`; target size is under 20 MB):

    make docker                      # docker build -t arena .

Run, choosing the host port at run time; scores persist in the `arena-data` volume:

    make docker-run PORT=8080        # docker run --rm -p 8080:8080 -v arena-data:/data -e ARENA_ADMIN_PASSWORD -e ARENA_BASE_URL arena

The container runs as uid 65532 on a scratch image and listens on 8080 inside. `docker stop` shuts it down cleanly
(SIGTERM). The stdin console is unavailable without `-i`; the server keeps running regardless.

At most 50 human players may join (`-max-players`, 0 = as many as the room has floor cells; house bots do not count).
More joins answer 409 "arena is full" until somebody is kicked or leaves.

A player can take the bot out of the arena and bring it back: on `/bot` the "Leave the arena" button detaches it (it
leaves the map and the TV, stops counting for `-max-players`; its token, name, look, program and leaderboard line are
kept) and "Return to Arena" puts it back at the next beat (409 when the arena is full). The routes are
`POST /api/bot/leave` and `POST /api/bot/return` (Bearer token, 204; 429 with `retryBeats` for more than one change per
3 beats). Every stay is limited by `-max-stay` (Go duration, default `24h`, `0` = no limit, counted in beats of the
match, so a stopped game does not use it up): then the bot detaches by itself and can return at once. `GET /api/bot` reports
`arena: {in, leftBy, stayLeftBeats}` and `beatMs`. The stay count is saved in `bots.json` at least every 60 beats and at
shutdown. The admin lists detached bots (and can kick them); the console `list` marks them `detached`.

Saving a new program (`PUT /api/bot`, 204) reboots the bot: while it is in a running game it leaves the map for `-reboot`
(Go duration, default `10s`, rounded up to whole beats, `0` = off) and comes back on the respawn spot running the new
program, keeping HP, gun state, memory and score. Saves during a reboot replace the program without restarting the
countdown, and there is no rate limit on saves (with `-reboot 0` a save is a hot-swap at the next beat, one per 3 beats).
`GET /api/bot` reports `rebootLeftBeats` (-1 = not rebooting, 0 = about to return) and the WS frames list `rebooting: [{bot, cell, hp, beatsLeft, total}]`; `rebootTotalBeats` is the full length. With the reboot on, a bot that left by its own Leave cannot return (429) until `-reboot` has passed since the leave.

Behind a reverse proxy, pass `-trust-proxy <address or CIDR>` (repeatable) for it: the per-IP limits then use the
client address from its `X-Forwarded-For` (the rightmost entry that is not a trusted proxy), and its
`X-Forwarded-Proto: https` makes the admin cookie `Secure`. Without the flag those headers are ignored.

## Program check for AI

`POST /api/check` takes a BOTSCRIPT program as the raw body, `GET /api/check?src=<url-encoded program>` takes it as a
parameter (for chat tools that can only fetch URLs). No token needed; 64 KB at most; 30 requests per minute per IP,
then 429 (every request counts, also the ones answered 400 or 413; at most 4 trials run at once, more get 503 with
`Retry-After: 1`). The answer is JSON: `{"ok": false, "errors": [{line, col, message, hint}]}` for an invalid program, or
`{"ok": true, "trial": {...}}` for a valid one. The trial plays 100 beats of the program against three house bots in the
small built-in 9x7 room (fixed seed, no effect on the live arena, same program = same answer) and reports the actions by
kind, missed beats, clicks, hits, kills, deaths, coins and plain-words `warnings`.

    curl -s --data-binary @bot.js http://localhost:8080/api/check
    curl -s -G http://localhost:8080/api/check --data-urlencode 'src=function loop() { scan(E); }'

Settings (`ARENA_*`) come from the environment or from a dotenv file (`-env`, default `.env` in the working
directory; `make run` and `make docker-run` use `arena/.env`). Real environment variables win over the file. The file
holds `KEY=VALUE` lines (`#` comments, optional `export ` and quotes); it is git-ignored.

Set `ARENA_BASE_URL` (e.g. `https://arena.example.com`; trailing slash trimmed; optional) to the public address of the
server. `GET /api/config` returns it as `{"baseUrl": ...}`, and "Copy rulebook for AI" puts it into the check URLs
of the rulebook (without it the page's own address is used; for a private address such as localhost the rulebook tells
the AI it may not be able to reach it).

## Admin

Set `ARENA_ADMIN_PASSWORD` (the env var is read once at start and never logged) to turn on the web admin at `/admin`;
unset or empty means disabled (`/api/admin/*` answers 404). Log in with the password: the session is an `HttpOnly;
SameSite=Strict` cookie that lasts 12 hours and is forgotten on restart. It is `Secure` only when the client came over HTTPS
(directly or through a `-trust-proxy`), so plain http on a LAN keeps working. Wrong passwords wait half a second, and 5 failures per minute per IP end in 429.

The page lists the bots (a Kick button on human bots), sets how many house bots fill the arena, and resets the
leaderboard. House bots are not kickable; lower the house-bot count instead. The stdin console (`make run`) has the
same powers: `list`, `kick <id|name>`, `reset-board`, `min-bots <n>`, `timer <minutes>`, `start`, `help`.

The Game box sets a timer in minutes (decimals allowed, Cancel removes it). When it runs out the game freezes: the TV
shows "Game over" with the final board, no beats are played, and that board becomes the "Last game" table on
`/leaderboard` (kept in `last.json`, replaced by the next finished game). Players can still join, edit and be kicked;
those apply when you press "Start new game" (empty live board, same players and programs, no timer, seed + 1). Start
answers 409 while a game runs. Edits made while stopped skip the swap rate limit (the last one wins), and Reset
leaderboard is refused (409, button disabled) because the start empties the board anyway. The timer is not saved: after a server restart the game runs with no timer.

The Map box makes a new room: type a width (9 to 41), a height (7 to 31) and the walls in percent of the inside (0 to 15, default 3), press Generate for a preview (a wall
border plus a few random walls, always connected), Regenerate for another, Apply to play it. Apply starts a new game
at the next beat: same players, tokens and programs, fresh positions, the leaderboard is kept, the beat count restarts
at 1 and the seed goes up by one. It is refused when the floor has fewer cells than there are bots (or house bots to
keep). The applied map is saved as `map.txt` in the data dir and used at the next start; the `-map` flag wins over it.
Delete `map.txt` to go back to the built-in room. A stored `map.txt` with fewer floor cells than `-min-bots` stops the
startup with an error until you delete it or lower `-min-bots`.

    ARENA_ADMIN_PASSWORD=secret make run
