// Renderer-agnostic description of one frame. Coordinates are map cells
// (floats allowed); the renderer owns pixels. No logic lives here.

export interface Pt { x: number; y: number }

export interface TracerLine {
  from: Pt;
  to: Pt;
  color: string;
  /** "hit" tracers end in a bot; "flying" ones are bullets still in the air; "wall" ones end at a wall edge. */
  kind: "flying" | "hit" | "wall";
  /** Fraction of the beat (0..1) the bullet needs to reach `to`: 1 for a bullet in flight, distance / speed for a hit or wall. */
  arrive: number;
  /** Hit tracers only: the bot the bullet lands in. */
  victim?: string;
}

/** A scan: a half circle facing the scan direction with a radar line sweeping across it. */
export interface RadarMark {
  x: number;
  y: number;
  /** Facing angle in radians, canvas convention (0 = east, PI/2 = south). */
  angle: number;
  color: string;
  /** 0..1, how far the sweep line has crossed the half circle; 1 = done (no line). */
  sweep: number;
}

/** How a bot is drawn: the body shape and hat of its look, and the face it shows right now (resting or a reaction). */
export interface Figure {
  body: string;
  hat: string;
  eyes: string;
  mouth: string;
}

export interface BotSprite extends Figure {
  id: string;
  x: number;
  y: number;
  color: string;
  name: string;
  /** Pencil badge after a recent hot-swap. */
  edited: boolean;
  shielded: boolean;
  mine: boolean;
  /** Took a hit this beat: blinks red during the effect phase. */
  hit: boolean;
  /** Health bar: hp of maxHp (hp may be fractional while it animates). */
  hp: number;
  maxHp: number;
  /** Gun charge 0..1: the body fills clockwise as the reload runs; 1 = ready to fire. */
  charge: number;
  /** Beat fraction (0..1) at which the bullet that hit this bot lands; its blink and drain start then. Unset: the common effect phase. */
  landAt?: number;
  /** 0..1 progress of this bot's own hit effect; set by the tween from landAt. Unset: use Scene.effect. */
  effect?: number;
}

export interface FallenSprite extends Figure {
  id: string;
  x: number;
  y: number;
  color: string;
  label: string;
  name: string;
  /** Killed this beat: drawn alive until the effect phase, then bursts. */
  dying: boolean;
  /** How the bot looked alive (last drawn by the tween): a dying sprite keeps this look until the bullet lands. */
  look?: BotSprite;
  /** Beat fraction (0..1) at which the killing bullet lands; the burst starts then. Unset: the common effect phase. */
  landAt?: number;
  /** 0..1 progress of this sprite's own burst; set by the tween from landAt. Unset: use Scene.effect. */
  effect?: number;
}

/** A bot off the map for a reboot: matrix rain in the cell it left, a progress bar under it. */
export interface RebootSprite {
  id: string;
  x: number;
  y: number;
  color: string;
  /** 0..1, how much of the reboot has passed: the bar fill. */
  progress: number;
}

/** A coin taken this beat: still drawn at its cell, under the taker, until the effect phase shrinks it away. */
export interface TakenCoin extends Pt {
  bot: string;
}

export interface Scene {
  /** The beat this scene shows; 0 for the empty scene before any frame. */
  beat: number;
  width: number;
  height: number;
  walls: Pt[];
  coins: Pt[];
  taken: TakenCoin[];
  radars: RadarMark[];
  tracers: TracerLine[];
  bots: BotSprite[];
  fallen: FallenSprite[];
  rebooting: RebootSprite[];
  /** 0..1 progress of the hit/death effects that follow the moves; 1 = done (static scene). */
  effect: number;
}

export interface Renderer {
  render(scene: Scene): void;
  resize(widthPx: number, heightPx: number): void;
  destroy(): void;
}
