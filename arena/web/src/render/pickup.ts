/** Sideways shake of a bot that took a coin, in cells. */
export const SHAKE_AMPLITUDE = 0.06;
const SHAKE_CYCLES = 3;
const GLOW_PEAK = 0.2;
const GLOW_MAX = 0.8;

/** Horizontal jitter (cells) at effect 0..1: a few quick swings decaying to 0. */
export function shakeOffset(effect: number): number {
  const e = Math.min(1, Math.max(0, effect));
  return SHAKE_AMPLITUDE * (1 - e) * Math.sin(e * SHAKE_CYCLES * Math.PI * 2);
}

/** Golden glow opacity at effect 0..1: rises to a peak early, fades to 0 at the end. */
export function glowAlpha(effect: number): number {
  const e = Math.min(1, Math.max(0, effect));
  const k = e < GLOW_PEAK ? e / GLOW_PEAK : (1 - e) / (1 - GLOW_PEAK);
  return GLOW_MAX * k;
}
