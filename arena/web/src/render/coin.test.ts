import { describe, expect, it } from "vitest";
import { ACT_MS, BOB_PX, CYCLE_MS, HOP, actPhase, bobPhase, coinPose } from "./coin";

/** First time (ms) at which the coin's action window opens. */
const actStart = (x: number, y: number): number => (CYCLE_MS - actPhase(x, y)) % CYCLE_MS;

describe("coinPose", () => {
  it("gives neighbouring cells different phases", () => {
    expect(bobPhase(1, 1)).not.toBe(bobPhase(2, 1));
    expect(actPhase(1, 1)).not.toBe(actPhase(1, 2));
  });

  it.each([40, 80])("bobs within +-BOB_PX pixels at cell %i px and never shrinks outside actions", (cell) => {
    let peak = 0;
    for (let t = 0; t < 20000; t += 37) {
      if ((t + actPhase(3, 4)) % CYCLE_MS < ACT_MS) continue;
      const p = coinPose(3, 4, t, cell);
      peak = Math.max(peak, Math.abs(p.dy * cell));
      expect(Math.abs(p.dy * cell)).toBeLessThanOrEqual(BOB_PX + 1e-9);
      expect(p.sx).toBe(1);
    }
    expect(peak).toBeGreaterThan(1);
    expect(BOB_PX).toBeLessThanOrEqual(2);
  });

  it("is deterministic", () => {
    expect(coinPose(2, 5, 1234, 40)).toEqual(coinPose(2, 5, 1234, 40));
  });

  it("spins an even cell: scale is 1 at the start, ~0 at a quarter, 1 at half the window", () => {
    const s = actStart(2, 2);
    expect(coinPose(2, 2, s, 40).sx).toBeCloseTo(1, 5);
    expect(coinPose(2, 2, s + ACT_MS / 4, 40).sx).toBeCloseTo(0, 5);
    expect(coinPose(2, 2, s + ACT_MS / 2, 40).sx).toBeCloseTo(1, 5);
    expect(coinPose(2, 2, s + ACT_MS + 1, 40).sx).toBe(1);
  });

  it("hops an odd cell: rises by HOP at mid window, scale stays 1", () => {
    const s = actStart(2, 3);
    const mid = coinPose(2, 3, s + ACT_MS / 2, 40);
    expect(mid.sx).toBe(1);
    expect(mid.dy).toBeLessThan(-HOP + 0.05);
    expect(mid.dy).toBeGreaterThan(-HOP - 0.05);
  });

  it("repeats every CYCLE_MS in the action", () => {
    const s = actStart(2, 2);
    expect(coinPose(2, 2, s + CYCLE_MS + ACT_MS / 4, 40).sx).toBeCloseTo(0, 5);
  });
});
