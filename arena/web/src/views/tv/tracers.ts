import type { Pt } from "../../render/scene";
import type { Grid } from "../../store/reducer";

export const BULLET_SPEED = 3;

const VEC: Record<string, Pt> = { N: { x: 0, y: -1 }, E: { x: 1, y: 0 }, S: { x: 0, y: 1 }, W: { x: -1, y: 0 } };
const OPPOSITE: Record<string, string> = { N: "S", S: "N", E: "W", W: "E" };

export const vec = (dir: string): Pt => VEC[dir] ?? { x: 0, y: 0 };

export const opposite = (dir: string): string => OPPOSITE[dir] ?? dir;

export function isFloor(g: Grid, p: Pt): boolean {
  return g.rows[p.y]?.[p.x] === ".";
}

/** Walks from `end` against `dir` up to `n` cells, stopping before a wall. */
export function tracerBack(end: Pt, dir: string, n: number, g: Grid): Pt {
  const v = vec(dir);
  let p = end;
  for (let i = 0; i < n; i++) {
    const prev = { x: p.x - v.x, y: p.y - v.y };
    if (!isFloor(g, prev)) break;
    p = prev;
  }
  return p;
}

/** True when `p` lies on the segment end -> back `n` cells against `dir`. */
export function onTrace(p: Pt, end: Pt, dir: string, n: number): boolean {
  const v = vec(dir);
  for (let i = 0; i <= n; i++) {
    if (p.x === end.x - v.x * i && p.y === end.y - v.y * i) return true;
  }
  return false;
}

/**
 * Flies a bullet from `from` along `dir` for up to n cells. Returns where it meets a wall (the wall edge,
 * half a cell past the last floor cell), or null when it stays in the air or reaches a cell in `occupied`.
 */
export function wallStop(from: Pt, dir: string, n: number, g: Grid, occupied: (p: Pt) => boolean): Pt | null {
  const v = vec(dir);
  let p = from;
  if (occupied(p)) return null;
  for (let i = 0; i < n; i++) {
    const next = { x: p.x + v.x, y: p.y + v.y };
    if (!isFloor(g, next)) return { x: p.x + v.x / 2, y: p.y + v.y / 2 };
    if (occupied(next)) return null;
    p = next;
  }
  return null;
}
