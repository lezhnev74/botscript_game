import { drawBot } from "../../render/drawbot";
import { PRIMARY, SECONDARY, TONES, button, el, section, INPUT, type Tone } from "./dom-kit";
import type { LookValue } from "./look-value";
import { tileGeometry } from "./look-geometry";
import { PreviewCycler } from "./preview-cycler";
import { LookController, PREVIEW_STEPS, previewFace, tileLook, tileValues, type LookModel, type LookSave, type Part, type ProfileApi } from "./look-controller";

const PARTS: [Part, string][] = [["body", "Body"], ["eyes", "Eyes"], ["mouth", "Mouth"], ["hat", "Hat"]];
const TILE = 44;
const PREVIEW = 150;
const HOVER_MS = 700;
const ERR = "text-sm text-rose-300";

function saveText(s: LookSave): { text: string; tone: Tone } {
  switch (s.kind) {
    case "saving": return { text: "Saving...", tone: "info" };
    case "saved": return { text: "Saved - the TV shows your new look from the next update", tone: "ok" };
    case "wait": return { text: `Too many changes: try again in ${s.beats} beat(s).`, tone: "error" };
    case "failed": return { text: `Save failed: ${s.message}`, tone: "error" };
    default: return { text: "", tone: "info" };
  }
}

function canvas(px: number): { el: HTMLCanvasElement; ctx: CanvasRenderingContext2D | null } {
  const c = el("canvas");
  const dpr = globalThis.devicePixelRatio || 1;
  c.width = px * dpr;
  c.height = px * dpr;
  c.style.width = `${px}px`;
  c.style.height = `${px}px`;
  const ctx = c.getContext("2d");
  ctx?.scale(dpr, dpr);
  return { el: c, ctx };
}

function paint(c: ReturnType<typeof canvas>, look: LookValue, step: number, px: number, zoomOn?: Part): void {
  if (!c.ctx) return;
  c.ctx.clearRect(0, 0, px, px);
  const g = tileGeometry(px, zoomOn);
  drawBot(c.ctx, look, previewFace(look, step), g.cx, g.cy, g.size);
}

export interface LookCard {
  element: HTMLElement;
  controller: LookController;
  /** Stops the hover cycle (call on unmount). */
  dispose(): void;
}

/** The "3. Look and name" card: name, color, tile rows, live preview, Randomize and Save look. */
export function createLookCard(api: ProfileApi, hooks: { onSaved(name: string, look: LookValue): void; onUnauthorized(): void }): LookCard {
  const card = section("3. Look and name", "Pick a name, a color and a face. Hover or tap the preview to see how it reacts.");
  const name = el("input", "", "look-name", INPUT + " w-full sm:w-72");
  name.maxLength = 24;
  name.addEventListener("input", () => ctl.setName(name.value));
  const nameErr = el("p", "", "err-name", ERR);
  const color = el("input", "", "look-color", "h-9 w-14 cursor-pointer rounded border border-slate-600 bg-slate-950");
  color.type = "color";
  color.addEventListener("input", () => ctl.setColor(color.value));
  const colorErr = el("p", "", "err-color", ERR);

  const preview = canvas(PREVIEW);
  preview.el.id = "look-preview";
  preview.el.className = "cursor-pointer rounded-lg bg-slate-950";
  let current: LookModel | null = null;
  let shown = 0;
  const draw = (): void => { if (current) paint(preview, current.look, shown, PREVIEW); };
  const cyc = new PreviewCycler({ steps: PREVIEW_STEPS, ms: HOVER_MS, onStep: (s) => { shown = s; draw(); } });
  preview.el.addEventListener("click", () => cyc.tap());
  preview.el.addEventListener("pointerenter", (e) => { if (e.pointerType === "mouse") cyc.hover(); });
  preview.el.addEventListener("pointerleave", () => cyc.leave());

  const random = button("Randomize", "randomize", SECONDARY, () => ctl.randomize());
  const side = el("div", "", "", "flex flex-col items-start gap-3");
  side.append(preview.el, random);

  const rows = PARTS.map(([part, label]) => {
    const row = el("div", "", `row-${part}`, "flex flex-wrap items-center gap-1.5");
    const tiles = tileValues(part).map((value) => {
      const t = button("", `tile-${part}-${value}`, "rounded-md border border-slate-700 bg-slate-950 p-0.5 hover:border-indigo-400", () => ctl.setPart(part, value));
      t.title = value;
      t.setAttribute("aria-label", `${label}: ${value}`);
      const c = canvas(TILE);
      t.append(c.el);
      row.append(t);
      return { value, t, c };
    });
    const err = el("p", "", `err-${part}`, ERR);
    const wrap = el("div", "", "", "space-y-1");
    wrap.append(el("div", label, "", "text-sm font-medium text-slate-300"), row, err);
    return { part, tiles, err, wrap };
  });

  const controls = el("div", "", "", "min-w-0 flex-1 space-y-4");
  const nameRow = el("div", "", "", "space-y-1");
  nameRow.append(el("div", "Name", "", "text-sm font-medium text-slate-300"), name, nameErr);
  const colorRow = el("div", "", "", "space-y-1");
  colorRow.append(el("div", "Color", "", "text-sm font-medium text-slate-300"), color, colorErr);
  controls.append(nameRow, colorRow, ...rows.map((r) => r.wrap));
  const body = el("div", "", "", "flex flex-wrap gap-6");
  body.append(side, controls);

  const saveBtn = button("Save look", "save-look", PRIMARY, () => void ctl.save());
  const badge = el("span", "", "look-badge", "rounded-full px-2.5 py-0.5 text-xs font-medium");
  const saveRow = el("div", "", "", "flex items-center gap-3");
  saveRow.append(saveBtn, badge);
  const info = el("p", "", "look-info");
  card.append(body, saveRow, info);

  const render = (m: LookModel): void => {
    current = m;
    if (name.value !== m.name) name.value = m.name;
    if (color.value !== m.look.color.toLowerCase()) color.value = m.look.color;
    draw();
    rows.forEach((r) => r.tiles.forEach((t) => {
      paint(t.c, tileLook(m.look, r.part, t.value), 0, TILE, r.part);
      const on = m.look[r.part] === t.value;
      t.t.setAttribute("aria-pressed", String(on));
      t.t.classList.toggle("ring-2", on);
      t.t.classList.toggle("ring-indigo-400", on);
    }));
    const fe = (f: string): string => (m.field?.field === f ? m.field.message : "");
    nameErr.textContent = fe("name");
    colorErr.textContent = fe("color");
    rows.forEach((r) => { r.err.textContent = fe(r.part); });
    badge.textContent = m.dirty ? "Unsaved changes" : "Saved";
    badge.className = "rounded-full px-2.5 py-0.5 text-xs font-medium " + (m.dirty ? "bg-amber-900 text-amber-300" : "bg-emerald-900 text-emerald-300");
    saveBtn.disabled = !m.canSave;
    const s = saveText(m.save);
    info.textContent = s.text;
    info.className = TONES[s.tone];
    info.hidden = s.text === "";
  };

  const ctl = new LookController({ api, onChange: render, onSaved: hooks.onSaved, onUnauthorized: hooks.onUnauthorized });
  return { element: card, controller: ctl, dispose: () => cyc.dispose() };
}
