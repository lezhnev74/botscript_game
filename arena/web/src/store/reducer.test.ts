import { describe, expect, it } from "vitest";
import type { Message } from "../protocol/protocol";
import { fixture } from "../testing/fixtures";
import { initialState, reduce, setMyBot, type ViewModel } from "./reducer";

/** Folds the first n messages (all by default) of a fixture. */
function play(name: string, n?: number, from: ViewModel = initialState()): ViewModel {
  return fixture(name).slice(0, n).reduce(reduce, from);
}

describe("initial state", () => {
  it("is empty and unsynced", () => {
    const s = initialState();
    expect(s).toMatchObject({ synced: false, beat: 0, bots: [], bullets: [], coins: [], scans: [], events: [], board: [], myBotId: null });
    expect(s.grid).toBeNull();
  });
});

describe("snapshot", () => {
  it("sets grid, bots and board with no scans or events", () => {
    const s = play("bullet_hits_stationary_target", 1);
    expect(s.synced).toBe(true);
    expect(s.beat).toBe(0);
    expect(s.grid).toEqual({ width: 7, height: 4, rows: ["#######", "#.....#", "#.....#", "#######"] });
    expect(s.bots.map((b) => [b.id, b.hp, b.x, b.y])).toEqual([["A", 4, 1, 1], ["B", 4, 4, 1]]);
    expect(s.events).toEqual([]);
    expect(s.scans).toEqual([]);
    expect(s.board.map((e) => e.bot)).toEqual(["A", "B"]);
    expect(s.maxHp).toBe(4);
    expect(s.reload).toBe(3);
  });

  it("after a reconnect replaces the world and the feed but keeps my bot", () => {
    const mid = setMyBot(play("respawn_after_five_beats", 4), "B");
    expect(mid.events.length).toBeGreaterThan(0);
    const again = reduce(mid, fixture("move_free_cell")[0]!);
    expect(again.beat).toBe(0);
    expect(again.events).toEqual([]);
    expect(again.bots.map((b) => b.id)).toEqual(["A"]);
    expect(again.myBotId).toBe("B");
  });
});

describe("beats", () => {
  it("moves a bot and advances the beat", () => {
    const before = play("move_free_cell", 1);
    const after = play("move_free_cell", 2);
    expect(after.beat).toBe(1);
    const moved = after.events.find((e) => e.type === "Moved")!;
    const id = moved.args[0]!;
    const a = before.bots.find((b) => b.id === id)!;
    const b = after.bots.find((x) => x.id === id)!;
    expect(a.x !== b.x || a.y !== b.y).toBe(true);
  });

  it("shows a hit: hp drops and the event is in the feed", () => {
    const s = play("bullet_hits_stationary_target");
    expect(s.bots.find((b) => b.id === "B")!.hp).toBe(3);
    expect(s.events.some((e) => e.type === "Hit" && e.by === "A")).toBe(true);
  });

  it("keeps the previous beat's bullets, none after a snapshot", () => {
    expect(play("bullet_travels_three_cells_per_beat", 1).prevBullets).toEqual([]);
    expect(play("bullet_travels_three_cells_per_beat", 3).prevBullets).toEqual([{ x: 4, y: 1, dir: "E", owner: "A" }]);
  });

  it("keeps scan x-ray records for this beat only", () => {
    const scanned = play("scan_enemy_level3", 2);
    expect(scanned.scans).toHaveLength(1);
    const scan = scanned.scans[0]!;
    expect(scan.bot).toBe("A");
    expect(scan.dir).toBe("E");
    expect(scan.cells.length).toBeGreaterThan(0);
    expect(scan.seen).toEqual([{ kind: "enemy", x: 3, y: 2, level: 3 }]);
    const quiet: Message = { ...fixture("scan_enemy_level3")[1]!, events: [] };
    expect(reduce(scanned, quiet).scans).toEqual([]);
  });

  it("shows a death as a fallen bot and a respawn after the wait", () => {
    const died = play("respawn_after_five_beats", 2);
    expect(died.fallen.map((f) => f.id)).toEqual(["B"]);
    expect(died.bots.some((b) => b.id === "B")).toBe(false);
    const back = play("respawn_after_five_beats");
    expect(back.fallen).toEqual([]);
    expect(back.bots.map((b) => b.id)).toContain("B");
  });

  it("updates the leaderboard from killed events", () => {
    const s = play("respawn_after_five_beats");
    const killer = s.board.find((e) => e.kills === 1)!;
    expect(killer.bot).toBe("A");
    expect(s.board.find((e) => e.bot === "B")!.deaths).toBe(1);
  });

  it("tracks coins on the map and picks them up", () => {
    const start = play("coin_pickup_gives_bonus", 1);
    expect(start.coins.length).toBe(1);
    const end = play("coin_pickup_gives_bonus");
    expect(end.coins).toEqual([]);
    expect(end.events.some((e) => e.type === "CoinTaken")).toBe(true);
    expect(end.bots.some((b) => b.bonus > 0)).toBe(true);
  });

  it("carries the program badge: num, line, edited beat", () => {
    const s = play("program_hot_swap_keeps_memory_hp_and_reload");
    expect(s.bots.some((b) => b.editedBeat > 0)).toBe(true);
    expect(s.bots.every((b) => b.num.length === 8)).toBe(true);
  });

  it("feeds bullets in flight", () => {
    const s = play("bullet_travels_three_cells_per_beat", 2);
    expect(s.bullets).toEqual([{ x: 4, y: 1, dir: "E", owner: "A" }]);
    expect(play("bullet_travels_three_cells_per_beat", 3).bullets[0]!.x).toBe(7);
  });

  it("caps the events feed, newest last", () => {
    const s = play("duel_5x5");
    expect(s.beat).toBe(300);
    expect(s.events.length).toBeLessThanOrEqual(100);
    const beats = s.events.map((e) => e.beat);
    expect(beats).toEqual([...beats].sort((a, b) => a - b));
  });

  it("is pure: the input state is not changed", () => {
    const s0 = play("bullet_hits_stationary_target", 1);
    const copy = structuredClone(s0);
    reduce(s0, fixture("bullet_hits_stationary_target")[1]!);
    expect(s0).toEqual(copy);
  });
});

describe("a new game on another map", () => {
  // The server restarts the beat count at 1 and may change the world size between two frames.
  const bigger = (m: Message): Message => ({
    ...m, frame: { ...m.frame, width: 9, height: 6, grid: ["#########", "#.......#", "#.......#", "#.......#", "#.......#", "#########"], bullets: [] },
  });

  it("takes the new grid at once, with nothing left of the old world", () => {
    const old = play("bullet_travels_three_cells_per_beat", 3);
    expect(old.bullets.length).toBeGreaterThan(0);
    const first = bigger({ ...fixture("move_free_cell")[1]!, events: [] });
    const s = reduce(old, first);
    expect(s.grid).toEqual({ width: 9, height: 6, rows: first.frame.grid });
    expect(s.beat).toBe(first.frame.beat);
    expect(s.bots.map((b) => b.id)).toEqual(first.frame.bots.map((b) => b.id));
    expect(s.prevBullets).toEqual([]);
    expect(s.fallen).toEqual(first.frame.fallen);
  });

  it("restarts the feed and forgets the old bullets when the beat count goes back", () => {
    const old = setMyBot(play("bullet_travels_three_cells_per_beat", 3), "A");
    const again = reduce(old, fixture("move_free_cell")[1]!);
    expect(again.beat).toBe(1);
    expect(again.prevBullets).toEqual([]);
    const own = fixture("move_free_cell")[1]!.events;
    expect(own.length).toBeGreaterThan(0);
    expect(again.events).toEqual(own);
    expect(again.myBotId).toBe("A");
  });

  it("restarts the feed when only the size changes", () => {
    const old = play("respawn_after_five_beats", 4);
    const next = bigger({ ...fixture("respawn_after_five_beats")[4]!, events: [] });
    expect(reduce(old, next).events).toEqual([]);
  });

  it("keeps the feed across ordinary beats", () => {
    const s = play("respawn_after_five_beats", 4);
    expect(s.events.length).toBeGreaterThan(0);
    expect(reduce(s, fixture("respawn_after_five_beats")[4]!).events.length).toBeGreaterThanOrEqual(s.events.length);
  });
});

describe("my bot", () => {
  it("is set and found in the world", () => {
    const s = setMyBot(play("bullet_hits_stationary_target"), "B");
    expect(s.myBotId).toBe("B");
  });
});

describe("game timer", () => {
  const msg = (patch: Partial<Message>): Message => ({ ...fixture("move_free_cell")[1]!, ...patch });

  it("starts with no timer and no game over", () => {
    expect(initialState()).toMatchObject({ remaining: -1, gameOver: false, beatMs: 0 });
  });
  it("follows the remaining beats and the beat length of each message", () => {
    const s = reduce(initialState(), msg({ remaining: 59, beatMs: 1000 }));
    expect(s).toMatchObject({ remaining: 59, beatMs: 1000, gameOver: false });
  });
  it("shows game over with the final board from the last beat and from a later snapshot", () => {
    const board = [{ bot: "A", name: "Ann", score: 3, kills: 1, deaths: 0, coins: 0 }];
    const over = reduce(initialState(), msg({ remaining: 0, gameOver: true, board }));
    expect(over).toMatchObject({ gameOver: true, remaining: 0, board });
    const snap = reduce(initialState(), { ...msg({ remaining: 0, gameOver: true, board }), type: "snapshot", events: [] });
    expect(snap).toMatchObject({ gameOver: true, board });
  });
  it("clears game over when the next game's first beat arrives", () => {
    const over = reduce(initialState(), msg({ remaining: 0, gameOver: true }));
    const next = reduce(over, msg({ remaining: -1, gameOver: false }));
    expect(next).toMatchObject({ gameOver: false, remaining: -1 });
  });
});

describe("looks", () => {
  it("keeps each bot's look from the roster, live and fallen", () => {
    const live = play("respawn_after_five_beats", 1);
    expect(live.bots.every((b) => /^#[0-9a-f]{6}$/i.test(b.look.color) && b.look.body !== "")).toBe(true);
    const dead = play("respawn_after_five_beats", 3);
    const f = dead.fallen[0]!;
    expect(f.look).toEqual(live.bots.find((b) => b.id === f.id)!.look);
  });
});

describe("rebooting bots", () => {
  const rebooting = { bot: "A", name: "Ann", look: { color: "#123456", body: "circle", eyes: "dots", mouth: "smile", hat: "none" }, cell: { x: 2, y: 1 }, hp: 3, beatsLeft: 4, total: 10 };

  it("keeps the list of the frame, empty when nobody reboots", () => {
    const msgs = fixture("move_free_cell");
    expect(msgs.reduce(reduce, initialState()).rebooting).toEqual([]);
    const withReboot = { ...msgs[1]!, frame: { ...msgs[1]!.frame, rebooting: [rebooting] } };
    expect(reduce(initialState(), withReboot).rebooting).toEqual([rebooting]);
  });
});
