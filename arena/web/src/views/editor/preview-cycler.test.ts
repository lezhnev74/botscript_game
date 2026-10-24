import { describe, expect, it } from "vitest";
import { PreviewCycler, type Timers } from "./preview-cycler";

function setup() {
  const steps: number[] = [];
  let tick: (() => void) | null = null;
  let cleared = 0;
  const timers: Timers = {
    set: (fn) => { tick = fn; return 1; },
    clear: () => { tick = null; cleared++; },
  };
  const c = new PreviewCycler({ steps: 4, ms: 700, timers, onStep: (s) => steps.push(s) });
  return { c, steps, tick: () => tick?.(), running: () => tick !== null, cleared: () => cleared };
}

describe("PreviewCycler", () => {
  it("hover advances at once and keeps cycling", () => {
    const s = setup();
    s.c.hover();
    expect(s.steps).toEqual([1]);
    s.tick(); s.tick(); s.tick();
    expect(s.steps).toEqual([1, 2, 3, 0]);
  });
  it("a second hover does not start a second timer", () => {
    const s = setup();
    s.c.hover(); s.c.hover();
    expect(s.steps).toEqual([1]);
  });
  it("leave stops the cycle and returns to resting", () => {
    const s = setup();
    s.c.hover();
    s.c.leave();
    expect(s.running()).toBe(false);
    expect(s.steps).toEqual([1, 0]);
  });
  it("tap advances one step", () => {
    const s = setup();
    s.c.tap(); s.c.tap();
    expect(s.steps).toEqual([1, 2]);
  });
  it("dispose clears the timer without drawing", () => {
    const s = setup();
    s.c.hover();
    s.c.dispose();
    expect(s.running()).toBe(false);
    expect(s.steps).toEqual([1]);
  });
  it("dispose without a running cycle clears nothing", () => {
    const s = setup();
    s.c.dispose();
    expect(s.cleared()).toBe(0);
  });
});
