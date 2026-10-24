import { describe, expect, it } from "vitest";
import { BODIES, BOT_DY, BOT_R, bodyPath, bodyTop, bounds, EYES, eyesDraw, HATS, hatDraw, BAR_H, BAR_TOP, inkFor, LABEL_BOTTOM, MOUTHS, mouthDraw, outlineFor, shownColor, textColorFor, type Cmd } from "./shapes";

describe("body paths", () => {
  it.each(BODIES)("%s stays inside the unit circle and the cell", (name) => {
    const b = bounds(bodyPath(name));
    expect(Math.min(b.x0, b.y0)).toBeGreaterThanOrEqual(-1);
    expect(Math.max(b.x1, b.y1)).toBeLessThanOrEqual(1);
    expect(b.x1 - b.x0).toBeGreaterThan(1.4);
    expect(bodyTop(name)).toBeCloseTo(b.y0, 1);
  });
  it("unknown names fall back to the circle", () => {
    expect(bodyPath("blob")).toEqual(bodyPath("circle"));
  });
});

/** The body outline as a dense polygon (arcs and curves flattened). */
function flatten(cmds: Cmd[]): [number, number][] {
  const pts: [number, number][] = [];
  let cur: [number, number] = [0, 0];
  cmds.forEach((c) => {
    if (c[0] === "M" || c[0] === "L") { cur = [c[1], c[2]]; pts.push(cur); }
    else if (c[0] === "Q") {
      for (let i = 1; i <= 16; i++) {
        const t = i / 16;
        pts.push([(1 - t) ** 2 * cur[0] + 2 * (1 - t) * t * c[1] + t * t * c[3], (1 - t) ** 2 * cur[1] + 2 * (1 - t) * t * c[2] + t * t * c[4]]);
      }
      cur = [c[3], c[4]];
    } else if (c[0] === "A") {
      for (let i = 0; i <= 64; i++) { const a = c[4] + ((c[5] - c[4]) * i) / 64; pts.push([c[1] + c[3] * Math.cos(a), c[2] + c[3] * Math.sin(a)]); }
    }
  });
  return pts;
}

function inside(poly: [number, number][], x: number, y: number): boolean {
  let in_ = false;
  for (let i = 0, j = poly.length - 1; i < poly.length; j = i++) {
    const [xi, yi] = poly[i]!;
    const [xj, yj] = poly[j]!;
    if (yi > y !== yj > y && x < ((xj - xi) * (y - yi)) / (yj - yi) + xi) in_ = !in_;
  }
  return in_;
}

/** True extent of a draw (curves flattened) grown by half its stroke width. */
function extent(d: { kind: string; cmds: Cmd[]; w?: number }): { x0: number; y0: number; x1: number; y1: number } {
  const pts = flatten(d.cmds);
  const m = d.kind === "stroke" ? (d.w ?? 0.12) / 2 : 0;
  return { x0: Math.min(...pts.map((p) => p[0])) - m, y0: Math.min(...pts.map((p) => p[1])) - m, x1: Math.max(...pts.map((p) => p[0])) + m, y1: Math.max(...pts.map((p) => p[1])) + m };
}

describe("face parts", () => {
  it.each(BODIES)("every eyes x mouth combination fits inside %s", (body) => {
    const poly = flatten(bodyPath(body));
    EYES.forEach((e) => MOUTHS.forEach((m) => {
      [...eyesDraw(e, body), ...mouthDraw(m, body)].forEach((d) => {
        const b = extent(d);
        [[b.x0, b.y0], [b.x1, b.y0], [b.x0, b.y1], [b.x1, b.y1]].forEach(([x, y]) => {
          expect(inside(poly, x!, y!), `${body} ${e} ${m} corner ${x!.toFixed(2)},${y!.toFixed(2)}`).toBe(true);
        });
      });
    }));
  });
  it("the wince eyes are internal (not pickable) and differ from x", () => {
    expect(EYES as readonly string[]).not.toContain("squint");
    expect(JSON.stringify(eyesDraw("squint"))).not.toBe(JSON.stringify(eyesDraw("x")));
    expect(JSON.stringify(eyesDraw("squint"))).not.toBe(JSON.stringify(eyesDraw("dots")));
  });
  it.each([...EYES])("eyes %s draw something above the mouth line", (n) => {
    const ds = eyesDraw(n);
    expect(ds.length).toBeGreaterThan(0);
    ds.forEach((d) => expect(bounds(d.cmds).y1).toBeLessThan(0.3));
  });
  it.each([...MOUTHS])("mouth %s draws something below the eyes", (n) => {
    const ds = mouthDraw(n);
    expect(ds.length).toBeGreaterThan(0);
    ds.forEach((d) => expect(bounds(d.cmds).y0).toBeGreaterThan(0));
  });
  it("every part differs from the others", () => {
    expect(new Set(EYES.map((n) => JSON.stringify(eyesDraw(n)))).size).toBe(EYES.length);
    expect(new Set(MOUTHS.map((n) => JSON.stringify(mouthDraw(n)))).size).toBe(MOUTHS.length);
  });
});

describe("hats", () => {
  it("none draws nothing", () => {
    expect(hatDraw("none", "circle")).toEqual([]);
  });
  it.each(HATS.filter((h) => h !== "none"))("%s sits on the body top and rises at most 0.2 above it", (h) => {
    BODIES.forEach((body) => {
      const top = bodyTop(body);
      const ys = hatDraw(h, body).map((d) => bounds(d.cmds));
      expect(ys.length).toBeGreaterThan(0);
      expect(Math.min(...ys.map((b) => b.y0))).toBeGreaterThanOrEqual(top - 0.2);
      expect(Math.min(...ys.map((b) => b.y0))).toBeLessThan(top + 0.3);
    });
  });
  it.each(HATS)("%s stays below the name label on every body", (h) => {
    BODIES.forEach((body) => {
      hatDraw(h, body).forEach((d) => {
        const w = d.kind === "stroke" ? (d.w ?? 0.12) / 2 : 0;
        expect(BOT_DY + BOT_R * (bounds(d.cmds).y0 - w)).toBeGreaterThanOrEqual(LABEL_BOTTOM);
      });
    });
  });
});

describe("layout", () => {
  it("no body bottom reaches the health bar, and the bar stays in the cell", () => {
    expect(BAR_TOP + BAR_H).toBeLessThanOrEqual(1);
    BODIES.forEach((body) => {
      expect(BOT_DY + BOT_R * bounds(bodyPath(body)).y1).toBeLessThanOrEqual(BAR_TOP);
    });
  });
  it("bots stay big and every body stays inside the cell", () => {
    expect(BOT_R).toBeGreaterThanOrEqual(0.33);
    BODIES.forEach((body) => {
      const b = bounds(bodyPath(body));
      expect(BOT_DY + BOT_R * b.y0).toBeGreaterThanOrEqual(0);
      expect(BOT_DY + BOT_R * b.y1).toBeLessThanOrEqual(1);
      expect(0.5 + BOT_R * b.x0).toBeGreaterThanOrEqual(0);
      expect(0.5 + BOT_R * b.x1).toBeLessThanOrEqual(1);
    });
  });
});

describe("ink", () => {
  it("is dark on light colors and light on dark ones", () => {
    expect(inkFor("#ffff00")).toBe("#14161c");
    expect(inkFor("#ffffff")).toBe("#14161c");
    expect(inkFor("#000000")).toBe("#f5f6fa");
    expect(inkFor("#202060")).toBe("#f5f6fa");
  });
  it("shownColor blends a charging body over the floor, so its ink follows what is seen", () => {
    expect(shownColor("#ffff00", 1, "#1c2230")).toBe("#ffff00");
    expect(inkFor(shownColor("#ffff00", 1, "#1c2230"))).toBe("#14161c");
    expect(inkFor(shownColor("#ffff00", 0, "#1c2230"))).toBe("#f5f6fa");
    expect(inkFor(shownColor("#ffffff", 0.05, "#1c2230"))).toBe("#f5f6fa");
  });
  it("outline is a darker shade", () => {
    expect(outlineFor("#ff8040")).toBe("rgb(128, 64, 32)");
  });
  it("outline turns light when the color is as dark as the floor, and stays a darker shade otherwise", () => {
    expect(outlineFor("#1c2a6e", "#1c2230")).toMatch(/^rgba\(245, 246, 250/);
    expect(outlineFor("#050505", "#1c2230")).toMatch(/^rgba\(245/);
    expect(outlineFor("#ff8040", "#1c2230")).toBe("rgb(128, 64, 32)");
    expect(outlineFor("#ffffff", "#1c2230")).toBe("rgb(128, 128, 128)");
  });
  it("copes with a malformed color", () => {
    expect(inkFor("nope")).toMatch(/^#/);
  });
});

describe("textColorFor", () => {
  it("lifts near-black and navy toward light", () => {
    expect(textColorFor("#000000")).not.toBe("#000000");
    expect(textColorFor("#1c2a6e")).not.toBe("#1c2a6e");
    expect(textColorFor("#1c2a6e")).toMatch(/^#[0-9a-f]{6}$/);
    const [r, g, b] = [1, 3, 5].map((i) => parseInt(textColorFor("#1c2a6e").slice(i, i + 2), 16));
    expect(b!).toBeGreaterThan(r!);
    expect(b!).toBeGreaterThan(g!);
  });
  it("keeps bright colors", () => {
    expect(textColorFor("#ffffff")).toBe("#ffffff");
    expect(textColorFor("#ff8040")).toBe("#ff8040");
  });
});
