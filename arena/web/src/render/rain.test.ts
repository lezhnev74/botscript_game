import { describe, expect, it } from "vitest";
import { RAIN_COLS, RAIN_TRAIL, mix, rainGlyphs, seedOf } from "./rain";

const snapshot = (seed: number, now: number): string => {
  const { glyphs, count } = rainGlyphs(seed, now);
  return JSON.stringify(glyphs.slice(0, count));
};

describe("rainGlyphs", () => {
  it("is a pure function of the seed and the clock", () => {
    expect(snapshot(7, 1234)).toBe(snapshot(7, 1234));
    expect(snapshot(7, 1234)).not.toBe(snapshot(7, 4321));
    expect(snapshot(7, 1234)).not.toBe(snapshot(8, 1234));
  });

  it("keeps every glyph inside the cell with a brightness in (0, 1]", () => {
    for (let now = 0; now < 5000; now += 130) {
      const { glyphs, count } = rainGlyphs(seedOf("A"), now);
      expect(count).toBeGreaterThan(0);
      expect(count).toBeLessThanOrEqual(RAIN_COLS * RAIN_TRAIL);
      glyphs.slice(0, count).forEach((g) => {
        expect(g.x).toBeGreaterThan(0);
        expect(g.x).toBeLessThan(1);
        expect(g.y).toBeGreaterThan(0);
        expect(g.y).toBeLessThan(1);
        expect(g.alpha).toBeGreaterThan(0);
        expect(g.alpha).toBeLessThanOrEqual(1);
        expect(g.ch).toHaveLength(1);
      });
    }
  });

  it("fills one shared buffer, so a frame allocates no glyphs", () => {
    expect(rainGlyphs(1, 0).glyphs).toBe(rainGlyphs(2, 999).glyphs);
  });

  it("rains down: a streak's head moves to a lower row as time passes", () => {
    const lowest = (now: number): number => {
      const { glyphs, count } = rainGlyphs(3, now);
      return Math.max(...glyphs.slice(0, count).filter((g) => g.alpha === 1 && g.x < 0.3).map((g) => g.y), -1);
    };
    const seen = new Set([0, 1, 2, 3, 4, 5, 6, 7].map((i) => lowest(i * 260)));
    expect(seen.size).toBeGreaterThan(3); // the head visits many rows over a fall
  });
});

describe("hashes", () => {
  it("mix and seedOf are stable and spread", () => {
    expect(mix(1, 2, 3)).toBe(mix(1, 2, 3));
    expect(mix(1, 2, 3)).not.toBe(mix(1, 2, 4));
    expect(seedOf("A")).not.toBe(seedOf("B"));
  });
});
