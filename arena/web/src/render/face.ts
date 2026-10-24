// Auto reactions: the face a bot shows from what just happened to it. Pure; the resting face is the chosen look.

export interface Face { eyes: string; mouth: string }

/** What happened to a bot, in beats: `hitBeat` / `killBeat` are the beats of its latest hit and kill. */
export interface Reactions {
  dead: boolean;
  /** The bot is on its respawn shield: back to the resting face. */
  shielded?: boolean;
  hitBeat?: number;
  killBeat?: number;
}

/** How many beats a wince and a triumph last. */
export const HIT_BEATS = 1;
export const KILL_BEATS = 2;

/** Dead > hit > kill; a respawn shield (or nothing recent) shows the resting face. */
export function faceOf(look: Face, r: Reactions, beat: number): Face {
  if (r.dead) return { eyes: "x", mouth: "flat" };
  if (r.shielded) return { eyes: look.eyes, mouth: look.mouth };
  if (r.hitBeat !== undefined && beat >= r.hitBeat && beat - r.hitBeat < HIT_BEATS) return { eyes: "squint", mouth: "o" };
  if (r.killBeat !== undefined && beat >= r.killBeat && beat - r.killBeat < KILL_BEATS) return { eyes: "wide", mouth: "grin" };
  return { eyes: look.eyes, mouth: look.mouth };
}
