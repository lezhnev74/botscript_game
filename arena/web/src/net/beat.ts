export const DEFAULT_BEAT_MS = 1000;
export const MIN_BEAT_MS = 50;

/** Reads the ?beat= playback interval: junk or non-positive -> default, tiny -> the minimum. */
export function parseBeatMs(raw: string | null): number {
  const n = Number(raw);
  if (!raw || !Number.isFinite(n) || n <= 0) return DEFAULT_BEAT_MS;
  return Math.max(n, MIN_BEAT_MS);
}
