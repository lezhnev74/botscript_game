import { describe, expect, it } from "vitest";
import { fitCells } from "./fit";

describe("fitCells", () => {
  it("is width-bound for a wide map and centres vertically", () => {
    expect(fitCells(900, 800, 9, 3)).toEqual({ cell: 100, offsetX: 0, offsetY: 250 });
  });
  it("is height-bound for a tall map and centres horizontally", () => {
    expect(fitCells(800, 600, 3, 9)).toEqual({ cell: 600 / 9, offsetX: (800 - 3 * (600 / 9)) / 2, offsetY: 0 });
  });
  it("fills a square map in a square stage", () => {
    expect(fitCells(500, 500, 5, 5)).toEqual({ cell: 100, offsetX: 0, offsetY: 0 });
  });
  it("never exceeds the stage and handles an empty map", () => {
    const f = fitCells(1000, 300, 9, 9);
    expect(f.cell * 9).toBeLessThanOrEqual(300);
    expect(fitCells(100, 100, 0, 0)).toEqual({ cell: 0, offsetX: 0, offsetY: 0 });
  });
});
