import type { BotSprite, Pt, Renderer, Scene, TracerLine } from "./scene";

/** Clock and frame scheduler; injectable for tests. */
export interface Frames {
  now(): number;
  request(fn: () => void): number;
  cancel(id: number): void;
}

export const rafFrames: Frames = {
  now: () => performance.now(),
  request: (fn) => requestAnimationFrame(fn),
  cancel: (id) => cancelAnimationFrame(id),
};

/** Jumps longer than this (cells) snap instead of gliding: respawns, new bots. */
const SNAP = 1.5;

const lerp = (a: Pt, b: Pt, t: number): Pt => ({ x: a.x + (b.x - a.x) * t, y: a.y + (b.y - a.y) * t });
const ease = (t: number): number => t * t * (3 - 2 * t);
/** Learned beat interval: default before two scenes, smoothing weight of a new gap, accepted range. */
export const DEFAULT_BEAT_MS = 1000;
const SMOOTH = 0.5;
const MIN_BEAT_MS = 200;
/** A flying bullet whose scene is late keeps going up to this fraction of its beat past the end (same speed). */
const OVERSHOOT = 1.1;
/** Length (cells) of the faint trail behind a bullet head. */
const TRAIL = 1;
const MAX_BEAT_MS = 3000;

/** Folds a gap between scenes into the beat estimate; a gap past the maximum (pause, reconnect) or under the minimum (a redraw) is ignored. */
export function learnBeat(beatMs: number, gapMs: number): number {
  if (gapMs > MAX_BEAT_MS || gapMs < MIN_BEAT_MS) return beatMs;
  return beatMs + (gapMs - beatMs) * SMOOTH;
}

interface Carry { startAbs: number; hpFrom: number; hpTo: number }
/** A flying bullet: the scene end it was flying to, where it was last drawn, and where its shot began (the trail is clipped there). */
interface Head { end: Pt; color: string; head: Pt; origin: Pt }
type Start = { head: Pt; origin: Pt };

const same = (a: Pt, b: Pt): boolean => a.x === b.x && a.y === b.y;

const near = (a: Pt, b: Pt): boolean => Math.hypot(a.x - b.x, a.y - b.y) <= SNAP;

/** Same size and same walls. */
function sameRoom(a: Scene, b: Scene): boolean {
  return a.width === b.width && a.height === b.height && a.walls.length === b.walls.length && a.walls.every((w, i) => same(w, b.walls[i]!));
}

/** Renderer decorator: over durationMs glides bots and sweeps radars; bullets fly linearly over the whole learned beat; then effects run (a hit's effect starts when its bullet lands). */
export class TweenRenderer implements Renderer {
  private target: Scene | null = null;
  private from = new Map<string, Pt>();
  private fromHp = new Map<string, number>();
  private fromCharge = new Map<string, number>();
  private fromFallen = new Map<string, Pt>();
  /** Last drawn sprites; `dead` ones are corpses (they glide on to a death cell but never seed a respawned bot). */
  private shown = new Map<string, Pt & { hp: number; charge: number; dead?: boolean }>();
  /** Unfinished hit/death effects of the previous scene, by bot id; they run out across the next scene. */
  private carry = new Map<string, Carry>();
  /** The last drawn sprite of each live bot: a bot that dies keeps this look until its killing bullet lands. */
  private looks = new Map<string, BotSprite>();
  /** Where each flying bullet was last drawn, and the scene end it was flying to. */
  private heads: Head[] = [];
  /** Per tracer of the current scene: where it starts (the last drawn head of the bullet it continues). */
  private starts: Start[] = [];
  private start = 0;
  private end = 0;
  private lastScene: number | null = null;
  private lastBeat: number | null = null;
  private beatMs = DEFAULT_BEAT_MS;
  private frame: number | null = null;

  /** durationMs: bot moves and radars; effectMs: hit and death effects right after (or at bullet landing). */
  constructor(
    private readonly inner: Renderer,
    private readonly durationMs: number,
    private readonly effectMs: number,
    private readonly frames: Frames = rafFrames,
  ) {}

  render(scene: Scene): void {
    const newGame = this.isNewGame(scene);
    if (newGame) this.forget();
    if (this.target && scene.width > 0 && scene.beat === this.lastBeat && !newGame) {
      this.target = scene; // a redraw of the same beat (sidebar change): same timing, same bullets
      this.cancel();
      this.tick();
      return;
    }
    this.rememberShown(scene);
    const now = this.frames.now();
    this.carry = this.carryOver(now);
    this.carry.forEach((c, id) => this.fromHp.set(id, c.hpTo));
    this.starts = scene.tracers.map((tr) => {
      const h = this.heads.find((x) => x.color === tr.color && same(x.end, tr.from));
      return h ? { head: h.head, origin: h.origin } : { head: tr.from, origin: tr.from };
    });
    this.target = scene;
    this.start = now;
    const empty = scene.width === 0;
    if (!empty && this.lastScene !== null && this.lastBeat === scene.beat - 1) this.beatMs = learnBeat(this.beatMs, this.start - this.lastScene);
    this.lastScene = empty ? null : this.start;
    this.lastBeat = empty ? null : scene.beat;
    this.end = this.endOf(scene);
    this.cancel();
    this.tick();
  }

  /** A new game: the beat count went back, or the room is another one (size or walls). Nothing of the old world may carry over. */
  private isNewGame(scene: Scene): boolean {
    const t = this.target;
    if (!t || t.width === 0 || scene.width === 0) return false;
    return (this.lastBeat !== null && scene.beat < this.lastBeat) || !sameRoom(t, scene);
  }

  /** Drops everything drawn of the old game: positions, effects in flight, bullets. */
  private forget(): void {
    this.shown = new Map();
    this.carry = new Map();
    this.looks = new Map();
    this.heads = [];
    this.lastScene = null;
  }

  /** Where each sprite of the new scene starts from: what was last drawn of it (a respawn never glides from its corpse). */
  private rememberShown(scene: Scene): void {
    this.from = new Map();
    this.fromHp = new Map();
    this.fromCharge = new Map();
    this.fromFallen = new Map();
    scene.bots.forEach((b) => {
      const p = this.shown.get(b.id);
      if (!p || p.dead) return;
      if (near(p, b)) this.from.set(b.id, p);
      this.fromHp.set(b.id, p.hp);
      if (p.charge <= b.charge) this.fromCharge.set(b.id, p.charge); // a shot empties the gun at once
    });
    scene.fallen.forEach((f) => {
      const p = this.shown.get(f.id);
      if (p && near(p, f)) this.fromFallen.set(f.id, p);
    });
  }

  /** Effects of the shown scene that have not finished at `now`, plus the ones still carried from before. */
  private carryOver(now: number): Map<string, Carry> {
    const out = new Map<string, Carry>();
    this.carry.forEach((c, id) => { if (c.startAbs + this.effectMs > now) out.set(id, c); });
    const s = this.target;
    if (!s) return out;
    const add = (id: string, landAt: number | undefined, on: boolean, hpFrom: number, hpTo: number) => {
      if (!on || landAt === undefined) return;
      const startAbs = this.start + this.beatMs * landAt;
      if (startAbs + this.effectMs > now) out.set(id, { startAbs, hpFrom, hpTo });
    };
    s.bots.forEach((b) => add(b.id, b.landAt, b.hit, this.fromHpOf(b.id, b.hp), b.hp));
    s.fallen.forEach((f) => add(f.id, f.landAt, f.dying, 0, 0));
    return out;
  }

  private fromHpOf(id: string, hp: number): number {
    return this.fromHp.get(id) ?? hp;
  }

  resize(widthPx: number, heightPx: number): void {
    this.inner.resize(widthPx, heightPx);
  }

  destroy(): void {
    this.cancel();
    this.target = null;
    this.inner.destroy();
  }

  private cancel(): void {
    if (this.frame !== null) this.frames.cancel(this.frame);
    this.frame = null;
  }

  /** When the last thing in the scene finishes: the common effect phase, a bullet's flight, or a landed hit's effect. */
  private endOf(s: Scene): number {
    const flights = s.tracers.map((t) => this.beatMs * t.arrive * (t.kind === "flying" ? OVERSHOOT : 1));
    const lands = [...s.bots, ...s.fallen].flatMap((x) => (x.landAt === undefined ? [] : [this.beatMs * x.landAt + this.effectMs]));
    const carried = [...this.carry.values()].map((c) => c.startAbs + this.effectMs - this.start);
    return Math.max(this.durationMs + this.effectMs, ...flights, ...lands, ...carried);
  }

  private readonly tick = (): void => {
    this.frame = null;
    if (!this.target) return;
    const elapsed = this.frames.now() - this.start;
    const effect = this.effectAt(elapsed, this.durationMs);
    this.inner.render(this.at(this.target, elapsed, effect));
    if (elapsed < this.end) this.frame = this.frames.request(this.tick);
  };

  private effectAt(elapsed: number, from: number): number {
    return Math.min(1, Math.max(0, (elapsed - from) / this.effectMs));
  }

  /** A bullet's own effect progress: starts when it lands; undefined for effects on the common timeline. */
  private landed(landAt: number | undefined, elapsed: number): number | undefined {
    return landAt === undefined ? undefined : this.effectAt(elapsed, this.beatMs * landAt);
  }

  /** Bullets fly linearly (a hit drops out on landing); bots glide on an eased curve; the radar line sweeps at a constant speed; health bars drain or refill in their effect phase; the gun charge fills steadily, a shot empties it at once. */
  private at(s: Scene, elapsed: number, effect: number): Scene {
    const raw = Math.min(1, elapsed / this.durationMs);
    this.shown = new Map();
    const bots = s.bots.map((b) => {
      const p = lerp(this.from.get(b.id) ?? b, b, this.glide(b.landAt, elapsed));
      const fx = this.effectOf(b.id, b.landAt, elapsed);
      const from = fx.carry ? fx.carry.hpFrom : this.fromHpOf(b.id, b.hp);
      const to = fx.carry ? fx.carry.hpTo : b.hp;
      const hp = from + (to - from) * (fx.own ?? effect);
      const c0 = this.fromCharge.get(b.id) ?? b.charge;
      const charge = c0 + (b.charge - c0) * raw;
      this.shown.set(b.id, { ...p, hp, charge });
      const drawn = { ...b, ...p, hp, charge, effect: fx.own, hit: b.hit || fx.carry !== undefined };
      this.looks.set(b.id, { ...drawn, hit: false, effect: undefined, landAt: undefined });
      return drawn;
    });
    const fallen = s.fallen.map((f) => {
      const fx = this.effectOf(f.id, f.landAt, elapsed);
      const p = lerp(this.fromFallen.get(f.id) ?? f, f, this.glide(f.landAt, elapsed));
      this.shown.set(f.id, { ...p, hp: 0, charge: 0, dead: true });
      return { ...f, ...p, dying: f.dying || fx.carry !== undefined, effect: fx.own, look: this.looks.get(f.id) };
    });
    this.looks.forEach((_, id) => { if (!this.shown.has(id)) this.looks.delete(id); }); // only bots of this scene
    this.heads = [];
    const tracers = s.tracers.flatMap((tr, i) => this.tracerAt(tr, this.starts[i] ?? { head: tr.from, origin: tr.from }, elapsed));
    const radars = s.radars.map((r) => ({ ...r, sweep: raw }));
    return { ...s, bots, fallen, tracers, radars, effect };
  }

  /** Eased glide progress; a sprite hit mid-beat arrives exactly when its bullet lands, so the bullet ends on it. */
  private glide(landAt: number | undefined, elapsed: number): number {
    const ms = landAt === undefined ? this.durationMs : Math.min(this.durationMs, this.beatMs * landAt);
    return ease(Math.min(1, elapsed / ms));
  }

  /** A sprite's own effect: the running carried one (when its scene has no landing of its own) and its progress. */
  private effectOf(id: string, landAt: number | undefined, elapsed: number): { carry?: Carry; own?: number } {
    const c = landAt === undefined ? this.carry.get(id) : undefined;
    const carried = c ? this.carried(c, elapsed) : undefined;
    return { carry: carried === undefined ? undefined : c, own: carried ?? this.landed(landAt, elapsed) };
  }

  /** A carried effect's progress while it runs (0..1, exclusive of done); undefined once finished. */
  private carried(c: Carry, elapsed: number): number | undefined {
    const p = this.effectAt(this.start + elapsed, c.startAbs);
    return p < 1 ? p : undefined;
  }

  /** Flies from the last drawn head to the scene's end, arriving on time; a flying bullet runs on a little past it if the scene is late. A hit or wall bullet drops on arrival. */
  private tracerAt(tr: TracerLine, st: Start, elapsed: number): TracerLine[] {
    const k = elapsed / (this.beatMs * tr.arrive);
    if (tr.kind !== "flying" && k >= 1) return [];
    const head = lerp(st.head, tr.to, Math.min(OVERSHOOT, k));
    if (tr.kind === "flying") this.heads.push({ end: tr.to, color: tr.color, head, origin: st.origin });
    return [{ ...tr, from: trailStart(st.origin, head), to: head }];
  }
}

/** Where the trail starts: TRAIL cells behind the head towards the shot's origin, never before it. */
function trailStart(origin: Pt, head: Pt): Pt {
  const d = Math.hypot(head.x - origin.x, head.y - origin.y);
  return d <= TRAIL ? origin : lerp(head, origin, TRAIL / d);
}
