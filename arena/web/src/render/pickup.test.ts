import { describe, expect, it } from "vitest";
import { glowAlpha, shakeOffset, SHAKE_AMPLITUDE } from "./pickup";

describe("coin pickup effect", () => {
  it("shake is 0 at effect 0 and 1 and stays within the amplitude", () => {
    expect(shakeOffset(0)).toBe(0);
    expect(Math.abs(shakeOffset(1))).toBeLessThan(1e-9);
    let max = 0;
    for (let i = 0; i <= 100; i++) max = Math.max(max, Math.abs(shakeOffset(i / 100)));
    expect(max).toBeLessThanOrEqual(SHAKE_AMPLITUDE);
    expect(max).toBeGreaterThan(SHAKE_AMPLITUDE / 2);
  });
  it("glow peaks early and is 0 at the end", () => {
    expect(glowAlpha(1)).toBe(0);
    expect(glowAlpha(0.2)).toBeGreaterThan(glowAlpha(0.8));
    expect(glowAlpha(0.2)).toBeGreaterThan(0);
    expect(glowAlpha(0.2)).toBeLessThanOrEqual(1);
  });
});
