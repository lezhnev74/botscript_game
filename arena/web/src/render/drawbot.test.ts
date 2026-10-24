import { describe, expect, it } from "vitest";
import { drawBot, traceCmds } from "./drawbot";
import { BODIES, BOT_R, EYES, HATS, MOUTHS } from "./shapes";

function recorder() {
  const calls: string[] = [];
  const ctx = new Proxy({}, { get: (_t, k: string) => (...a: unknown[]) => { calls.push(`${k}:${a.length}`); }, set: () => true });
  return { ctx: ctx as unknown as CanvasRenderingContext2D, calls };
}

describe("drawBot", () => {
  it.each(BODIES)("paints a %s bot with every eyes, mouth and hat", (body) => {
    const { ctx, calls } = recorder();
    EYES.forEach((eyes) => MOUTHS.forEach((mouth) => HATS.forEach((hat) => drawBot(ctx, { color: "#336699", body, eyes, mouth, hat }, { eyes, mouth }, 20, 20, 30))));
    expect(calls.filter((c) => c.startsWith("fill:")).length).toBeGreaterThan(0);
    expect(calls.some((c) => c.startsWith("stroke:"))).toBe(true);
  });

  it("keeps the TV's outline proportion (0.04 of a cell for a 2*BOT_R body)", () => {
    const widths: number[] = [];
    const ctx = new Proxy({}, { get: () => () => undefined, set: (_t, k: string, v: number) => { if (k === "lineWidth") widths.push(v); return true; } });
    drawBot(ctx as unknown as CanvasRenderingContext2D, { color: "#336699", body: "circle", eyes: "dots", mouth: "smile", hat: "none" }, { eyes: "dots", mouth: "smile" }, 0, 0, 100);
    expect(widths[0]).toBeCloseTo(100 * 0.04 / (2 * BOT_R), 5);
  });

  it("scales unit commands around the centre", () => {
    const pts: number[][] = [];
    const ctx = { beginPath: () => undefined, moveTo: (x: number, y: number) => pts.push([x, y]), lineTo: (x: number, y: number) => pts.push([x, y]), closePath: () => undefined } as unknown as CanvasRenderingContext2D;
    traceCmds(ctx, [["M", -1, 0], ["L", 1, 1], ["Z"]], 10, 20, 5);
    expect(pts).toEqual([[5, 20], [15, 25]]);
  });
});
