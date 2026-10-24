import { describe, expect, it } from "vitest";
import { initialState, reduce, type ViewModel } from "../../store/reducer";
import { fixture } from "../../testing/fixtures";
import { colorOf, FALLBACK_COLOR } from "./color";
import { buildScene, charge } from "./scene";
import { sidebarRows } from "./sidebar";
import { tracerBack } from "./tracers";

function play(name: string, upTo: number): ViewModel {
  return fixture(name).slice(0, upTo).reduce(reduce, initialState());
}
const grid = { width: 9, height: 3, rows: ["#########", "#.......#", "#########"] };

const withColors = (vm: ViewModel, colors: Record<string, string>): ViewModel => ({
  ...vm,
  bots: vm.bots.map((b) => ({ ...b, look: { ...b.look, color: colors[b.id] ?? b.look.color } })),
  fallen: vm.fallen.map((f) => ({ ...f, look: { ...f.look, color: colors[f.id] ?? f.look.color } })),
});

describe("colorOf", () => {
  it("is the look color of a live or fallen bot", () => {
    const vm = withColors(play("respawn_after_five_beats", 3), { A: "#010203", B: "#0a0b0c" });
    expect(vm.fallen.length).toBeGreaterThan(0);
    expect(colorOf(vm, vm.bots[0]!.id)).toBe(vm.bots[0]!.look.color);
    expect(colorOf(vm, vm.fallen[0]!.id)).toBe(vm.fallen[0]!.look.color);
  });
  it("falls back to grey for a bot the frame does not list", () => {
    expect(colorOf(play("move_free_cell", 1), "nobody")).toBe(FALLBACK_COLOR);
  });
});

describe("custom look colors reach the scene and the sidebar", () => {
  it("uses look.color for bots, radars, tracers and sidebar rows", () => {
    const vm = withColors(play("bullet_hits_stationary_target", 2), { A: "#123456", B: "#abcdef" });
    const s = buildScene(vm);
    expect(s.bots.find((b) => b.id === "A")?.color).toBe("#123456");
    expect(s.bots.find((b) => b.id === "B")?.color).toBe("#abcdef");
    expect(s.tracers.map((t) => t.color)).toEqual(["#123456"]);
    expect(sidebarRows(vm).map((r) => [r.id, r.color]).sort()).toEqual([["A", "#123456"], ["B", "#abcdef"]]);
  });
});

describe("tracerBack", () => {
  it("walks back up to n floor cells against the direction", () => {
    expect(tracerBack({ x: 4, y: 1 }, "E", 3, grid)).toEqual({ x: 1, y: 1 });
  });
  it("stops at a wall", () => {
    expect(tracerBack({ x: 2, y: 1 }, "E", 3, grid)).toEqual({ x: 1, y: 1 });
  });
});

describe("charge", () => {
  it("is empty right after a shot, fills each beat, full when loaded", () => {
    expect(charge({ loaded: false, cooldown: 2 }, 3)).toBe(0);
    expect(charge({ loaded: false, cooldown: 1 }, 3)).toBe(0.5);
    expect(charge({ loaded: true, cooldown: 0 }, 3)).toBe(1);
  });
  it("is full with a bonus shot or without a reload", () => {
    expect(charge({ loaded: true, cooldown: 2 }, 3)).toBe(1);
    expect(charge({ loaded: false, cooldown: 1 }, 1)).toBe(1);
  });
});

describe("buildScene taken coins", () => {
  it("keeps a taken coin with the taker id, apart from coins", () => {
    const vm = play("coin_pickup_gives_bonus", 999);
    const ev = vm.events.find((e) => e.type === "CoinTaken")!;
    const s = buildScene({ ...vm, beat: ev.beat });
    expect(s.taken).toEqual([{ bot: ev.args[0], x: Number(ev.args[1]), y: Number(ev.args[2]) }]);
    expect(s.coins.some((c) => c.x === s.taken[0]!.x && c.y === s.taken[0]!.y)).toBe(false);
  });
  it("has none on a beat without a pickup", () => {
    const vm = play("coin_pickup_gives_bonus", 1);
    expect(buildScene(vm).taken).toEqual([]);
  });
});

describe("buildScene", () => {
  it("is empty before the first message", () => {
    const s = buildScene(initialState());
    expect(s.width).toBe(0);
    expect(s.bots).toEqual([]);
  });

  it("lists walls, coins and bots with stable colors", () => {
    const vm = play("coin_pickup_gives_bonus", 1);
    const s = buildScene(vm);
    expect(s.walls.length).toBeGreaterThan(0);
    expect(s.coins).toEqual(vm.coins.map((c) => ({ x: c.x, y: c.y })));
    expect(s.bots[0]?.color).toBe(colorOf(vm, s.bots[0]!.id));
    expect(s.bots[0]).toMatchObject({ hp: vm.bots[0]!.hp, maxHp: 4, charge: 1 });
  });

  it("draws a scan as a radar on the scanning bot, facing the scan direction, without cones", () => {
    const vm = play("scan_enemy_level3", 2);
    const scan = vm.scans[0]!;
    const bot = vm.bots.find((b) => b.id === scan.bot)!;
    const s = buildScene(vm);
    expect(s.radars).toEqual([{ x: bot.x, y: bot.y, angle: { E: 0, S: Math.PI / 2, W: Math.PI, N: -Math.PI / 2 }[scan.dir], color: colorOf(vm, bot.id), sweep: 1 }]);
    expect(Object.keys(s)).toEqual(["beat", "width", "height", "walls", "coins", "taken", "radars", "tracers", "bots", "fallen", "rebooting", "effect"]);
  });

  it("flies a bullet the engine removed into the wall edge", () => {
    const vm = play("bullet_travels_three_cells_per_beat", 4); // beat 3, bullet gone
    const s = buildScene(vm);
    expect(s.tracers).toEqual([{ from: { x: 7, y: 1 }, to: { x: 7.5, y: 1 }, color: colorOf(vm, "A"), kind: "wall", arrive: 1 / 6 }]);
  });

  it("flies a shot fired next to a wall into it in the same beat", () => {
    const vm = play("bullet_travels_three_cells_per_beat", 2);
    const near = { ...vm, bullets: [], bots: vm.bots.map((b) => (b.id === "A" ? { ...b, x: 6 } : b)) };
    expect(buildScene(near).tracers).toEqual([{ from: { x: 6, y: 1 }, to: { x: 7.5, y: 1 }, color: colorOf(near, "A"), kind: "wall", arrive: 0.5 }]);
  });

  it("has no wall tracer for a bullet still flying or stopped by a bot", () => {
    expect(buildScene(play("bullet_travels_three_cells_per_beat", 3)).tracers.map((t) => t.kind)).toEqual(["flying"]);
    expect(buildScene(play("bullet_hits_stationary_target", 2)).tracers.map((t) => t.kind)).toEqual(["hit"]);
  });

  it("draws a flying bullet as a tracer from where it was a beat ago", () => {
    const vm = play("bullet_travels_three_cells_per_beat", 3); // beat 2
    const s = buildScene(vm);
    expect(s.tracers).toEqual([{ from: { x: 4, y: 1 }, to: { x: 7, y: 1 }, color: colorOf(vm, "A"), kind: "flying", arrive: 1 }]);
  });

  it("starts a just-fired tracer at the shooter", () => {
    const s = buildScene(play("bullet_travels_three_cells_per_beat", 2)); // beat 1, bullet at x=4
    expect(s.tracers[0]?.from).toEqual({ x: 1, y: 1 });
    expect(s.tracers[0]?.to).toEqual({ x: 4, y: 1 });
  });

  it("continues a flying bullet from where the previous beat's tracer ended", () => {
    const a = buildScene(play("bullet_travels_three_cells_per_beat", 2));
    const b = buildScene(play("bullet_travels_three_cells_per_beat", 3));
    expect(b.tracers[0]?.from).toEqual(a.tracers[0]?.to);
  });

  it("lands a hit after distance / speed of the beat, for the tracer and the victim", () => {
    const s = buildScene(play("bullet_hits_stationary_target", 2));
    const t = s.tracers.find((x) => x.kind === "hit")!;
    expect(t.arrive).toBeCloseTo(Math.hypot(t.to.x - t.from.x, t.to.y - t.from.y) / 3);
    expect(s.bots.find((b) => b.id === t.victim)?.landAt).toBe(t.arrive);
    expect(s.bots.find((b) => b.id !== t.victim)?.landAt).toBeUndefined();
  });

  it("lands the victim when the earliest of several bullets arrives", () => {
    const vm = play("bullet_hits_stationary_target", 2);
    const hit = vm.events.find((e) => e.type === "Hit")!;
    const near = { ...vm, bots: vm.bots.map((b) => (b.id === "A" ? { ...b, x: 3 } : b)) };
    const slow = { ...hit, by: "Z" };
    const s = buildScene({ ...near, events: [slow, ...near.events] });
    const arrivals = s.tracers.filter((x) => x.kind === "hit").map((x) => x.arrive);
    expect(arrivals[0]).toBeGreaterThan(arrivals[1]!);
    expect(s.bots.find((b) => b.id === hit.args[0])?.landAt).toBe(arrivals[1]);
  });

  it("lands a kill on the dying bot", () => {
    const s = buildScene(play("respawn_after_five_beats", 2));
    expect(s.fallen[0]?.landAt).toBe(s.tracers.find((x) => x.kind === "hit")?.arrive);
  });

  it("draws a hit as a tracer ending in the victim", () => {
    const msgs = fixture("bullet_hits_stationary_target");
    const idx = msgs.findIndex((m) => m.events.some((e) => e.type === "Hit"));
    const vm = play("bullet_hits_stationary_target", idx + 1);
    const hit = vm.events.find((e) => e.type === "Hit")!;
    const victim = vm.bots.find((b) => b.id === hit.args[0])!;
    const s = buildScene(vm);
    const t = s.tracers.find((x) => x.kind === "hit")!;
    expect(t.to).toEqual({ x: victim.x, y: victim.y });
    expect(t.color).toBe(colorOf(vm, hit.by!));
  });

  it("starts a hit tracer at the shooter when it is closer than a full flight", () => {
    const vm = play("bullet_hits_stationary_target", 2);
    const near = { ...vm, bots: vm.bots.map((b) => (b.id === "A" ? { ...b, x: 2 } : b)) };
    const t = buildScene(near).tracers.find((x) => x.kind === "hit")!;
    expect(t.from).toEqual({ x: 2, y: 1 });
    expect(t.to).toEqual({ x: 4, y: 1 });
  });

  it("starts the hit tracer at the shooter cell in the fixture", () => {
    const t = buildScene(play("bullet_hits_stationary_target", 2)).tracers.find((x) => x.kind === "hit")!;
    expect(t.from).toEqual({ x: 1, y: 1 });
  });

  it("starts a hit tracer at a bullet's previous position when it was already in flight", () => {
    const vm = play("bullet_hits_stationary_target", 2);
    const victim = vm.bots.find((b) => b.id === "B")!;
    const prev = { x: victim.x - 1, y: victim.y, dir: "E", owner: "A" };
    const t = buildScene({ ...vm, prevBullets: [prev] }).tracers.find((x) => x.kind === "hit")!;
    expect(t.from).toEqual({ x: prev.x, y: prev.y });
    expect(t.arrive).toBeCloseTo(1 / 3);
  });

  it("has no hit tracer a beat after the hit", () => {
    const vm = play("bullet_hits_stationary_target", 2);
    const later = { ...vm, beat: vm.beat + 1 };
    expect(buildScene(later).tracers).toEqual([]);
  });

  it("marks a bot hit this beat, and only this beat", () => {
    const vm = play("bullet_hits_stationary_target", 2);
    expect(buildScene(vm).bots.map((b) => [b.id, b.hit])).toEqual([["A", false], ["B", true]]);
    expect(buildScene({ ...vm, beat: vm.beat + 1 }).bots.every((b) => !b.hit)).toBe(true);
  });

  it("takes a Kicked event in the feed without touching the scene", () => {
    const vm = play("respawn_after_five_beats", 2);
    const kicked = { ...vm, events: [...vm.events, { beat: vm.beat, type: "Kicked", args: ["B"] }] };
    expect(buildScene(kicked).fallen).toEqual(buildScene(vm).fallen);
  });

  it("marks a bot killed this beat as dying where it died", () => {
    const vm = play("respawn_after_five_beats", 2);
    expect(buildScene(vm).fallen).toMatchObject([{ id: "B", x: 4, y: 1, dying: true }]);
    expect(buildScene({ ...vm, beat: vm.beat + 1 }).fallen[0]?.dying).toBe(false);
  });

  it("is a static scene with effects done", () => {
    expect(buildScene(play("move_free_cell", 2)).effect).toBe(1);
  });

  it("marks a freshly respawned bot as shielded", () => {
    const s = buildScene(play("respawn_after_five_beats", 7));
    expect(s.bots.find((b) => b.id === "B")?.shielded).toBe(true);
    expect(s.bots.find((b) => b.id === "A")?.shielded).toBe(false);
  });

  it("marks respawning bots", () => {
    const msgs = fixture("respawn_after_five_beats");
    const idx = msgs.findIndex((m) => m.frame.fallen.length > 0);
    const s = buildScene(play("respawn_after_five_beats", idx + 1));
    expect(s.fallen[0]?.label).toMatch(/^\d+$/);
  });

  it("shows the pencil only briefly after a hot-swap", () => {
    const name = "program_hot_swap_keeps_memory_hp_and_reload";
    const idx = fixture(name).findIndex((m) => m.frame.bots.some((b) => b.editedBeat > 0));
    expect(idx).toBeGreaterThanOrEqual(0);
    const vm = play(name, idx + 1);
    expect(buildScene(vm).bots.some((b) => b.edited)).toBe(true);
    expect(buildScene({ ...vm, beat: vm.beat + 50 }).bots.some((b) => b.edited)).toBe(false);
  });

});

describe("sidebarRows", () => {
  it("follows the board order and adds hp", () => {
    const vm = play("duel_5x5", 3);
    const rows = sidebarRows(vm);
    expect(rows.map((r) => r.id)).toEqual(vm.board.map((e) => e.bot));
    expect(rows[0]).toMatchObject({ hp: 4, maxHp: 4, kills: 0, deaths: 0, score: 0 });
  });

  it("keeps fallen bots in the table with hp 0", () => {
    const msgs = fixture("respawn_after_five_beats");
    const idx = msgs.findIndex((m) => m.frame.fallen.length > 0);
    const rows = sidebarRows(play("respawn_after_five_beats", idx + 1));
    expect(rows.some((r) => r.hp === 0)).toBe(true);
  });

  it("carries the board coins and shows 0 for bots without a board entry", () => {
    const vm = play("duel_5x5", 2);
    const [a, b] = vm.bots;
    const withCoins: ViewModel = { ...vm, board: [{ bot: a!.id, name: a!.name, score: 1, kills: 0, deaths: 0, coins: 7 }] };
    const rows = sidebarRows(withCoins);
    expect(rows.find((r) => r.id === a!.id)?.coins).toBe(7);
    expect(rows.find((r) => r.id === b!.id)?.coins).toBe(0);
  });

  it("appends present bots missing from the board and drops absent board entries", () => {
    const vm = play("duel_5x5", 2);
    const odd: ViewModel = { ...vm, board: [{ bot: "ghost", name: "g", score: 9, kills: 0, deaths: 0, coins: 0 }] };
    const rows = sidebarRows(odd);
    expect(rows.map((r) => r.id)).toEqual(vm.bots.map((b) => b.id));
    expect(rows.every((r) => r.score === 0)).toBe(true);
  });

  it("flags my bot", () => {
    const vm = { ...play("scan_enemy_level3", 2), myBotId: "A" };
    const rows = sidebarRows(vm);
    expect(rows.find((r) => r.id === "A")?.mine).toBe(true);
    expect(rows.find((r) => r.id === "B")?.mine).toBe(false);
  });
});

describe("faces in the scene", () => {
  const vm = play("bullet_hits_stationary_target", 2);
  const withEvents = (events: ViewModel["events"], beat = vm.beat) => ({ ...vm, beat, events });
  const face = (v: ViewModel, id: string) => { const b = buildScene(v).bots.find((x) => x.id === id)!; return [b.eyes, b.mouth]; };

  it("carries the body and hat of the look and shows the resting face", () => {
    const rest = buildScene(withEvents([])).bots.find((b) => b.id === "A")!;
    const look = vm.bots.find((b) => b.id === "A")!.look;
    expect([rest.body, rest.hat, rest.eyes, rest.mouth]).toEqual([look.body, look.hat, look.eyes, look.mouth]);
  });
  it("winces on the beat of a hit only", () => {
    expect(face(vm, "B")).toEqual(["squint", "o"]);
    expect(face(withEvents(vm.events, vm.beat + 1), "B")).not.toEqual(["squint", "o"]);
  });
  it("grins for two beats after a kill", () => {
    const killed = [{ beat: vm.beat, type: "Killed", args: ["A", "B"] }];
    expect(face(withEvents(killed), "A")).toEqual(["wide", "grin"]);
    expect(face(withEvents(killed, vm.beat + 1), "A")).toEqual(["wide", "grin"]);
    expect(face(withEvents(killed, vm.beat + 2), "A")).not.toEqual(["wide", "grin"]);
  });
  it("fallen bots wear x eyes and a flat mouth", () => {
    const dead = play("respawn_after_five_beats", 3);
    expect(dead.fallen.length).toBeGreaterThan(0);
    const f = buildScene(dead).fallen[0]!;
    expect([f.eyes, f.mouth]).toEqual(["x", "flat"]);
  });
});

describe("rebooting bots", () => {
  const rebooting = { bot: "A", name: "Ann", look: { color: "#123456", body: "circle", eyes: "dots", mouth: "smile", hat: "none" }, cell: { x: 2, y: 1 }, hp: 3, beatsLeft: 4, total: 10 };
  const vm = (): ViewModel => {
    const base = play("bullet_hits_stationary_target", 2);
    return { ...base, bots: base.bots.filter((b) => b.id !== "A"), rebooting: [rebooting] };
  };

  it("draws the effect at the cell the bot left, in its color, with the fraction of the reboot that has passed", () => {
    const s = buildScene(vm());
    expect(s.rebooting).toEqual([{ id: "A", x: 2, y: 1, color: "#123456", progress: 0.6 }]);
    expect(s.bots.map((b) => b.id)).not.toContain("A");
  });

  it("fills the bar from empty to full as the beats left go down, and clamps odd values", () => {
    const at = (beatsLeft: number, total = 10) => buildScene({ ...vm(), rebooting: [{ ...rebooting, beatsLeft, total }] }).rebooting[0]!.progress;
    expect([at(10), at(5), at(1), at(0), at(12), at(3, 0)]).toEqual([0, 0.5, 0.9, 1, 0, 1]);
  });

  it("keeps the effect in its cell even when another bot stands there", () => {
    const base = vm();
    const other = { ...base.bots[0]!, x: 2, y: 1 };
    const s = buildScene({ ...base, bots: [other] });
    expect(s.rebooting[0]).toMatchObject({ x: 2, y: 1 });
    expect(s.bots[0]).toMatchObject({ x: 2, y: 1 });
  });

  it("lists a rebooting bot in the sidebar with a label, after the live ones", () => {
    const rows = sidebarRows(vm());
    const row = rows.find((r) => r.id === "A");
    expect(row).toMatchObject({ name: "Ann", rebooting: true, hp: 3, color: "#123456" });
    expect(rows.filter((r) => r.rebooting)).toHaveLength(1);
  });

  it("is empty for the empty scene", () => {
    expect(buildScene({ ...vm(), grid: null }).rebooting).toEqual([]);
  });
});
