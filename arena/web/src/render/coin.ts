/** Coin animation, pure: where a coin is drawn at time tMs. Render-side only; the sim never sees it. */
export interface CoinPose {
  /** Vertical offset in cells (negative = up). */
  dy: number;
  /** Horizontal scale 0..1 (a spin squeezes the coin through its edge). */
  sx: number;
}

/** A coin's idle bob: amplitude in screen pixels (subtle at any zoom) and period. */
export const BOB_PX = 1.5;
export const BOB_MS = 2400;
/** Every CYCLE_MS a coin spends ACT_MS on a spin (even cells) or a hop (odd cells). */
export const CYCLE_MS = 4000;
export const ACT_MS = 600;
export const HOP = 0.25;

/** Per-cell offsets so neighbours do not move in sync. */
export const bobPhase = (x: number, y: number): number => (((x * 7 + y * 13) % 17) / 17) * BOB_MS;
export const actPhase = (x: number, y: number): number => (((x * 31 + y * 17) % 11) / 11) * CYCLE_MS;

/** `cellPx`: cell size in pixels, to keep the bob at BOB_PX. */
export function coinPose(x: number, y: number, tMs: number, cellPx: number): CoinPose {
  const bob = (Math.sin(((tMs + bobPhase(x, y)) / BOB_MS) * Math.PI * 2) * BOB_PX) / cellPx;
  const u = (tMs + actPhase(x, y)) % CYCLE_MS;
  if (u >= ACT_MS) return { dy: bob, sx: 1 };
  const k = u / ACT_MS;
  if ((x + y) % 2 === 0) return { dy: bob, sx: Math.abs(Math.cos(k * Math.PI * 2)) };
  return { dy: bob - Math.sin(k * Math.PI) * HOP, sx: 1 };
}
