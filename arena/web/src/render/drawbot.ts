import { BOT_R, bodyPath, eyesDraw, hatDraw, inkFor, mouthDraw, outlineFor, type Cmd, type Draw } from "./shapes";

/** Hats are the body color with a light edge: they sit on the dark floor, not on the body. */
export const HAT_EDGE = "#f5f6fa";
/** The floor a bot stands on (outline contrast reference). */
export const FLOOR = "#1c2230";

/** What drawBot needs of a look. */
export interface BotLook { color: string; body: string; hat: string; eyes: string; mouth: string }
/** The eyes and mouth to show (see face.ts). */
export interface BotFace { eyes: string; mouth: string }

/** Starts a new canvas path from unit-space commands, scaled by r around (cx, cy) in pixels. */
export function traceCmds(ctx: CanvasRenderingContext2D, cmds: Cmd[], cx: number, cy: number, r: number): void {
  const x = (u: number) => cx + u * r;
  const y = (u: number) => cy + u * r;
  ctx.beginPath();
  cmds.forEach((k) => {
    if (k[0] === "M") ctx.moveTo(x(k[1]), y(k[2]));
    else if (k[0] === "L") ctx.lineTo(x(k[1]), y(k[2]));
    else if (k[0] === "Q") ctx.quadraticCurveTo(x(k[1]), y(k[2]), x(k[3]), y(k[4]));
    else if (k[0] === "A") ctx.arc(x(k[1]), y(k[2]), k[3] * r, k[4], k[5]);
    else ctx.closePath();
  });
}

/** One ink stroke or fill; a hat (given `edge`) gets a thin light outline so it shows against the floor. */
export function paintInk(ctx: CanvasRenderingContext2D, d: Draw, cx: number, cy: number, r: number, ink: string, edge?: string): void {
  traceCmds(ctx, d.cmds, cx, cy, r);
  if (d.kind === "fill") {
    ctx.globalAlpha = 1;
    ctx.fillStyle = ink;
    ctx.fill();
    if (edge) { ctx.strokeStyle = edge; ctx.lineWidth = 1; ctx.stroke(); }
    return;
  }
  ctx.strokeStyle = ink;
  ctx.lineWidth = Math.max(1.2, r * (d.w ?? 0.12));
  ctx.lineCap = "round";
  ctx.lineJoin = "round";
  ctx.stroke();
}

/** Draws a whole bot (solid body, outline, face, hat) centred at (x, y) px; `size` is the body diameter in px. */
export function drawBot(ctx: CanvasRenderingContext2D, look: BotLook, face: BotFace, x: number, y: number, size: number): void {
  const r = size / 2;
  const path = bodyPath(look.body);
  traceCmds(ctx, path, x, y, r);
  ctx.globalAlpha = 1;
  ctx.fillStyle = look.color;
  ctx.fill();
  ctx.strokeStyle = outlineFor(look.color, FLOOR);
  ctx.lineWidth = Math.max(1.5, size * 0.04 / (2 * BOT_R));
  ctx.stroke();
  const ink = inkFor(look.color);
  [...eyesDraw(face.eyes, look.body), ...mouthDraw(face.mouth, look.body)].forEach((d) => paintInk(ctx, d, x, y, r, ink));
  hatDraw(look.hat, look.body).forEach((d) => paintInk(ctx, d, x, y, r, d.kind === "fill" ? look.color : HAT_EDGE, HAT_EDGE));
}
