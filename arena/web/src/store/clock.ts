/**
 * The game timer as "mm:ss" (minutes grow past 59, e.g. "90:00"). Beats left are
 * turned into time with the beat length and rounded up, so the clock reads
 * 00:00 only when the game is over. Null when no timer runs (remaining < 0) or
 * the beat length is unknown.
 */
export function formatRemaining(remaining: number, beatMs: number): string | null {
  if (remaining < 0 || beatMs <= 0) return null;
  const seconds = Math.ceil((remaining * beatMs) / 1000);
  const mm = Math.floor(seconds / 60);
  const ss = seconds % 60;
  return `${String(mm).padStart(2, "0")}:${String(ss).padStart(2, "0")}`;
}
