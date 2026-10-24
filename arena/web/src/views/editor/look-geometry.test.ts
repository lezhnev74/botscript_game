import { describe, expect, it } from "vitest";
import { FACE_ZOOM, tileGeometry } from "./look-geometry";

describe("tileGeometry", () => {
  it("body and hat tiles draw the whole bot", () => {
    for (const part of ["body", "hat", undefined] as const) {
      expect(tileGeometry(44, part)).toEqual({ cx: 22, cy: 44 * 0.56, size: 44 * 0.6 });
    }
  });
  it("eyes and mouth tiles zoom so the feature is centred", () => {
    const eyes = tileGeometry(44, "eyes");
    const mouth = tileGeometry(44, "mouth");
    for (const g of [eyes, mouth]) {
      expect(g.size).toBeCloseTo(44 * FACE_ZOOM);
      expect(g.size).toBeGreaterThan(44);
      expect(g.cx).toBe(22);
    }
    expect(eyes.cy + -0.15 * (eyes.size / 2)).toBeCloseTo(22);
    expect(mouth.cy + 0.42 * (mouth.size / 2)).toBeCloseTo(22);
  });
});
