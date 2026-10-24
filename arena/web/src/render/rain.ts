// Matrix rain for a rebooting bot: a few columns of falling glyphs inside one cell. Pure and deterministic
// (a function of the seed and the clock), and it fills one preallocated buffer, so a frame allocates nothing.

/** One glyph: position in the cell (0..1 from its top-left), the character and its brightness 0..1. */
export interface RainGlyph {
  x: number;
  y: number;
  ch: string;
  alpha: number;
}

/** Columns of rain per cell, glyph rows per column and glyphs in a falling streak. */
export const RAIN_COLS = 4;
export const RAIN_ROWS = 5;
export const RAIN_TRAIL = 3;

const GLYPHS = "01ABCDEF<>/\\|=+*#$%";
/** Milliseconds a streak needs to fall one row (slowest column; faster ones are up to twice as quick). */
const ROW_MS = 260;
/** How often a glyph changes into another one. */
const FLICKER_MS = 180;

const buffer: RainGlyph[] = Array.from({ length: RAIN_COLS * RAIN_TRAIL }, () => ({ x: 0, y: 0, ch: " ", alpha: 0 }));

/** A small integer hash of three numbers (uint32). */
export function mix(a: number, b: number, c: number): number {
  let h = Math.imul(a ^ 0x9e3779b9, 0x85ebca6b) ^ Math.imul(b + 0x7f4a7c15, 0xc2b2ae35) ^ Math.imul(c + 0x165667b1, 0x27d4eb2f);
  h ^= h >>> 15;
  h = Math.imul(h, 0x2c1b3c6d);
  h ^= h >>> 12;
  return h >>> 0;
}

/** A number for any seed string (a bot id), so each cell rains differently. */
export function seedOf(id: string): number {
  let h = 17;
  for (let i = 0; i < id.length; i++) h = Math.imul(h ^ id.charCodeAt(i), 0x01000193);
  return h >>> 0;
}

/**
 * The glyphs at time `now` (ms), in a shared buffer that the next call overwrites: use the first `count`
 * entries at once. The head of each streak is the brightest and the trail fades behind it.
 */
export function rainGlyphs(seed: number, now: number): { glyphs: readonly RainGlyph[]; count: number } {
  let n = 0;
  for (let c = 0; c < RAIN_COLS; c++) {
    const speed = 1 + (mix(seed, c, 1) % 100) / 100;
    const phase = mix(seed, c, 2) % 1000;
    const cycle = RAIN_ROWS + RAIN_TRAIL;
    const head = Math.floor((now * speed) / ROW_MS + phase) % cycle;
    for (let t = 0; t < RAIN_TRAIL; t++) {
      const row = head - t;
      if (row < 0 || row >= RAIN_ROWS) continue;
      const g = buffer[n++]!;
      g.x = (c + 0.5) / RAIN_COLS;
      g.y = (row + 0.5) / RAIN_ROWS;
      g.ch = GLYPHS[mix(seed, c * 31 + row, Math.floor(now / FLICKER_MS)) % GLYPHS.length]!;
      g.alpha = 1 - t / RAIN_TRAIL;
    }
  }
  return { glyphs: buffer, count: n };
}
