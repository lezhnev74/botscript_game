// Pure path data for a bot's look. Unit space: the body fits the circle of radius 1 around (0, 0), y grows
// down. The renderer scales it by the bot radius. No canvas here.

export type Cmd =
  | readonly ["M", number, number]
  | readonly ["L", number, number]
  | readonly ["Q", number, number, number, number]
  | readonly ["A", number, number, number, number, number]
  | readonly ["Z"];

/** One thing to paint with the ink: a filled shape, or a stroke of `w` (fraction of the body radius, default 0.12). */
export interface Draw { kind: "fill" | "stroke"; cmds: Cmd[]; w?: number }

export const BODIES = ["circle", "square", "rounded", "triangle", "hexagon", "diamond", "ghost"] as const;
export const EYES = ["dots", "wide", "sleepy", "angry", "x", "hearts", "visor", "cyclops"] as const;
export const MOUTHS = ["smile", "grin", "flat", "frown", "o", "teeth", "tongue", "zigzag"] as const;
export const HATS = ["none", "antenna", "crown", "horns", "headband", "bow"] as const;

/** Cell layout (cell units): bot radius, body centre height, name label centre and its conservative bottom edge. */
export const BOT_R = 0.33;
export const BOT_DY = 0.58;
export const LABEL_Y = 0.08;
export const LABEL_BOTTOM = 0.19;
/** The health bar: its top edge and height (cell units), flush with the cell bottom, below every body. */
export const BAR_TOP = 0.92;
export const BAR_H = 0.06;

const TAU = Math.PI * 2;
const M = (x: number, y: number): Cmd => ["M", x, y];
const L = (x: number, y: number): Cmd => ["L", x, y];
const Q = (cx: number, cy: number, x: number, y: number): Cmd => ["Q", cx, cy, x, y];
const Z: Cmd = ["Z"];

const poly = (pts: [number, number][]): Cmd[] => [M(...pts[0]!), ...pts.slice(1).map((p) => L(...p)), Z];
const circle = (x: number, y: number, r: number): Cmd[] => [M(x + r, y), ["A", x, y, r, 0, TAU], Z];
const rect = (x0: number, y0: number, x1: number, y1: number): Cmd[] => poly([[x0, y0], [x1, y0], [x1, y1], [x0, y1]]);
const fill = (cmds: Cmd[]): Draw => ({ kind: "fill", cmds });
const stroke = (cmds: Cmd[], w?: number): Draw => ({ kind: "stroke", cmds, ...(w === undefined ? {} : { w }) });

function rounded(h: number, r: number): Cmd[] {
  return [M(-h + r, -h), L(h - r, -h), Q(h, -h, h, -h + r), L(h, h - r), Q(h, h, h - r, h), L(-h + r, h), Q(-h, h, -h, h - r), L(-h, -h + r), Q(-h, -h, -h + r, -h), Z];
}

const GHOST: Cmd[] = [
  M(-0.95, 0), ["A", 0, 0, 0.95, Math.PI, TAU], L(0.95, 0.9),
  Q(0.63, 0.6, 0.32, 0.9), Q(0, 0.6, -0.32, 0.9), Q(-0.63, 0.6, -0.95, 0.9), Z,
];

/** Body outlines, all inside the unit circle. */
const BODY_PATH: Record<string, Cmd[]> = {
  circle: circle(0, 0, 1),
  square: rect(-0.88, -0.88, 0.88, 0.88),
  rounded: rounded(0.92, 0.38),
  triangle: poly([[0, -0.95], [1, 0.85], [-1, 0.85]]),
  hexagon: poly([[1, 0], [0.5, 0.86], [-0.5, 0.86], [-1, 0], [-0.5, -0.86], [0.5, -0.86]]),
  diamond: poly([[0, -1], [1, 0], [0, 1], [-1, 0]]),
  ghost: GHOST,
};
/** Top edge (unit y) of each body: where a hat sits. */
const BODY_TOP: Record<string, number> = { circle: -1, square: -0.88, rounded: -0.92, triangle: -0.95, hexagon: -0.86, diamond: -1, ghost: -0.95 };

export const bodyPath = (name: string): Cmd[] => BODY_PATH[name] ?? BODY_PATH.circle!;
export const bodyTop = (name: string): number => BODY_TOP[name] ?? BODY_TOP.circle!;

const EX = 0.33;
const EY = -0.15;
const heart = (cx: number, cy: number, s: number): Draw[] => [
  fill(circle(cx - 0.5 * s, cy - 0.2 * s, 0.55 * s)), fill(circle(cx + 0.5 * s, cy - 0.2 * s, 0.55 * s)),
  fill(poly([[cx - 1.02 * s, cy - 0.05 * s], [cx + 1.02 * s, cy - 0.05 * s], [cx, cy + s]])),
];
const cross = (cx: number, cy: number, s: number): Draw =>
  stroke([M(cx - s, cy - s), L(cx + s, cy + s), M(cx + s, cy - s), L(cx - s, cy + s)]);

const EYE_DRAW: Record<string, Draw[]> = {
  dots: [fill(circle(-EX, EY, 0.11)), fill(circle(EX, EY, 0.11))],
  wide: [-EX, EX].flatMap((x) => [stroke(circle(x, EY, 0.19), 0.08), fill(circle(x, EY, 0.07))]),
  sleepy: [stroke([M(-0.5, EY), L(-0.16, EY)]), stroke([M(0.16, EY), L(0.5, EY)])],
  angry: [fill(circle(-EX, EY + 0.05, 0.09)), fill(circle(EX, EY + 0.05, 0.09)), stroke([M(-0.55, -0.42), L(-0.15, -0.26)]), stroke([M(0.55, -0.42), L(0.15, -0.26)])],
  x: [cross(-EX, EY, 0.13), cross(EX, EY, 0.13)],
  // Internal (a wince on a hit, not pickable): squeezed ">" "<" eyes, so a hit differs from the dead x.
  squint: [stroke([M(-EX - 0.1, EY - 0.12), L(-EX + 0.1, EY), L(-EX - 0.1, EY + 0.12)]), stroke([M(EX + 0.1, EY - 0.12), L(EX - 0.1, EY), L(EX + 0.1, EY + 0.12)])],
  hearts: [...heart(-EX, EY, 0.17), ...heart(EX, EY, 0.17)],
  visor: [fill(rect(-0.62, EY - 0.14, 0.62, EY + 0.12))],
  cyclops: [stroke(circle(0, EY, 0.26), 0.09), fill(circle(0, EY, 0.1))],
};

const MY = 0.42;
const MOUTH_DRAW: Record<string, Draw[]> = {
  smile: [stroke([M(-0.3, MY - 0.1), Q(0, MY + 0.3, 0.3, MY - 0.1)])],
  grin: [fill([M(-0.36, MY - 0.15), L(0.36, MY - 0.15), Q(0, MY + 0.5, -0.36, MY - 0.15), Z])],
  flat: [stroke([M(-0.28, MY), L(0.28, MY)])],
  frown: [stroke([M(-0.3, MY + 0.12), Q(0, MY - 0.28, 0.3, MY + 0.12)])],
  o: [stroke(circle(0, MY, 0.14), 0.09)],
  teeth: [stroke(rect(-0.3, MY - 0.14, 0.3, MY + 0.16), 0.07), stroke([M(-0.3, MY + 0.01), L(0.3, MY + 0.01), M(-0.1, MY - 0.14), L(-0.1, MY + 0.16), M(0.1, MY - 0.14), L(0.1, MY + 0.16)], 0.05)],
  tongue: [stroke([M(-0.3, MY - 0.1), Q(0, MY + 0.2, 0.3, MY - 0.1)]), fill([M(0, MY + 0.05), L(0.2, MY + 0.05), Q(0.2, MY + 0.4, 0.1, MY + 0.4), Q(0, MY + 0.4, 0, MY + 0.05), Z])],
  zigzag: [stroke([M(-0.36, MY + 0.05), L(-0.18, MY - 0.1), L(0, MY + 0.05), L(0.18, MY - 0.1), L(0.36, MY + 0.05)])],
};

/** Hats sit on the body top `t` and rise at most 0.2 above it, so the name label above stays clear. */
const HAT_DRAW: Record<string, (t: number) => Draw[]> = {
  none: () => [],
  antenna: (t) => [stroke([M(0, t + 0.05), L(0, t - 0.08)], 0.08), fill(circle(0, t - 0.1, 0.08))],
  crown: (t) => [fill(poly([[-0.5, t + 0.15], [-0.5, t - 0.14], [-0.25, t], [0, t - 0.18], [0.25, t], [0.5, t - 0.14], [0.5, t + 0.15]]))],
  horns: (t) => [fill(poly([[-0.7, t + 0.25], [-0.72, t - 0.18], [-0.25, t + 0.05]])), fill(poly([[0.7, t + 0.25], [0.72, t - 0.18], [0.25, t + 0.05]]))],
  headband: (t) => [stroke([M(-0.6, t + 0.3), L(0.6, t + 0.3)], 0.16), fill(poly([[0.6, t + 0.3], [0.9, t + 0.12], [0.85, t + 0.34]]))],
  bow: (t) => [fill(poly([[0, t], [-0.42, t - 0.18], [-0.42, t + 0.14]])), fill(poly([[0, t], [0.42, t - 0.18], [0.42, t + 0.14]])), fill(circle(0, t, 0.09))],
};

/** Where the face goes on a body: scaled by `s` and shifted down by `dy` (unit space), so every part stays inside narrow bodies. */
const FACE_FIT: Record<string, { s: number; dy: number }> = {
  triangle: { s: 0.62, dy: 0.2 },
  diamond: { s: 0.8, dy: 0.02 },
  ghost: { s: 0.9, dy: -0.02 },
};

function fitCmd(c: Cmd, s: number, dy: number): Cmd {
  if (c[0] === "M" || c[0] === "L") return [c[0], c[1] * s, c[2] * s + dy];
  if (c[0] === "Q") return ["Q", c[1] * s, c[2] * s + dy, c[3] * s, c[4] * s + dy];
  if (c[0] === "A") return ["A", c[1] * s, c[2] * s + dy, c[3] * s, c[4], c[5]];
  return c;
}

function fitted(ds: Draw[], body: string): Draw[] {
  const f = FACE_FIT[body];
  return f ? ds.map((d) => ({ ...d, cmds: d.cmds.map((c) => fitCmd(c, f.s, f.dy)) })) : ds;
}

export const eyesDraw = (name: string, body = "circle"): Draw[] => fitted(EYE_DRAW[name] ?? EYE_DRAW.dots!, body);
export const mouthDraw = (name: string, body = "circle"): Draw[] => fitted(MOUTH_DRAW[name] ?? MOUTH_DRAW.smile!, body);
export const hatDraw = (name: string, bodyName: string): Draw[] => (HAT_DRAW[name] ?? HAT_DRAW.none!)(bodyTop(bodyName));

export interface Box { x0: number; y0: number; x1: number; y1: number }

/** Bounding box of path commands (an arc counts as its whole circle, a curve by its control points: conservative). */
export function bounds(cmds: Cmd[]): Box {
  const b: Box = { x0: Infinity, y0: Infinity, x1: -Infinity, y1: -Infinity };
  const add = (x: number, y: number) => {
    b.x0 = Math.min(b.x0, x); b.y0 = Math.min(b.y0, y); b.x1 = Math.max(b.x1, x); b.y1 = Math.max(b.y1, y);
  };
  cmds.forEach((c) => {
    if (c[0] === "M" || c[0] === "L") add(c[1], c[2]);
    else if (c[0] === "Q") { add(c[1], c[2]); add(c[3], c[4]); }
    else if (c[0] === "A") { add(c[1] - c[3], c[2] - c[3]); add(c[1] + c[3], c[2] + c[3]); }
  });
  return b;
}

const HEX = /^#([0-9a-f]{2})([0-9a-f]{2})([0-9a-f]{2})$/i;

function rgb(color: string): [number, number, number] {
  const m = HEX.exec(color);
  return m ? [parseInt(m[1]!, 16), parseInt(m[2]!, 16), parseInt(m[3]!, 16)] : [154, 160, 166];
}

const INK_DARK = "#14161c";
const INK_LIGHT = "#f5f6fa";

/** Face and hat ink: dark on light colors, light on dark ones (WCAG relative luminance). */
export function inkFor(color: string): string {
  return luminance(color) > 0.179 ? INK_DARK : INK_LIGHT;
}

/** The color a body really shows: a charging body is drawn dim over the floor, so blend toward it (full charge: as is). */
export function shownColor(color: string, charge: number, floor: string): string {
  if (charge >= 1) return color;
  const a = 0.25 + 0.75 * Math.max(0, charge);
  const c = rgb(color);
  const f = rgb(floor);
  const hex = (i: 0 | 1 | 2) => Math.round(c[i] * a + f[i] * (1 - a)).toString(16).padStart(2, "0");
  return `#${hex(0)}${hex(1)}${hex(2)}`;
}

const FLOOR_COLOR = "#1c2230";

function luminance(color: string): number {
  const [r, g, b] = rgb(color).map((v) => {
    const s = v / 255;
    return s <= 0.03928 ? s / 12.92 : ((s + 0.055) / 1.055) ** 2.4;
  }) as [number, number, number];
  return 0.2126 * r + 0.7152 * g + 0.0722 * b;
}

/** The body outline color: the fill at half brightness, or a light rim when the color is about as dark as the floor. */
export function outlineFor(color: string, floor: string = FLOOR_COLOR): string {
  if (luminance(color) - luminance(floor) < 0.05) return "rgba(245, 246, 250, 0.75)";
  const [r, g, b] = rgb(color).map((v) => Math.round(v * 0.5));
  return `rgb(${r}, ${g}, ${b})`;
}

const MIN_TEXT_GAP = 0.15;

/** A readable text color for a bot color on a dark background: the color itself when bright enough, else lifted toward white (hue kept as far as possible). */
export function textColorFor(color: string, bg: string = FLOOR_COLOR): string {
  const need = luminance(bg) + MIN_TEXT_GAP;
  const c = rgb(color);
  for (let t = 0; t <= 1; t += 0.05) {
    const [r, g, b] = c.map((v) => Math.round(v + (255 - v) * t)) as [number, number, number];
    const hex = `#${[r, g, b].map((v) => v.toString(16).padStart(2, "0")).join("")}`;
    if (luminance(hex) >= need) return t === 0 ? color : hex;
  }
  return "#ffffff";
}
