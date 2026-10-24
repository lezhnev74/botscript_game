import { LEADERBOARD_COLUMNS as COLUMNS } from "../../render/columns";
import type { Overlay } from "./overlay";

/** The TV overlay: a clock in the corner of the stage and a "Game over" panel with the final board. */
export interface OverlayView {
  render(o: Overlay): void;
}

function rowCells(tr: HTMLTableRowElement, texts: string[], css: string): void {
  texts.forEach((t) => {
    const td = tr.insertCell();
    td.textContent = t;
    td.style.cssText = css;
  });
}

function finalTable(o: Overlay): HTMLTableElement {
  const table = document.createElement("table");
  table.id = "final-board";
  table.style.cssText = "font:18px monospace;border-collapse:collapse;margin:0 auto";
  rowCells(table.createTHead().insertRow(), [...COLUMNS], "padding:4px 12px;color:#999");
  const body = table.createTBody();
  for (const r of o.rows) {
    const tr = body.insertRow();
    rowCells(tr, [String(r.rank), (r.mine ? "* " : "") + r.name, String(r.score), String(r.kills), String(r.deaths), String(r.coins)], "padding:4px 12px");
    if (r.mine) tr.style.cssText = "background:#2a3b1f;color:#bfef45";
  }
  return table;
}

export function mountOverlay(stage: HTMLElement): OverlayView {
  const clock = document.createElement("div");
  clock.id = "game-clock";
  clock.style.cssText = "position:absolute;right:8px;top:8px;font:bold 28px monospace;color:#fff;background:rgba(0,0,0,.55);padding:2px 10px;display:none";
  const over = document.createElement("div");
  over.id = "game-over";
  over.style.cssText =
    "position:absolute;inset:0;display:none;flex-direction:column;align-items:center;justify-content:center;gap:16px;background:rgba(0,0,0,.78);color:#ddd;overflow:auto";
  stage.append(clock, over);
  return {
    render(o) {
      clock.style.display = o.clock === null ? "none" : "block";
      clock.textContent = o.clock ?? "";
      over.style.display = o.gameOver ? "flex" : "none";
      const title = document.createElement("h1");
      title.textContent = "Game over";
      title.style.cssText = "font:bold 48px monospace;margin:0;color:#fff";
      over.replaceChildren(...(o.gameOver ? [title, finalTable(o)] : []));
    },
  };
}
