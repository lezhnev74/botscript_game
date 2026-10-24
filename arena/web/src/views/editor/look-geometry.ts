import type { Part } from "./look-controller";

/** Face feature rows sit at these unit heights; a face tile zooms in on them. */
const FACE_Y: Partial<Record<Part, number>> = { eyes: -0.15, mouth: 0.42 };
export const FACE_ZOOM = 1.35;

/** Where and how large to draw the bot in a square canvas of `px`; eyes/mouth zoom on their feature. */
export function tileGeometry(px: number, zoomOn?: Part): { cx: number; cy: number; size: number } {
  const fy = zoomOn ? FACE_Y[zoomOn] : undefined;
  if (fy === undefined) return { cx: px / 2, cy: px * 0.56, size: px * 0.6 };
  const size = px * FACE_ZOOM;
  return { cx: px / 2, cy: px / 2 - fy * (size / 2), size };
}
