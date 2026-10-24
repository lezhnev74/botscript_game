import { describe, expect, it } from "vitest";
import type { BotSprite, FallenSprite, Renderer, Scene, TracerLine } from "./scene";
import { learnBeat, TweenRenderer, type Frames } from "./tween";

const FIG = { body: "circle", hat: "none", eyes: "dots", mouth: "smile" };

const bot = (x: number, y: number): BotSprite => ({
  id: "A", x, y, color: "#f00", name: "A", ...FIG, edited: false, shielded: false, mine: false, hit: false, hp: 4, maxHp: 4, charge: 1,
});
let nextBeat = 0;
const scene = (x: number, y: number, extra: Partial<Scene> = {}): Scene => ({
  beat: ++nextBeat, width: 9, height: 9, walls: [], coins: [], taken: [], rebooting: [], radars: [], tracers: [], bots: [bot(x, y)], fallen: [], effect: 1, ...extra,
});

const fly = (x0: number, x1: number): TracerLine => ({ from: { x: x0, y: 1 }, to: { x: x1, y: 1 }, color: "#f00", kind: "flying", arrive: 1 });

describe("learnBeat", () => {
  it("smooths towards the observed gap", () => {
    expect(learnBeat(1000, 2000)).toBe(1500);
  });
  it("ignores a gap under the noise floor", () => {
    expect(learnBeat(1000, 10)).toBe(1000);
  });
  it("ignores a long gap (pause, reconnect)", () => {
    expect(learnBeat(1000, 60000)).toBe(1000);
  });
});

function setup(durationMs = 500, effectMs = 200) {
  let now = 0;
  let pending: (() => void) | null = null;
  const frames: Frames = { now: () => now, request: (fn) => { pending = fn; return 1; }, cancel: () => { pending = null; } };
  const drawn: Scene[] = [];
  const inner: Renderer = { render: (s) => drawn.push(s), resize: () => undefined, destroy: () => undefined };
  const r = new TweenRenderer(inner, durationMs, effectMs, frames);
  const advance = (ms: number) => { now += ms; const fn = pending; pending = null; fn?.(); };
  return { r, drawn, advance, last: () => drawn[drawn.length - 1]!, idle: () => pending === null };
}

describe("TweenRenderer", () => {
  it("draws the first scene as is", () => {
    const { r, last } = setup();
    r.render(scene(1, 1));
    expect(last().bots[0]).toMatchObject({ x: 1, y: 1 });
  });

  it("glides a one-cell move over the whole duration", () => {
    const { r, last, advance, idle } = setup();
    r.render(scene(1, 1));
    r.render(scene(2, 1));
    expect(last().bots[0]?.x).toBe(1);
    advance(125);
    expect(last().bots[0]?.x).toBeCloseTo(1.156, 2); // eased, linear would be 1.25
    advance(125);
    expect(last().bots[0]?.x).toBeCloseTo(1.5);
    advance(249);
    expect(last().bots[0]?.x).toBeLessThan(2);
    advance(1);
    expect(last().bots[0]?.x).toBe(2);
    advance(200);
    expect(idle()).toBe(true);
  });

  it("continues from the shown position when a new scene arrives mid-move", () => {
    const { r, last, advance } = setup();
    r.render(scene(1, 1));
    r.render(scene(2, 1));
    advance(250);
    r.render(scene(2, 2));
    expect(last().bots[0]).toMatchObject({ x: 1.5, y: 1 });
  });

  it("snaps long jumps (respawn)", () => {
    const { r, last } = setup();
    r.render(scene(1, 1));
    r.render(scene(6, 6));
    expect(last().bots[0]).toMatchObject({ x: 6, y: 6 });
  });

  it("flies a bullet linearly over the whole beat interval", () => {
    const { r, last, advance } = setup();
    r.render(scene(1, 1, { tracers: [fly(1, 4)] }));
    expect(last().tracers[0]?.to).toEqual({ x: 1, y: 1 });
    advance(250);
    expect(last().tracers[0]?.to.x).toBeCloseTo(1.75);
    advance(500);
    expect(last().tracers[0]?.to.x).toBeCloseTo(3.25);
    advance(250);
    expect(last().tracers[0]?.to).toEqual({ x: 4, y: 1 });
  });

  it("keeps animating until a bullet has flown the whole beat", () => {
    const { r, advance, idle } = setup();
    r.render(scene(1, 1, { tracers: [fly(1, 4)] }));
    advance(1000);
    expect(idle()).toBe(false);
    advance(100);
    expect(idle()).toBe(true);
  });

  it("starts the next beat's bullet where the last one was drawn", () => {
    const { r, last, advance } = setup();
    r.render(scene(1, 1, { tracers: [fly(1, 4)] }));
    advance(1000);
    const endHead = last().tracers[0]!.to;
    advance(0);
    r.render(scene(1, 1, { tracers: [fly(4, 7)] }));
    expect(last().tracers[0]?.to).toEqual(endHead);
  });

  it("lands a hit bullet after distance / speed of the beat and then drops it", () => {
    const { r, last, advance } = setup();
    r.render(scene(1, 1, { tracers: [{ ...fly(1, 3), kind: "hit", arrive: 2 / 3 }] }));
    advance(333);
    expect(last().tracers[0]?.to.x).toBeCloseTo(2, 1);
    advance(334);
    expect(last().tracers).toEqual([]);
  });

  it("starts a hit blink and hp drain when its bullet lands", () => {
    const { r, last, advance } = setup();
    r.render(scene(1, 1));
    advance(1000);
    r.render(scene(1, 1, { bots: [{ ...bot(1, 1), hp: 3, hit: true, landAt: 0.2 }] }));
    advance(199);
    expect(last().bots[0]).toMatchObject({ hp: 4, effect: 0 });
    advance(101);
    expect(last().bots[0]?.effect).toBeCloseTo(0.5);
    expect(last().bots[0]?.hp).toBeCloseTo(3.5);
    advance(100);
    expect(last().bots[0]).toMatchObject({ hp: 3, effect: 1 });
  });

  it("starts a death burst when the killing bullet lands and runs until it ends", () => {
    const { r, last, advance, idle } = setup();
    const dead: FallenSprite = { id: "B", x: 3, y: 1, color: "#0f0", name: "B", ...FIG, label: "respawn 5", dying: true, landAt: 0.5 };
    r.render(scene(1, 1, { fallen: [dead] }));
    advance(500);
    expect(last().fallen[0]?.effect).toBe(0);
    advance(199);
    expect(idle()).toBe(false);
    advance(1);
    expect(last().fallen[0]?.effect).toBe(1);
    expect(idle()).toBe(true);
  });

  it("leaves effects without a bullet on the common timeline", () => {
    const { r, last } = setup();
    r.render(scene(1, 1));
    expect(last().bots[0]?.effect).toBeUndefined();
  });

  it("learns the beat interval from the gap between scenes", () => {
    const { r, last, advance } = setup();
    r.render(scene(1, 1));
    advance(2000);
    r.render(scene(1, 1, { tracers: [fly(1, 4)] }));
    advance(750); // interval 1500 after smoothing 1000 and 2000: half way
    expect(last().tracers[0]?.to.x).toBeCloseTo(2.5);
  });

  it("sweeps radars at a constant speed and ends done", () => {
    const { r, last, advance } = setup();
    r.render(scene(1, 1, { radars: [{ x: 1, y: 1, angle: 0, color: "#f00", sweep: 1 }] }));
    expect(last().radars[0]?.sweep).toBe(0);
    advance(125);
    expect(last().radars[0]?.sweep).toBeCloseTo(0.25);
    advance(375);
    expect(last().radars[0]?.sweep).toBe(1);
  });

  it("runs the effect phase after the moves", () => {
    const { r, last, advance, idle } = setup();
    r.render(scene(1, 1));
    expect(last().effect).toBe(0);
    advance(500);
    expect(last().effect).toBe(0);
    advance(100);
    expect(last().effect).toBeCloseTo(0.5);
    advance(100);
    expect(last().effect).toBe(1);
    expect(idle()).toBe(true);
  });

  it("drains the health bar during the effect phase", () => {
    const { r, last, advance } = setup();
    r.render(scene(1, 1));
    advance(700);
    r.render(scene(1, 1, { bots: [{ ...bot(1, 1), hp: 3 }] }));
    expect(last().bots[0]?.hp).toBe(4);
    advance(500);
    expect(last().bots[0]?.hp).toBe(4);
    advance(100);
    expect(last().bots[0]?.hp).toBeCloseTo(3.5);
    advance(100);
    expect(last().bots[0]?.hp).toBe(3);
  });

  it("fills the gun charge steadily and empties it at once", () => {
    const { r, last, advance } = setup();
    r.render(scene(1, 1));
    advance(700);
    r.render(scene(1, 1, { bots: [{ ...bot(1, 1), charge: 0 }] }));
    expect(last().bots[0]?.charge).toBe(0);
    advance(700);
    r.render(scene(1, 1, { bots: [{ ...bot(1, 1), charge: 0.5 }] }));
    advance(250);
    expect(last().bots[0]?.charge).toBeCloseTo(0.25);
    advance(250);
    expect(last().bots[0]?.charge).toBe(0.5);
  });

  it("keeps taken coins through the move and the effect phase", () => {
    const { r, last, advance } = setup();
    const taken = [{ x: 2, y: 1, bot: "A" }];
    r.render(scene(1, 1));
    r.render(scene(2, 1, { taken }));
    advance(250);
    expect(last().taken).toEqual(taken);
    expect(last().effect).toBe(0);
    advance(350);
    expect(last().taken).toEqual(taken);
    expect(last().effect).toBeGreaterThan(0);
  });

  it("shows a new bot's health as is", () => {
    const { r, last } = setup();
    r.render(scene(1, 1, { bots: [{ ...bot(1, 1), hp: 2 }] }));
    expect(last().bots[0]?.hp).toBe(2);
  });

  it("stops animating on destroy", () => {
    const { r, advance, drawn } = setup();
    r.render(scene(1, 1));
    r.render(scene(2, 1));
    r.destroy();
    const n = drawn.length;
    advance(100);
    expect(drawn.length).toBe(n);
  });

  it("keeps a hit bullet visible until it lands, past the effect phase start", () => {
    const { r, last, advance } = setup();
    r.render(scene(1, 1, { tracers: [{ ...fly(1, 4), kind: "hit", arrive: 1 }] }));
    advance(700);
    expect(last().effect).toBeGreaterThan(0);
    expect(last().tracers[0]?.to.x).toBeCloseTo(3.1);
  });

  it("finishes a hit blink and hp drain that straddle the next scene", () => {
    const { r, last, advance } = setup();
    r.render(scene(1, 1));
    advance(1000);
    r.render(scene(1, 1, { bots: [{ ...bot(1, 1), hp: 3, hit: true, landAt: 1 }] }));
    advance(1000);
    r.render(scene(1, 1, { bots: [{ ...bot(1, 1), hp: 3 }] }));
    expect(last().bots[0]).toMatchObject({ hit: true, effect: 0, hp: 4 });
    advance(100);
    expect(last().bots[0]?.effect).toBeCloseTo(0.5);
    expect(last().bots[0]?.hp).toBeCloseTo(3.5);
    advance(100);
    expect(last().bots[0]).toMatchObject({ hit: false, hp: 3 });
    advance(500);
    expect(last().bots[0]).toMatchObject({ hit: false, hp: 3 });
  });

  it("finishes a death burst that straddles the next scene", () => {
    const { r, last, advance, idle } = setup();
    const dead = (dying: boolean, landAt?: number): FallenSprite => ({ id: "B", x: 3, y: 1, color: "#0f0", name: "B", ...FIG, label: "respawn 5", dying, ...(landAt === undefined ? {} : { landAt }) });
    r.render(scene(1, 1));
    advance(1000);
    r.render(scene(1, 1, { fallen: [dead(true, 1)] }));
    advance(1000);
    r.render(scene(1, 1, { fallen: [dead(false)] }));
    expect(last().fallen[0]).toMatchObject({ dying: true, effect: 0 });
    advance(100);
    expect(last().fallen[0]?.effect).toBeCloseTo(0.5);
    advance(100);
    expect(last().fallen[0]?.dying).toBe(false);
    advance(600);
    expect(idle()).toBe(true);
  });

  it("starts a hit bullet already in flight from where it was last drawn", () => {
    const { r, last, advance } = setup();
    r.render(scene(1, 1, { tracers: [fly(1, 4)] }));
    advance(1000);
    r.render(scene(1, 1, { tracers: [{ ...fly(4, 5), kind: "hit", arrive: 1 / 3, victim: "A" }] }));
    expect(last().tracers[0]?.to).toEqual({ x: 4, y: 1 });
    advance(166);
    expect(last().tracers[0]?.to.x).toBeCloseTo(4.5, 1);
  });

  it("does not jump a continuing bullet when the next scene comes early", () => {
    const { r, last, advance } = setup();
    r.render(scene(1, 1, { tracers: [fly(1, 4)] }));
    advance(970);
    const head = last().tracers[0]!.to;
    expect(head.x).toBeCloseTo(3.91);
    r.render(scene(1, 1, { tracers: [fly(4, 7)] }));
    expect(last().tracers[0]?.to.x).toBeCloseTo(3.91);
    advance(985);
    expect(last().tracers[0]?.to).toEqual({ x: 7, y: 1 });
  });

  it("does not restart a continuing bullet behind when the next scene comes late", () => {
    const { r, last, advance } = setup();
    r.render(scene(1, 1, { tracers: [fly(1, 4)] }));
    advance(1000);
    advance(30);
    const head = last().tracers[0]!.to;
    expect(head.x).toBeCloseTo(4.09);
    r.render(scene(1, 1, { tracers: [fly(4, 7)] }));
    expect(last().tracers[0]?.to).toEqual(head);
    advance(1015);
    expect(last().tracers[0]?.to).toEqual({ x: 7, y: 1 });
  });

  it("keeps a late bullet flying past its end, then waits", () => {
    const { r, last, advance, idle } = setup();
    r.render(scene(1, 1, { tracers: [fly(1, 4)] }));
    advance(1050);
    expect(last().tracers[0]?.to.x).toBeCloseTo(4.15);
    advance(50);
    expect(last().tracers[0]?.to.x).toBeCloseTo(4.3);
    expect(idle()).toBe(true);
  });

  it("drops a wall bullet when it arrives", () => {
    const { r, last, advance } = setup();
    r.render(scene(1, 1, { tracers: [{ ...fly(1, 1.5), kind: "wall", arrive: 1 / 6 }] }));
    advance(100);
    expect(last().tracers).toHaveLength(1);
    advance(70);
    expect(last().tracers).toEqual([]);
  });

  it("draws a fixed-length trail behind the head, clipped at the origin", () => {
    const { r, last, advance } = setup();
    r.render(scene(1, 1, { tracers: [fly(1, 4)] }));
    advance(250);
    expect(last().tracers[0]?.from.x).toBeCloseTo(1);
    advance(500);
    expect(last().tracers[0]?.from.x).toBeCloseTo(2.25);
    advance(250);
    r.render(scene(1, 1, { tracers: [fly(4, 7)] }));
    expect(last().tracers[0]?.from.x).toBeCloseTo(3);
    advance(500);
    const t = last().tracers[0]!;
    expect(t.to.x - t.from.x).toBeCloseTo(1);
  });

  it("learns only from consecutive beats", () => {
    const { r, last, advance } = setup();
    r.render(scene(1, 1, { beat: 10 }));
    advance(2000);
    r.render(scene(1, 1, { beat: 15, tracers: [fly(1, 4)] }));
    advance(500);
    expect(last().tracers[0]?.to.x).toBeCloseTo(2.5);
  });

  it("ignores gaps under the noise floor", () => {
    const { r, last, advance } = setup();
    r.render(scene(1, 1, { beat: 10 }));
    advance(100);
    r.render(scene(1, 1, { beat: 11, tracers: [fly(1, 4)] }));
    advance(500);
    expect(last().tracers[0]?.to.x).toBeCloseTo(2.5);
  });

  it("does not learn from the empty scene before the first frame", () => {
    const { r, last, advance } = setup();
    r.render(scene(1, 1, { beat: 0, width: 0, height: 0, bots: [] }));
    advance(50);
    r.render(scene(1, 1, { beat: 1, tracers: [fly(1, 4)] }));
    advance(500);
    expect(last().tracers[0]?.to.x).toBeCloseTo(2.5);
  });

  it("does not restart or move bullets when the same beat is drawn again", () => {
    const { r, last, advance } = setup();
    const s = scene(1, 1, { beat: 7, tracers: [fly(1, 4)] });
    r.render(s);
    advance(500);
    const before = last().tracers[0]!;
    r.render({ ...s, bots: s.bots.map((b) => ({ ...b, mine: true })) });
    expect(last().tracers[0]).toEqual(before);
    expect(last().bots[0]?.mine).toBe(true);
    advance(500);
    expect(last().tracers[0]?.to.x).toBeCloseTo(4);
  });

  it("keeps a bot's eased glide when the same beat is drawn again", () => {
    const { r, last, advance } = setup();
    const s = scene(2, 1, { beat: 7 });
    r.render(scene(1, 1, { beat: 6 }));
    r.render(s);
    advance(250);
    r.render(s);
    expect(last().bots[0]?.x).toBeCloseTo(1.5);
  });

  describe("death while moving", () => {
    const corpse = (x: number, y: number, extra: Partial<FallenSprite> = {}): FallenSprite => ({ id: "A", x, y, color: "#f00", name: "A", ...FIG, label: "respawn 5", dying: true, ...extra });
    const drawnCount = (sc: Scene): number => sc.bots.length + sc.fallen.length;

    it("glides the corpse from the old cell to the death cell, drawn exactly once per frame", () => {
      const { r, drawn, last, advance } = setup();
      r.render(scene(1, 1));
      advance(1000);
      const n = drawn.length;
      r.render(scene(0, 0, { bots: [], fallen: [corpse(2, 1, { landAt: 0.5 })] }));
      expect(last().fallen[0]).toMatchObject({ x: 1, y: 1 });
      advance(125);
      expect(last().fallen[0]?.x).toBeCloseTo(1.156, 2);
      advance(375);
      expect(last().fallen[0]).toMatchObject({ x: 2, y: 1, effect: 0 });
      advance(100);
      expect(last().fallen[0]).toMatchObject({ x: 2, dying: true });
      expect(last().fallen[0]?.effect).toBeCloseTo(0.5);
      expect(drawn.slice(n).every((d) => drawnCount(d) === 1)).toBe(true);
    });

    it("keeps a corpse that dies without moving in place", () => {
      const { r, drawn, advance } = setup();
      r.render(scene(1, 1));
      advance(1000);
      const n = drawn.length;
      r.render(scene(0, 0, { bots: [], fallen: [corpse(1, 1, { landAt: 0.3 })] }));
      advance(500);
      expect(drawn.slice(n).every((d) => d.fallen[0]?.x === 1 && d.fallen[0]?.y === 1 && drawnCount(d) === 1)).toBe(true);
    });

    it("keeps the glide going when the next scene arrives mid-glide", () => {
      const { r, last, advance } = setup();
      r.render(scene(1, 1));
      advance(1000);
      r.render(scene(0, 0, { bots: [], fallen: [corpse(2, 1, { landAt: 0.5 })] }));
      advance(250);
      r.render(scene(0, 0, { bots: [], fallen: [corpse(2, 1, { dying: false })] }));
      expect(last().fallen[0]?.x).toBeCloseTo(1.5);
      advance(500);
      expect(last().fallen[0]?.x).toBe(2);
    });

    it("glides a bot that moves into a bullet and blinks at the new cell", () => {
      const { r, last, advance } = setup();
      r.render(scene(1, 1));
      advance(1000);
      r.render(scene(2, 1, { bots: [{ ...bot(2, 1), hp: 3, hit: true, landAt: 0.5 }] }));
      advance(250);
      expect(last().bots[0]?.x).toBeCloseTo(1.5);
      advance(250);
      expect(last().bots[0]).toMatchObject({ x: 2, effect: 0, hit: true });
      advance(200);
      expect(last().bots[0]).toMatchObject({ x: 2, effect: 1, hp: 3 });
    });

    it("respawns at the spawn cell without gliding, even next to the corpse", () => {
      const { r, last, advance } = setup();
      r.render(scene(1, 1));
      advance(1000);
      r.render(scene(0, 0, { bots: [], fallen: [corpse(2, 1, { landAt: 0.5 })] }));
      advance(1000);
      r.render(scene(0, 0, { bots: [], fallen: [corpse(2, 1, { dying: false, label: "respawn 1" })] }));
      advance(1000);
      r.render(scene(3, 1));
      expect(last().bots[0]).toMatchObject({ x: 3, y: 1 });
      expect(last().fallen).toEqual([]);
      advance(250);
      expect(last().bots[0]).toMatchObject({ x: 3, y: 1 });
    });

    describe("at real timing (600 ms glide, 350 ms effect, 1000 ms beat, bullet lands at 1/3)", () => {
      const L = 1 / 3;
      const trace = (r: ReturnType<typeof setup>, ms: number, pick: () => number | undefined): Array<[number, number | undefined]> => {
        const out: Array<[number, number | undefined]> = [];
        for (let t = 50; t <= ms; t += 50) { r.advance(50); out.push([t, pick()]); }
        return out;
      };

      it("a bot moving into a bullet is at its new cell the moment the bullet lands", () => {
        const h = setup(600, 350);
        h.r.render(scene(1, 1));
        h.advance(1000);
        h.r.render(scene(2, 1, { bots: [{ ...bot(2, 1), hp: 3, hit: true, landAt: L }] }));
        const xs = trace(h, 350, () => h.last().bots[0]?.x);
        expect(xs.find(([t]) => t === 300)?.[1]).toBeLessThan(2);
        expect(xs.filter(([t]) => t >= 350).every(([, x]) => x === 2)).toBe(true);
      });

      it("a dying corpse is at the death cell when the bullet lands and the burst never moves", () => {
        const h = setup(600, 350);
        h.r.render(scene(1, 1));
        h.advance(1000);
        h.r.render(scene(0, 0, { bots: [], fallen: [corpse(2, 1, { landAt: L })] }));
        const xs = trace(h, 700, () => h.last().fallen[0]?.x);
        expect(xs.find(([t]) => t === 300)?.[1]).toBeLessThan(2);
        expect(xs.filter(([t]) => t >= 350).every(([, x]) => x === 2)).toBe(true);
      });

      it("forgets the look of a bot that left the scene", () => {
        const h = setup(600, 350);
        h.r.render(scene(1, 1, { bots: [bot(1, 1)] }));
        h.advance(1000);
        h.r.render(scene(0, 0, { bots: [], fallen: [] }));
        h.advance(100);
        h.r.render(scene(0, 0, { bots: [], fallen: [corpse(2, 1, { landAt: L })] }));
        h.advance(100);
        expect(h.last().fallen[0]?.look).toBeUndefined();
      });

      it("a dying sprite keeps the look of the live bot until it lands", () => {
        const h = setup(600, 350);
        h.r.render(scene(1, 1, { bots: [{ ...bot(1, 1), shielded: true, mine: true, hp: 2, charge: 0.5 }] }));
        h.advance(1000);
        h.r.render(scene(0, 0, { bots: [], fallen: [corpse(2, 1, { landAt: L })] }));
        h.advance(100);
        expect(h.last().fallen[0]?.look).toMatchObject({ shielded: true, mine: true, hp: 2, charge: 0.5 });
      });
    });
  });
});

describe("TweenRenderer across a new game", () => {
  it("does not glide a bot from the old map's cell when the room changes", () => {
    const { r, last } = setup();
    r.render(scene(1, 1));
    r.render(scene(2, 1, { width: 11, height: 7 }));
    expect(last().bots[0]).toMatchObject({ x: 2, y: 1 });
  });

  it("starts over when the beat count goes back, even on the same size", () => {
    const { r, last, advance } = setup();
    r.render(scene(1, 1, { beat: 50 }));
    r.render(scene(2, 1, { beat: 51 }));
    advance(500);
    r.render(scene(3, 1, { beat: 1 }));
    advance(10);
    expect(last().bots[0]?.x).toBe(3);
  });

  it("draws the walls of a different map on the same beat at once", () => {
    const { r, last, drawn } = setup();
    r.render(scene(1, 1, { beat: 1, walls: [{ x: 3, y: 3 }] }));
    const before = drawn.length;
    r.render(scene(2, 1, { beat: 1, walls: [{ x: 4, y: 4 }] }));
    expect(drawn.length).toBeGreaterThan(before);
    expect(last().walls).toEqual([{ x: 4, y: 4 }]);
    expect(last().bots[0]?.x).toBe(2); // no glide from the old game
  });

  it("still treats a redraw of the same room and beat as the same scene", () => {
    const { r, last, advance } = setup();
    r.render(scene(1, 1, { beat: 10 }));
    r.render(scene(2, 1, { beat: 11 }));
    advance(125);
    const x = last().bots[0]!.x;
    r.render(scene(2, 1, { beat: 11 }));
    expect(last().bots[0]!.x).toBeCloseTo(x, 5);
  });
});
