import { coinPose } from "./coin";
import { FLOOR, HAT_EDGE, paintInk, traceCmds } from "./drawbot";
import { fitCells } from "./fit";
import { glowAlpha, shakeOffset } from "./pickup";
import { BAR_H, BAR_TOP, BOT_DY, BOT_R, bodyPath, eyesDraw, hatDraw, inkFor, LABEL_Y, mouthDraw, outlineFor, shownColor, textColorFor, type Cmd, type Draw } from "./shapes";
import { rainGlyphs, seedOf } from "./rain";
import { rafFrames, type Frames } from "./tween";
import type { BotSprite, Figure, FallenSprite, Pt, RadarMark, RebootSprite, Renderer, Scene, TakenCoin, TracerLine } from "./scene";

const BG = "#0a0a0a";
const GRID = "#2c3446";
const WALL = "#555a66";
const COIN = "#ffd700";
const GLOW = "255, 215, 0";
const BULLET = "#fff";
const HIT = "#ff2a2a";
const SPARK = "#ffb000";
/** Health bar: empty well, red fill, its lighter top highlight, segment ticks. */
const BAR_BG = "#2a0606";
const BAR_FILL = "#c81818";
const BAR_SHINE = "#ff5a5a";
const BAR_EDGE = "#000";
/** Bot disc radius and centre offset inside its cell. */
/** Hit blinks (on+off pairs) during the effect phase; death spark count. */
const BLINKS = 3;
/** True during the red-on half of a blink cycle while an effect runs. */
const blinkOn = (e: number): boolean => e > 0 && e < 1 && Math.floor(e * BLINKS * 2) % 2 === 0;
const SPARKS = 8;
/** Radar half circle radius and the sweep line's fading trail, in cells and radians. */
const RADAR_R = 0.9;
const RADAR_TRAIL = 1.2;
const RADAR_SLICES = 8;

/** Reboot effect: rain glyphs (head, trail), and the progress bar well and fill. */
const RAIN_HEAD = "#d8ffe0";
const RAIN_TRAIL = "#00ff66";
const REBOOT_WELL = "#06210f";
/** Progress bar geometry inside the cell, in cells. */
const REBOOT_BAR = { x: 0.1, y: 0.88, w: 0.8, h: 0.07 };

/** Coin sprite edge in cells. */
const COIN_SIZE = 0.6;

let coinImg: HTMLImageElement | null = null;

/** Loads /coin.png once; yields it only when ready (null before, or without a browser). */
function coinSprite(): CanvasImageSource | null {
  if (typeof Image === "undefined") return null;
  if (!coinImg) {
    coinImg = new Image();
    coinImg.src = "/coin.png";
  }
  return coinImg.complete && coinImg.naturalWidth > 0 ? coinImg : null;
}

/** Thin canvas2d painter: draws Scene primitives, decides nothing. */
export class Canvas2dRenderer implements Renderer {
  private readonly ctx: CanvasRenderingContext2D;
  private cell = 1;
  private ox = 0;
  private oy = 0;
  private cssW = 0;
  private cssH = 0;
  private last: Scene | null = null;
  private frame: number | null = null;
  /** Set by render(); the self-repaint skips a frame when someone else painted since the last one. */
  private rendered = false;

  /** `sprite`: coin image getter (null until loaded; default loads /coin.png once); `frames`: clock for the coin animation (null = static). */
  constructor(
    private readonly canvas: HTMLCanvasElement,
    private readonly dpr: number = globalThis.devicePixelRatio || 1,
    private readonly frames: Frames | null = typeof requestAnimationFrame === "function" ? rafFrames : null,
    private readonly sprite: () => CanvasImageSource | null = coinSprite,
  ) {
    this.ctx = canvas.getContext("2d") as CanvasRenderingContext2D;
  }

  resize(widthPx: number, heightPx: number): void {
    this.cssW = widthPx;
    this.cssH = heightPx;
    this.canvas.width = Math.round(widthPx * this.dpr);
    this.canvas.height = Math.round(heightPx * this.dpr);
    if (this.canvas.style) {
      this.canvas.style.width = `${widthPx}px`;
      this.canvas.style.height = `${heightPx}px`;
    }
    if (this.last) this.render(this.last);
  }

  destroy(): void {
    this.last = null;
    this.stop();
  }

  private stop(): void {
    if (this.frame !== null) this.frames?.cancel(this.frame);
    this.frame = null;
  }

  /** Coins and the reboot rain move on their own clock: keep repainting the last scene while it has either. */
  private animate(): void {
    this.stop();
    if (!this.frames || !this.last || (this.last.coins.length === 0 && this.last.rebooting.length === 0)) return;
    this.frame = this.frames.request(() => {
      this.frame = null;
      const driven = this.rendered;
      this.rendered = false;
      if (this.last && !driven) this.paint(this.last);
      this.animate();
    });
  }

  render(scene: Scene): void {
    this.last = scene;
    this.rendered = true;
    this.paint(scene);
    this.animate();
  }

  private paint(scene: Scene): void {
    const { ctx } = this;
    ctx.setTransform(this.dpr, 0, 0, this.dpr, 0, 0);
    ctx.fillStyle = BG;
    ctx.fillRect(0, 0, this.cssW, this.cssH);
    if (scene.width === 0) return;
    const fit = fitCells(this.cssW, this.cssH, scene.width, scene.height);
    this.cell = fit.cell;
    this.ox = fit.offsetX;
    this.oy = fit.offsetY;
    ctx.translate(this.ox, this.oy);
    ctx.fillStyle = FLOOR;
    ctx.fillRect(0, 0, this.px(scene.width), this.px(scene.height));
    this.drawGrid(scene);
    scene.walls.forEach((w) => this.fillCell(w, WALL));
    const now = this.frames?.now() ?? 0;
    scene.coins.forEach((c) => this.coin(c, now));
    scene.taken.forEach((c) => this.taken(c, scene.effect, now));
    scene.rebooting.forEach((r) => this.reboot(r, now)); // under bots, tracers and radars
    scene.radars.forEach((r) => this.radar(r));
    scene.tracers.forEach((t) => this.bullet(t));
    scene.fallen.forEach((f) => this.fallen(f, f.effect ?? scene.effect));
    scene.bots.forEach((b) => this.bot(b, scene.effect, scene.taken.some((t) => t.bot === b.id)));
  }

  /** A rebooting bot: its cell tinted in the bot's color, matrix rain falling in it and a progress bar under the rain. */
  private reboot(r: RebootSprite, now: number): void {
    this.fillCell(r, r.color, 0.18);
    const { glyphs, count } = rainGlyphs(seedOf(r.id), now);
    const size = Math.max(6, this.cell * 0.2);
    for (let i = 0; i < count; i++) {
      const g = glyphs[i]!;
      this.ctx.globalAlpha = g.alpha;
      this.text(g.ch, r.x + g.x, r.y + g.y + 0.06, g.alpha === 1 ? RAIN_HEAD : RAIN_TRAIL, size);
    }
    this.ctx.globalAlpha = 1;
    const { x, y, w, h } = REBOOT_BAR;
    this.ctx.fillStyle = REBOOT_WELL;
    this.ctx.fillRect(this.px(r.x + x), this.px(r.y + y), this.px(w), this.px(h));
    this.ctx.fillStyle = RAIN_TRAIL;
    this.ctx.fillRect(this.px(r.x + x), this.px(r.y + y), this.px(w * r.progress), this.px(h));
    this.ctx.strokeStyle = r.color;
    this.ctx.lineWidth = 1;
    this.ctx.strokeRect(this.px(r.x + x), this.px(r.y + y), this.px(w), this.px(h));
  }

  /** A coin taken this beat: whole under the gliding taker, then shrinking and fading away during the effect phase. */
  private taken(c: TakenCoin, effect: number, now: number): void {
    if (effect >= 1) return;
    this.ctx.globalAlpha = 1 - effect;
    this.coin(c, now, 1 - effect);
    this.ctx.globalAlpha = 1;
  }

  /** Soft golden radial glow around a bot's body. */
  private glow(c: Pt, alpha: number): void {
    if (alpha <= 0) return;
    const r = this.px(0.8);
    const g = this.ctx.createRadialGradient(this.px(c.x), this.px(c.y), 0, this.px(c.x), this.px(c.y), r);
    g.addColorStop(0, `rgba(${GLOW}, ${alpha})`);
    g.addColorStop(1, `rgba(${GLOW}, 0)`);
    this.ctx.fillStyle = g;
    this.ctx.beginPath();
    this.ctx.arc(this.px(c.x), this.px(c.y), r, 0, Math.PI * 2);
    this.ctx.fill();
  }

  /** A coin: the sprite (bob, spin or hop from coinPose) once loaded, a plain gold disc until then. */
  private coin(c: Pt, now: number, scale = 1): void {
    const pose = coinPose(c.x, c.y, now, this.cell);
    const cx = c.x + 0.5;
    const cy = c.y + 0.5 + pose.dy;
    const img = this.sprite();
    if (!img) return this.disc({ x: cx, y: cy }, 0.25 * scale, COIN);
    const size = this.px(COIN_SIZE * scale);
    this.ctx.drawImage(img, this.px(cx) - (size * pose.sx) / 2, this.px(cy) - size / 2, size * pose.sx, size);
  }

  private px(v: number): number {
    return v * this.cell;
  }

  private drawGrid(s: Scene): void {
    for (let x = 0; x <= s.width; x++) this.line({ x, y: 0 }, { x, y: s.height }, GRID, 1);
    for (let y = 0; y <= s.height; y++) this.line({ x: 0, y }, { x: s.width, y }, GRID, 1);
  }

  private fillCell(p: Pt, color: string, alpha = 1): void {
    this.ctx.globalAlpha = alpha;
    this.ctx.fillStyle = color;
    this.ctx.fillRect(this.px(p.x), this.px(p.y), this.cell, this.cell);
    this.ctx.globalAlpha = 1;
  }

  private strokeCell(p: Pt, color: string, width = 1): void {
    this.ctx.strokeStyle = color;
    this.ctx.lineWidth = width;
    this.ctx.strokeRect(this.px(p.x) + 1, this.px(p.y) + 1, this.cell - 2, this.cell - 2);
  }

  /** Filled circle; `c` is the absolute centre in cell units. */
  private disc(c: Pt, r: number, color: string): void {
    this.ctx.fillStyle = color;
    this.ctx.beginPath();
    this.ctx.arc(this.px(c.x), this.px(c.y), this.px(r), 0, Math.PI * 2);
    this.ctx.fill();
  }

  private ring(c: Pt, r: number, color: string): void {
    this.ctx.strokeStyle = color;
    this.ctx.lineWidth = 2;
    this.ctx.beginPath();
    this.ctx.arc(this.px(c.x), this.px(c.y), this.px(r), 0, Math.PI * 2);
    this.ctx.stroke();
  }

  /** Line between cells; `centered` aims at cell centres instead of corners. */
  private line(a: Pt, b: Pt, color: string, width: number, centered = false): void {
    const o = centered ? 0.5 : 0;
    this.ctx.strokeStyle = color;
    this.ctx.lineWidth = width;
    this.ctx.beginPath();
    this.ctx.moveTo(this.px(a.x + o), this.px(a.y + o));
    this.ctx.lineTo(this.px(b.x + o), this.px(b.y + o));
    this.ctx.stroke();
  }

  /** Text centred at (x, y) cells; size in css px. */
  private text(s: string, x: number, y: number, color: string, sizePx: number): void {
    this.ctx.fillStyle = color;
    this.ctx.font = `${sizePx}px monospace`;
    this.ctx.textAlign = "center";
    this.ctx.fillText(s, this.px(x), this.px(y));
  }

  /** Filled wedge around centre c (cells) between angles a0..a1 (radians). */
  private wedge(c: Pt, r: number, a0: number, a1: number, color: string, alpha: number): void {
    const x = this.px(c.x);
    const y = this.px(c.y);
    this.ctx.globalAlpha = alpha;
    this.ctx.fillStyle = color;
    this.ctx.beginPath();
    this.ctx.moveTo(x, y);
    this.ctx.arc(x, y, this.px(r), a0, a1);
    this.ctx.closePath();
    this.ctx.fill();
    this.ctx.globalAlpha = 1;
  }

  /** Scan: a faint half circle facing the scan direction, its rim, and a radar line sweeping across with a fading trail. */
  private radar(r: RadarMark): void {
    const c = { x: r.x + 0.5, y: r.y + BOT_DY };
    const start = r.angle - Math.PI / 2;
    this.wedge(c, RADAR_R, start, start + Math.PI, r.color, 0.3);
    this.ctx.strokeStyle = r.color;
    this.ctx.lineWidth = 1.5;
    this.ctx.beginPath();
    this.ctx.arc(this.px(c.x), this.px(c.y), this.px(RADAR_R), start, start + Math.PI);
    this.ctx.stroke();
    if (r.sweep >= 1) return;
    const head = start + Math.PI * r.sweep;
    const step = Math.min(RADAR_TRAIL, head - start) / RADAR_SLICES;
    for (let i = 0; i < RADAR_SLICES; i++) {
      this.wedge(c, RADAR_R, head - step * (i + 1), head - step * i, r.color, 0.45 * (1 - i / RADAR_SLICES));
    }
    const tip = { x: c.x + Math.cos(head) * RADAR_R, y: c.y + Math.sin(head) * RADAR_R };
    this.line(c, tip, r.color, 1.5);
  }

  /** Bullet: a faint trail behind a small ringed slug at the head; drawn under bots, so a hit vanishes into its victim. */
  private bullet(t: TracerLine): void {
    this.ctx.globalAlpha = 0.3;
    this.line(t.from, t.to, t.color, 1, true);
    this.ctx.globalAlpha = 1;
    const head = { x: t.to.x + 0.5, y: t.to.y + 0.5 };
    this.disc(head, 0.13, t.color);
    this.disc(head, 0.07, BULLET);
  }

  /** A name above a bot, drawn after the bot with a dark halo so a hat or a similar floor never hides it. */
  private label(s: string, x: number, y: number, color: string): void {
    const size = this.labelSize();
    this.ctx.font = `${size}px monospace`;
    this.ctx.textAlign = "center";
    this.ctx.lineJoin = "round";
    this.ctx.lineWidth = Math.max(2, size * 0.28);
    this.ctx.strokeStyle = BG;
    this.ctx.strokeText(s, this.px(x), this.px(y));
    this.text(s, x, y, textColorFor(color, FLOOR), size);
  }

  private labelSize(): number {
    return Math.min(14, Math.max(9, Math.round(this.cell * 0.22)));
  }

  /** A bot: its body (a gun gauge), name above it, shield/mine/pencil marks, and a red blink when hit this beat. */
  private bot(b: BotSprite, effect: number, tookCoin: boolean): void {
    const shake = tookCoin ? shakeOffset(effect) : 0;
    const c = { x: b.x + 0.5 + shake, y: b.y + BOT_DY };
    if (tookCoin) this.glow(c, glowAlpha(effect));
    this.body(c, b, b.charge);
    const he = b.effect ?? effect;
    if (b.hit && blinkOn(he)) {
      this.trace(bodyPath(b.body), c, BOT_R * 1.1);
      this.fillPath(HIT, 0.75);
      this.ring(c, BOT_R + 0.12, HIT);
    }
    this.face(c, b, b.color, b.charge);
    this.marks(b);
  }

  /** Shield, mine and pencil marks, the name and the health bar of a bot. */
  private marks(b: BotSprite): void {
    if (b.shielded) this.strokeCell(b, "#fff", 2);
    if (b.mine) this.text("*", b.x + 0.14, b.y + 0.62, "#fff", Math.max(10, this.cell * 0.4));
    this.healthBar(b);
    if (b.edited) this.text("✎", b.x + 0.85, b.y + 0.3, "#fff", Math.max(10, this.cell * 0.3));
    this.label(b.name, b.x + 0.5, b.y + LABEL_Y, b.color);
  }

  /** Adds the path commands to the current canvas path, in unit space scaled by r around c (cells). */
  private trace(cmds: Cmd[], c: Pt, r: number): void {
    traceCmds(this.ctx, cmds, this.px(c.x), this.px(c.y), this.px(r));
  }

  /** Fills the traced path. */
  private fillPath(color: string, alpha = 1): void {
    this.ctx.globalAlpha = alpha;
    this.ctx.fillStyle = color;
    this.ctx.fill();
    this.ctx.globalAlpha = 1;
  }

  /** Bot body as a gun gauge: the shape dim, filled bottom-up of the wedge clockwise from 12 o'clock with the charge; full = solid, ready to fire. A darker outline keeps it readable on the floor. */
  private body(c: Pt, f: Figure & { color: string }, charge: number, r = BOT_R): void {
    const ctx = this.ctx;
    const path = bodyPath(f.body);
    this.trace(path, c, r);
    if (charge >= 1) {
      this.fillPath(f.color);
    } else {
      this.fillPath(f.color, 0.25);
      if (charge > 0) {
        ctx.save();
        ctx.beginPath();
        ctx.moveTo(this.px(c.x), this.px(c.y));
        ctx.arc(this.px(c.x), this.px(c.y), this.px(r * 2), -Math.PI / 2, -Math.PI / 2 + Math.PI * 2 * charge);
        ctx.closePath();
        ctx.clip();
        this.trace(path, c, r);
        this.fillPath(f.color);
        ctx.restore();
      }
    }
    this.trace(path, c, r);
    ctx.strokeStyle = charge >= 1 ? outlineFor(f.color, FLOOR) : f.color;
    ctx.lineWidth = Math.max(1.5, this.px(0.04));
    ctx.stroke();
  }

  /** Eyes and mouth in the ink that contrasts with the color the body really shows (a charging one is dim); the hat keeps the body color with a light edge so it reads on any floor. */
  private face(c: Pt, f: Figure, color: string, charge = 1, r = BOT_R): void {
    const ink = inkFor(shownColor(color, charge, FLOOR));
    [...eyesDraw(f.eyes, f.body), ...mouthDraw(f.mouth, f.body)].forEach((d) => this.ink(d, c, r, ink));
    hatDraw(f.hat, f.body).forEach((d) => this.ink(d, c, r, d.kind === "fill" ? color : HAT_EDGE, HAT_EDGE));
  }

  /** One ink stroke or fill; a hat (given `edge`) gets a thin light outline so it shows against the floor. */
  private ink(d: Draw, c: Pt, r: number, ink: string, edge?: string): void {
    paintInk(this.ctx, d, this.px(c.x), this.px(c.y), this.px(r), ink, edge);
  }

  /** Diablo-style health bar under the bot: a dark well, a red fill with a highlight, one tick per hit point. */
  private healthBar(b: BotSprite): void {
    if (b.maxHp <= 0) return;
    const x = this.px(b.x + 0.12);
    const w = this.px(0.76);
    const h = Math.max(3, this.px(BAR_H));
    const y = this.px(b.y + BAR_TOP);
    const fill = (w * Math.max(0, Math.min(b.hp, b.maxHp))) / b.maxHp;
    this.ctx.fillStyle = BAR_EDGE;
    this.ctx.fillRect(x - 1, y - 1, w + 2, h + 2);
    this.ctx.fillStyle = BAR_BG;
    this.ctx.fillRect(x, y, w, h);
    this.ctx.fillStyle = BAR_FILL;
    this.ctx.fillRect(x, y, fill, h);
    this.ctx.fillStyle = BAR_SHINE;
    this.ctx.fillRect(x, y, fill, Math.max(1, h / 3));
    this.ctx.fillStyle = BAR_EDGE;
    for (let i = 1; i < b.maxHp; i++) this.ctx.fillRect(x + (w * i) / b.maxHp - 0.5, y, 1, h);
  }

  /** A dead bot: alive-looking until the effect phase, then a shrinking body in a red ring and sparks; then a faded ghost with x eyes and its respawn label. */
  private fallen(f: FallenSprite, effect: number): void {
    const c = { x: f.x + 0.5, y: f.y + BOT_DY };
    if (!f.dying || effect >= 1) {
      this.ctx.globalAlpha = 0.4;
      this.body(c, f, 1);
      this.face(c, f, f.color);
      this.ctx.globalAlpha = 1;
      this.label(f.label, f.x + 0.5, f.y + LABEL_Y, f.color); // just the number, in the name slot (a fallen bot has no bar)
      return;
    }
    if (effect === 0) {
      if (f.look) this.bot({ ...f.look, x: f.x, y: f.y }, 0, false); // the bullet has not landed: still the live bot
      else this.plain(f, c);
      return;
    }
    const look = f.look ?? { ...f, edited: false, shielded: false, mine: false, hit: false, hp: 0, maxHp: 0, charge: 1 };
    const r = BOT_R * (1 - effect);
    this.body(c, look, look.charge, r);
    this.ctx.globalAlpha = 1 - effect;
    this.face(c, { ...look, eyes: f.eyes, mouth: f.mouth }, look.color, look.charge, r); // the killing hit has landed: x eyes, flat mouth
    this.marks({ ...look, x: f.x, y: f.y, hp: look.hp * (1 - effect) });
    if (blinkOn(effect)) { this.trace(bodyPath(f.body), c, r * 1.1); this.fillPath(HIT); }
    this.ring(c, BOT_R + 0.5 * effect, HIT);
    this.sparks(c, effect);
    this.ctx.globalAlpha = 1;
  }

  /** A dying sprite with no remembered look, before its bullet lands: a plain body, face and name. */
  private plain(f: FallenSprite, c: Pt): void {
    this.body(c, f, 1);
    this.face(c, f, f.color);
    this.label(f.name, f.x + 0.5, f.y + LABEL_Y, f.color);
  }

  private sparks(c: Pt, effect: number): void {
    for (let i = 0; i < SPARKS; i++) {
      const a = (i / SPARKS) * Math.PI * 2;
      const r0 = 0.2 + 0.4 * effect;
      const r1 = r0 + 0.15;
      this.line({ x: c.x + Math.cos(a) * r0, y: c.y + Math.sin(a) * r0 }, { x: c.x + Math.cos(a) * r1, y: c.y + Math.sin(a) * r1 }, SPARK, 2);
    }
  }
}
