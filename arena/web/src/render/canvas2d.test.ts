import { describe, expect, it } from "vitest";
import { BAR_H, BAR_TOP, BOT_R } from "./shapes";
import { Canvas2dRenderer } from "./canvas2d";
import { TweenRenderer } from "./tween";
import type { Scene } from "./scene";

const FIG = { body: "circle", hat: "none", eyes: "dots", mouth: "smile" };

const scene: Scene = {
  beat: 1, width: 5, height: 5, walls: [{ x: 0, y: 0 }], coins: [{ x: 2, y: 2 }], taken: [], rebooting: [{ id: "R", x: 4, y: 4, color: "#0ff", progress: 0.4 }],
  radars: [{ x: 1, y: 1, angle: 0, color: "#f00", sweep: 0.5 }, { x: 2, y: 2, angle: Math.PI, color: "#0f0", sweep: 1 }],
  tracers: [{ from: { x: 1, y: 1 }, to: { x: 4, y: 1 }, color: "#f00", kind: "flying", arrive: 1 }],
  bots: [{ id: "A", x: 1, y: 1, color: "#f00", name: "A", ...FIG, edited: true, shielded: true, mine: true, hit: true, hp: 2.5, maxHp: 4, charge: 0.4 }, { id: "D", x: 3, y: 3, color: "#0ff", name: "D", ...FIG, edited: false, shielded: false, mine: false, hit: false, hp: 1, maxHp: 0, charge: 1 }],
  fallen: [{ id: "B", x: 3, y: 1, color: "#0f0", name: "B", ...FIG, label: "respawn 2", dying: false }, { id: "C", x: 2, y: 3, color: "#00f", name: "C", ...FIG, label: "respawn 5", dying: true }],
  effect: 0.1,
};

/** A 2D context that records calls and accepts any property. */
function fakeCanvas() {
  const calls: string[] = [];
  const ctx = new Proxy({}, {
    get: (_t, k: string) => (k === "measureText" ? () => ({ width: 10 }) : (...a: unknown[]) => { calls.push(`${k}:${a.length}`); return k === "createRadialGradient" ? { addColorStop: () => undefined } : undefined; }),
    set: () => true,
  });
  const canvas = { width: 0, height: 0, style: {} as Record<string, string>, getContext: () => ctx } as unknown as HTMLCanvasElement;
  return { canvas, calls };
}

describe("Canvas2dRenderer (smoke)", () => {
  it("draws a full scene and an empty one without throwing", () => {
    const { canvas, calls } = fakeCanvas();
    const r = new Canvas2dRenderer(canvas, 2);
    r.resize(500, 500);
    r.render(scene);
    r.render({ ...scene, effect: 0 });
    r.render({ ...scene, effect: 1 });
    r.render({ ...scene, width: 0, height: 0, walls: [], coins: [], taken: [], rebooting: [], radars: [], tracers: [], bots: [], fallen: [], effect: 1 });
    expect(canvas.width).toBe(1000);
    expect(calls.some((c) => c.startsWith("fillText"))).toBe(true);
    r.destroy();
  });
});

describe("Canvas2dRenderer reboot", () => {
  const lone: Scene = { ...scene, coins: [], walls: [], radars: [], tracers: [], bots: [], fallen: [], effect: 1 };

  it("draws rain glyphs and a progress bar at the cell the bot left, and nothing without a reboot", () => {
    const draw = (sc: Scene): string[] => {
      const { canvas, calls } = fakeCanvas();
      const r = new Canvas2dRenderer(canvas, 1, null);
      r.resize(500, 500);
      r.render(sc);
      return calls;
    };
    const n = (calls: string[], k: string): number => calls.filter((c) => c === k).length;
    const rebooting = draw(lone);
    const none = draw({ ...lone, rebooting: [] });
    expect(n(rebooting, "fillText:3")).toBeGreaterThanOrEqual(4); // rain glyphs
    expect(n(none, "fillText:3")).toBe(0);
    expect(n(rebooting, "fillRect:4")).toBe(n(none, "fillRect:4") + 3); // cell tint, bar well, bar fill (the floor is already in none)
    expect(n(rebooting, "strokeRect:4")).toBe(1); // bar border
  });

  it("fills the bar by the progress and falls with the clock", () => {
    const rects: number[][] = [];
    const ctx = new Proxy({}, {
      get: (_t, k: string) => (k === "fillRect" ? (...a: number[]) => rects.push(a) : () => undefined),
      set: () => true,
    });
    const canvas = { width: 0, height: 0, style: {}, getContext: () => ctx } as unknown as HTMLCanvasElement;
    const r = new Canvas2dRenderer(canvas, 1, null);
    r.resize(500, 500);
    r.render({ ...lone, rebooting: [{ id: "R", x: 1, y: 1, color: "#0ff", progress: 0.5 }] });
    const widths = rects.map((a) => a[2]!).filter((w) => w > 0 && w < 100);
    expect(widths[widths.length - 1]! / widths[widths.length - 2]!).toBeCloseTo(0.5, 5); // fill / well
  });

  it("keeps repainting for the rain while a bot reboots, and stops with the last reboot", () => {
    const { canvas } = fakeCanvas();
    const queue: (() => void)[] = [];
    const frames = { request: (cb: () => void) => queue.push(cb), cancel: () => undefined, now: () => 0 };
    const r = new Canvas2dRenderer(canvas, 1, frames as never);
    r.resize(500, 500);
    r.render(lone);
    expect(queue).toHaveLength(1);
    r.render({ ...lone, rebooting: [] });
    expect(queue).toHaveLength(1);
  });
});

describe("Canvas2dRenderer pickup", () => {
  const only: Scene = { ...scene, coins: [], walls: [], radars: [], tracers: [], fallen: [], bots: [scene.bots[1]!], taken: [{ x: 3, y: 3, bot: "D" }] };

  it("draws the taken coin under the bot, then a golden glow around the taker", () => {
    const { canvas, calls } = fakeCanvas();
    const r = new Canvas2dRenderer(canvas, 1, null, () => ({}) as CanvasImageSource);
    r.resize(500, 500);
    r.render({ ...only, effect: 0.3 });
    expect(calls.filter((c) => c === "drawImage:5").length).toBe(1);
    expect(calls.indexOf("drawImage:5")).toBeLessThan(calls.indexOf("createRadialGradient:6"));
  });

  it("falls back to a disc, and draws no coin or glow once the effect is done", () => {
    const { canvas, calls } = fakeCanvas();
    const r = new Canvas2dRenderer(canvas, 1, null, () => null);
    r.resize(500, 500);
    r.render({ ...only, effect: 1 });
    expect(calls).not.toContain("drawImage:5");
    expect(calls).not.toContain("createRadialGradient:6");
    const n = calls.filter((c) => c === "arc:5").length;
    r.render({ ...only, effect: 0.5 });
    expect(calls.filter((c) => c === "arc:5").length - n).toBeGreaterThan(2); // coin disc + glow + body
  });
});

describe("Canvas2dRenderer hit bullets", () => {
  it("draws a hit bullet during the effect phase until the tween drops it", () => {
    const { canvas, calls } = fakeCanvas();
    const r = new Canvas2dRenderer(canvas, 1, null, () => null);
    r.resize(500, 500);
    const bare: Scene = { ...scene, coins: [], walls: [], radars: [], fallen: [], bots: [], effect: 0.5 };
    r.render({ ...bare, tracers: [] });
    const none = calls.filter((c) => c === "arc:5").length;
    r.render({ ...bare, tracers: [{ from: { x: 1, y: 1 }, to: { x: 2, y: 1 }, color: "#f00", kind: "hit", arrive: 1, victim: "A" }] });
    expect(calls.filter((c) => c === "arc:5").length - none).toBe(2);
  });
});

describe("Canvas2dRenderer coins", () => {
  const frames = () => {
    let queued: (() => void) | null = null;
    let t = 0;
    return {
      f: { now: () => t, request: (fn: () => void) => ((queued = fn), 1), cancel: () => { queued = null; } },
      tick: (to: number) => { t = to; const q = queued; queued = null; q?.(); },
      pending: () => queued !== null,
    };
  };

  it("draws a gold disc until the sprite is ready, then the image", () => {
    const { canvas, calls } = fakeCanvas();
    let img: CanvasImageSource | null = null;
    const r = new Canvas2dRenderer(canvas, 1, null, () => img);
    r.resize(500, 500);
    r.render(scene);
    expect(calls.includes("drawImage:5")).toBe(false);
    img = {} as CanvasImageSource;
    r.render(scene);
    expect(calls.includes("drawImage:5")).toBe(true);
  });

  it("repaints on its own clock while coins exist and stops without", () => {
    const { canvas, calls } = fakeCanvas();
    const fr = frames();
    const r = new Canvas2dRenderer(canvas, 1, fr.f, () => null);
    r.resize(500, 500);
    r.render(scene);
    const before = calls.length;
    expect(fr.pending()).toBe(true);
    fr.tick(100); // first frame after a render is skipped: render just painted
    fr.tick(200);
    expect(calls.length).toBeGreaterThan(before);
    expect(fr.pending()).toBe(true);
    r.render({ ...scene, coins: [], rebooting: [] });
    expect(fr.pending()).toBe(false);
    r.render(scene);
    r.destroy();
    expect(fr.pending()).toBe(false);
  });

  it("paints at most once per frame while a tween drives it", () => {
    const { canvas, calls } = fakeCanvas();
    const q = new Map<number, () => void>();
    let t = 0;
    let id = 0;
    const f = { now: () => t, request: (fn: () => void) => (q.set(++id, fn), id), cancel: (i: number) => void q.delete(i) };
    const r = new TweenRenderer(new Canvas2dRenderer(canvas, 1, f, () => null), 1000, 1000, f);
    r.resize(500, 500);
    r.render(scene);
    const paints = () => calls.filter((c) => c === "setTransform:6").length;
    for (let i = 1; i <= 5; i++) {
      t = i * 16;
      const before = paints();
      const run = [...q.values()];
      q.clear();
      run.forEach((fn) => fn());
      expect(paints() - before).toBe(1);
    }
  });

  it("draws a dying sprite exactly like the live bot until its bullet lands", () => {
    const look = { id: "A", x: 1, y: 1, color: "#f00", name: "A", ...FIG, edited: true, shielded: true, mine: true, hit: false, hp: 2, maxHp: 4, charge: 0.4 };
    const base = { beat: 1, width: 5, height: 5, walls: [], coins: [], taken: [], rebooting: [], radars: [], tracers: [] };
    const draw = (sc: Scene): string[] => {
      const { canvas, calls } = fakeCanvas();
      const r = new Canvas2dRenderer(canvas, 2);
      r.resize(500, 500);
      r.render(sc);
      return calls;
    };
    const live = draw({ ...base, bots: [look], fallen: [], effect: 0 });
    const dying = draw({ ...base, bots: [], fallen: [{ id: "A", x: 1, y: 1, color: "#f00", name: "A", ...FIG, label: "respawn 5", dying: true, look, effect: 0 }], effect: 0 });
    const after = draw({ ...base, bots: [], fallen: [{ id: "A", x: 1, y: 1, color: "#f00", name: "A", ...FIG, label: "respawn 5", dying: true, look, effect: 0.5 }], effect: 0 });
    expect(dying).toEqual(live);
    expect(after).not.toEqual(live);
  });

  it("shows x eyes and a flat mouth on a dying sprite once the bullet has landed, whatever the remembered look", () => {
    const look = { id: "A", x: 1, y: 1, color: "#f00", name: "A", ...FIG, edited: false, shielded: false, mine: false, hit: false, hp: 2, maxHp: 4, charge: 1 };
    const base = { beat: 1, width: 5, height: 5, walls: [], coins: [], taken: [], rebooting: [], radars: [], tracers: [], bots: [], effect: 0 };
    const draw = (eyes: string, mouth: string, lookEyes: string, effect: number): string[] => {
      const { canvas, calls } = fakeCanvas();
      const r = new Canvas2dRenderer(canvas, 2);
      r.resize(500, 500);
      r.render({ ...base, fallen: [{ id: "A", x: 1, y: 1, color: "#f00", name: "A", ...FIG, eyes, mouth, label: "respawn 5", dying: true, effect, look: { ...look, eyes: lookEyes } }] });
      return calls;
    };
    expect(draw("x", "flat", "dots", 0.5)).toEqual(draw("x", "flat", "wide", 0.5));
    expect(draw("x", "flat", "dots", 0.5)).not.toEqual(draw("dots", "smile", "dots", 0.5));
    expect(draw("x", "flat", "dots", 0)).toEqual(draw("dots", "smile", "dots", 0)); // before the landing: the live face
  });

  it("draws no hit ring or sparks for a dying sprite without a look before it lands, and keeps the gauge while it shrinks", () => {
    const base = { beat: 1, width: 5, height: 5, walls: [], coins: [], taken: [], rebooting: [], radars: [], tracers: [], bots: [], effect: 0 };
    const draw = (effect: number, look?: Scene["bots"][number]): string[] => {
      const { canvas, calls } = fakeCanvas();
      const r = new Canvas2dRenderer(canvas, 2);
      r.resize(500, 500);
      r.render({ ...base, fallen: [{ id: "A", x: 1, y: 1, color: "#f00", name: "A", ...FIG, label: "respawn 5", dying: true, effect, ...(look ? { look } : {}) }] });
      return calls;
    };
    const n = (calls: string[], k: string) => calls.filter((c) => c === k).length;
    const bare = (() => { const { canvas, calls } = fakeCanvas(); const r = new Canvas2dRenderer(canvas, 2); r.resize(500, 500); r.render({ ...base, fallen: [] }); return calls; })();
    expect(n(draw(0), "arc:5")).toBeGreaterThan(n(bare, "arc:5")); // the plain figure
    expect(n(draw(0.5), "moveTo:2")).toBeGreaterThanOrEqual(n(draw(0), "moveTo:2") + 8); // eight sparks
    const gauge = { id: "A", x: 1, y: 1, color: "#f00", name: "A", ...FIG, edited: false, shielded: false, mine: false, hit: false, hp: 2, maxHp: 4, charge: 0.4 };
    expect(draw(0.5, gauge)).toContain("closePath:0");
  });
});

describe("Canvas2dRenderer looks", () => {
  it("draws every body, eyes, mouth and hat, plus a ghost for the fallen, without throwing", () => {
    const bodies = ["circle", "square", "rounded", "triangle", "hexagon", "diamond", "ghost"];
    const eyes = ["dots", "wide", "sleepy", "angry", "x", "hearts", "visor", "cyclops"];
    const mouths = ["smile", "grin", "flat", "frown", "o", "teeth", "tongue", "zigzag"];
    const hats = ["none", "antenna", "crown", "horns", "headband", "bow"];
    const bots = bodies.map((body, i) => ({
      id: `b${i}`, x: i % 4, y: Math.floor(i / 4), color: "#4488cc", name: "n", edited: false, shielded: false, mine: false, hit: i === 1,
      hp: 3, maxHp: 4, charge: i === 2 ? 0.5 : 1, body, eyes: eyes[i]!, mouth: mouths[i]!, hat: hats[i % hats.length]!,
    }));
    const fallen = [{ id: "f", x: 4, y: 4, color: "#ffee00", name: "f", label: "respawn 3", dying: false, body: "ghost", eyes: "x", mouth: "flat", hat: "crown" }];
    const { canvas, calls } = fakeCanvas();
    const r = new Canvas2dRenderer(canvas, 1);
    r.resize(500, 500);
    r.render({ ...scene, coins: [], taken: [], rebooting: [], radars: [], tracers: [], bots, fallen, effect: 0.2 });
    expect(calls.filter((c) => c === "stroke:0").length).toBeGreaterThan(bots.length * 3);
    expect(calls).toContain("clip:0");
    expect(calls).toContain("quadraticCurveTo:4");
  });
});

/** A context recording text draws and filled rects with their values. */
function recordingCanvas() {
  const texts: { s: string; x: number; y: number; font: string }[] = [];
  const rects: { x: number; y: number; w: number; h: number }[] = [];
  const props: Record<string, unknown> = {};
  const ctx = new Proxy({}, {
    get: (_t, k: string) => {
      if (k === "measureText") return (s: string) => ({ width: s.length * 6 });
      if (k === "fillText") return (s: string, x: number, y: number) => { texts.push({ s, x, y, font: String(props.font) }); };
      if (k === "fillRect") return (x: number, y: number, w: number, h: number) => { rects.push({ x, y, w, h }); };
      return k in props ? props[k] : () => undefined;
    },
    set: (_t, k: string, v) => { props[k] = v; return true; },
  });
  const canvas = { width: 0, height: 0, style: {} as Record<string, string>, getContext: () => ctx } as unknown as HTMLCanvasElement;
  return { canvas, texts, rects };
}

describe("Canvas2dRenderer layout", () => {
  const cell = 50;
  const base: Scene = { ...scene, walls: [], coins: [], taken: [], rebooting: [], radars: [], tracers: [], bots: [], fallen: [], effect: 1 };

  it("does not draw a near-black bot's name, countdown or dying name in its raw color", () => {
    const fills: Record<string, string[]> = {};
    const props: Record<string, unknown> = {};
    const ctx = new Proxy({}, {
      get: (_t, k: string) => {
        if (k === "measureText") return (s: string) => ({ width: s.length * 6 });
        if (k === "fillText") return (s: string) => { (fills[s] ??= []).push(String(props.fillStyle)); };
        return k in props ? props[k] : () => undefined;
      },
      set: (_t, k: string, v) => { props[k] = v; return true; },
    });
    const canvas = { width: 0, height: 0, style: {} as Record<string, string>, getContext: () => ctx } as unknown as HTMLCanvasElement;
    const r = new Canvas2dRenderer(canvas, 1);
    r.resize(5 * cell, 5 * cell);
    const bot = { ...scene.bots[1]!, id: "K", color: "#000000", name: "KBOT" };
    r.render({ ...base, bots: [bot], fallen: [{ id: "F", x: 3, y: 3, color: "#000000", name: "FALL", ...FIG, label: "9", dying: false }, { id: "G", x: 1, y: 3, color: "#000000", name: "DYING", ...FIG, label: "r", dying: true }], effect: 0 });
    for (const t of ["KBOT", "9", "DYING"]) {
      expect(fills[t], t).toBeDefined();
      expect(fills[t]!.every((f) => f !== "#000000"), t).toBe(true);
    }
  });

  it("draws a fallen bot's countdown as a bare number inside its cell", () => {
    const { canvas, texts } = recordingCanvas();
    const r = new Canvas2dRenderer(canvas, 1);
    r.resize(5 * cell, 5 * cell);
    r.render({ ...base, fallen: [{ id: "B", x: 2, y: 2, color: "#0f0", name: "B", ...FIG, label: "7", dying: false }] });
    const t = texts.find((x) => x.s === "7");
    expect(t).toBeDefined();
    expect(t!.x - 5).toBeGreaterThanOrEqual(2 * cell);
    expect(t!.x + 5).toBeLessThanOrEqual(3 * cell);
    expect(t!.y).toBeGreaterThan(2 * cell);
    expect(t!.y).toBeLessThanOrEqual(3 * cell);
    expect(Number.parseInt(t!.font, 10)).toBeLessThanOrEqual(14);
  });

  it("puts the health bar below every body", () => {
    const { canvas, rects } = recordingCanvas();
    const r = new Canvas2dRenderer(canvas, 1);
    r.resize(5 * cell, 5 * cell);
    r.render({ ...base, bots: [{ ...scene.bots[0]!, x: 1, y: 1, hit: false, hp: 4, maxHp: 4 }] });
    const bar = rects.filter((x) => x.h < cell * 0.2 && x.w > cell * 0.5);
    expect(bar.length).toBeGreaterThan(0);
    const top = Math.min(...bar.map((x) => x.y));
    expect(top).toBeGreaterThanOrEqual(cell + BAR_TOP * cell - 1.5);
    expect(Math.max(...bar.map((x) => x.y + x.h))).toBeLessThanOrEqual(2 * cell + 1);
    expect(BAR_H).toBeGreaterThan(0);
    expect(BOT_R).toBeGreaterThan(0);
  });
});
