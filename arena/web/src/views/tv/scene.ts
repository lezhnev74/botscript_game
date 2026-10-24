import { faceOf, type Reactions } from "../../render/face";
import type { BotSprite, FallenSprite, Pt, RadarMark, RebootSprite, Scene, TakenCoin, TracerLine } from "../../render/scene";
import type { ViewModel } from "../../store/reducer";
import { colorOf } from "./color";
import { BULLET_SPEED, onTrace, opposite, tracerBack, wallStop } from "./tracers";

/** How many beats the pencil stays after a hot-swap. */
export const PENCIL_BEATS = 8;

type Event = ViewModel["events"][number];
type Grid = NonNullable<ViewModel["grid"]>;

const pt = (p: Pt): Pt => ({ x: p.x, y: p.y });

function wallsOf(g: Grid): Pt[] {
  const out: Pt[] = [];
  g.rows.forEach((row, y) => [...row].forEach((ch, x) => { if (ch === "#") out.push({ x, y }); }));
  return out;
}

const thisBeat = (vm: ViewModel): Event[] => vm.events.filter((e) => e.beat === vm.beat);

/**
 * Gun charge 0..1. After a shot the cooldown reads reload-1 (the reload phase already counted that beat), so it
 * runs reload-1 .. 0: empty right after firing, full when ready. A stored bonus shot means ready too.
 */
export function charge(b: Pick<ViewModel["bots"][number], "loaded" | "cooldown">, reload: number): number {
  if (b.loaded || reload <= 1) return 1;
  return Math.max(0, 1 - b.cooldown / (reload - 1));
}

type Landings = Map<string, number>;

/** Beat fraction a bullet needs to cover `from -> to` at BULLET_SPEED, kept within (0, 1]. */
export function arrival(from: Pt, to: Pt): number {
  return Math.min(1, Math.max(0.05, Math.hypot(to.x - from.x, to.y - from.y) / BULLET_SPEED));
}

const landAt = (l: Landings, id: string): { landAt?: number } => (l.has(id) ? { landAt: l.get(id) } : {});

type Looked = ViewModel["bots"][number] | ViewModel["fallen"][number];

/** The sprite fields of a bot's look and its current face: wince on a hit, grin after a kill, x eyes when fallen. */
function figureOf(vm: ViewModel, b: Looked, r: Reactions): Pick<BotSprite, "body" | "hat" | "eyes" | "mouth"> {
  return { body: b.look.body, hat: b.look.hat, ...faceOf(b.look, r, vm.beat) };
}

const latest = (vm: ViewModel, type: string, at: number, id: string): number | undefined =>
  vm.events.filter((e) => e.type === type && e.args[at] === id).reduce<number | undefined>((m, e) => Math.max(m ?? 0, e.beat), undefined);

function botSprites(vm: ViewModel, events: Event[], land: Landings): BotSprite[] {
  return vm.bots.map((b) => ({
    id: b.id, x: b.x, y: b.y, color: colorOf(vm, b.id), name: b.name,
    edited: b.editedBeat > 0 && vm.beat - b.editedBeat < PENCIL_BEATS,
    shielded: b.shield > 0, mine: b.id === vm.myBotId,
    hit: events.some((e) => e.type === "Hit" && e.args[0] === b.id),
    hp: b.hp, maxHp: vm.maxHp, charge: charge(b, vm.reload), ...landAt(land, b.id),
    ...figureOf(vm, b, { dead: false, shielded: b.shield > 0, hitBeat: latest(vm, "Hit", 0, b.id), killBeat: latest(vm, "Killed", 0, b.id) }),
  }));
}

function fallenSprites(vm: ViewModel, events: Event[], land: Landings): FallenSprite[] {
  return vm.fallen.map((f) => ({
    id: f.id, x: f.x, y: f.y, color: colorOf(vm, f.id), name: f.name, label: `${f.wait}`,
    dying: events.some((e) => e.type === "Killed" && e.args[1] === f.id), ...landAt(land, f.id),
    ...figureOf(vm, f, { dead: true }),
  }));
}

function flyingTracers(vm: ViewModel, g: Grid, events: Event[]): TracerLine[] {
  return vm.bullets.map((b) => {
    const fired = events.some((e) => e.type === "Fired" && e.args[0] === b.owner && e.args[1] === b.dir);
    const shooter = vm.bots.find((x) => x.id === b.owner);
    const back = tracerBack(b, b.dir, BULLET_SPEED, g);
    const from = fired && shooter && onTrace(shooter, b, b.dir, BULLET_SPEED) ? pt(shooter) : back;
    return { from, to: pt(b), color: colorOf(vm, b.owner), kind: "flying" as const, arrive: 1 };
  });
}

function hitTracers(vm: ViewModel, g: Grid, events: Event[]): TracerLine[] {
  return events.filter((e) => e.type === "Hit").flatMap((e) => {
    const victim = [...vm.bots, ...vm.fallen].find((x) => x.id === e.args[0]);
    if (!victim) return [];
    const travel = opposite(e.args[1] ?? "");
    const shooter = vm.bots.find((x) => x.id === e.by);
    const back = tracerBack(victim, travel, BULLET_SPEED, g);
    const inFlight = vm.prevBullets.find((b) => b.owner === e.by && b.dir === travel && onTrace(b, victim, travel, BULLET_SPEED) && (b.x !== victim.x || b.y !== victim.y));
    const from = inFlight ? pt(inFlight) : shooter && onTrace(shooter, victim, travel, BULLET_SPEED) ? pt(shooter) : back;
    return [{ from, to: pt(victim), color: colorOf(vm, e.by ?? ""), kind: "hit" as const, arrive: arrival(from, victim), victim: victim.id }];
  });
}

/**
 * Bullets the engine removed at a wall this beat: last beat's bullets and this beat's shots that meet a wall
 * within one flight. Ones stopped by a bot are drawn by hitTracers (or vanish on a shield).
 */
function wallTracers(vm: ViewModel, g: Grid, events: Event[]): TracerLine[] {
  const shots = events.filter((e) => e.type === "Fired").flatMap((e) => {
    const shooter = [...vm.bots, ...vm.fallen].find((b) => b.id === e.args[0]);
    return shooter ? [{ x: shooter.x, y: shooter.y, dir: e.args[1] ?? "", owner: shooter.id, self: shooter.id }] : [];
  });
  const sources = [...vm.prevBullets.map((b) => ({ ...b, self: "" })), ...shots];
  return sources.flatMap((b) => {
    const occupied = (p: Pt) => [...vm.bots, ...vm.fallen].some((x) => x.id !== b.self && x.x === p.x && x.y === p.y);
    const end = wallStop(b, b.dir, BULLET_SPEED, g, occupied);
    return end ? [{ from: pt(b), to: end, color: colorOf(vm, b.owner), kind: "wall" as const, arrive: arrival(b, end) }] : [];
  });
}

const ANGLE: Record<string, number> = { E: 0, S: Math.PI / 2, W: Math.PI, N: -Math.PI / 2 };

function radars(vm: ViewModel): RadarMark[] {
  return vm.scans.flatMap((s) => {
    const b = vm.bots.find((x) => x.id === s.bot);
    const angle = ANGLE[s.dir];
    return b && angle !== undefined ? [{ x: b.x, y: b.y, angle, color: colorOf(vm, b.id), sweep: 1 }] : [];
  });
}

/** Coins taken this beat, from CoinTaken events (the frame no longer lists them in coins). */
function takenCoins(events: Event[]): TakenCoin[] {
  return events.filter((e) => e.type === "CoinTaken").map((e) => ({ bot: e.args[0] ?? "", x: Number(e.args[1]), y: Number(e.args[2]) }));
}

/** Rebooting bots: the rain stays in the cell the bot left, the bar fills as the beats left go down. */
function rebootSprites(vm: ViewModel): RebootSprite[] {
  return vm.rebooting.map((r) => ({
    id: r.bot, x: r.cell.x, y: r.cell.y, color: colorOf(vm, r.bot),
    progress: r.total > 0 ? Math.min(1, Math.max(0, 1 - r.beatsLeft / r.total)) : 1,
  }));
}

const EMPTY: Scene = { beat: 0, width: 0, height: 0, walls: [], coins: [], taken: [], radars: [], tracers: [], bots: [], fallen: [], rebooting: [], effect: 1 };

/** Pure: the view model -> a renderer-agnostic scene. Scans are drawn as radars, not cones. */
export function buildScene(vm: ViewModel): Scene {
  const g = vm.grid;
  if (!g) return EMPTY;
  const events = thisBeat(vm);
  const hits = hitTracers(vm, g, events);
  const land: Landings = new Map();
  hits.forEach((t) => { if (t.victim) land.set(t.victim, Math.min(land.get(t.victim) ?? t.arrive, t.arrive)); });
  return {
    beat: vm.beat, width: g.width, height: g.height, walls: wallsOf(g), coins: vm.coins.map(pt), taken: takenCoins(events),
    radars: radars(vm),
    tracers: [...flyingTracers(vm, g, events), ...hits, ...wallTracers(vm, g, events)],
    bots: botSprites(vm, events, land), fallen: fallenSprites(vm, events, land),
    rebooting: rebootSprites(vm), effect: 1,
  };
}
