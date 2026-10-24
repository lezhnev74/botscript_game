import { TV_COLUMNS as COLUMNS, isIconColumn } from "../../render/columns";
import { headerIcon } from "../../render/icons";
import { textColorFor } from "../../render/shapes";
import type { SidebarRow } from "./sidebar";

function cell(tr: HTMLTableRowElement, text: string, color?: string): void {
  const td = tr.insertCell();
  td.textContent = text;
  td.style.cssText = "padding:2px 6px;white-space:nowrap;overflow:hidden;text-overflow:ellipsis";
  if (color) td.style.color = color;
}

/** HP as a small red bar: fill = hp / maxHp. */
function hpCell(tr: HTMLTableRowElement, hp: number, maxHp: number): void {
  const td = tr.insertCell();
  td.style.cssText = "padding:2px 6px";
  td.title = `${hp}/${maxHp}`;
  const well = document.createElement("div");
  well.style.cssText = "height:8px;background:#2a0606;border:1px solid #000";
  const fill = document.createElement("div");
  const pct = maxHp > 0 ? Math.round((100 * Math.min(hp, maxHp)) / maxHp) : 0;
  fill.style.cssText = `height:100%;width:${pct}%;background:#c81818;box-shadow:inset 0 2px 0 #ff5a5a`;
  well.append(fill);
  td.append(well);
}

/** Replaces the table contents with the rows (thin DOM glue; rows are built by sidebarRows). */
export function renderSidebar(table: HTMLTableElement, rows: SidebarRow[]): void {
  table.replaceChildren();
  table.style.width = `calc(${COLUMNS.reduce((n, [, ch]) => n + ch, 0)}ch + ${COLUMNS.length * 12}px)`;
  const head = table.createTHead().insertRow();
  COLUMNS.forEach(([c, ch]) => {
    cell(head, isIconColumn(c) ? "" : c);
    if (isIconColumn(c)) head.cells[head.cells.length - 1]!.append(headerIcon(c));
    head.cells[head.cells.length - 1]!.style.width = `${ch}ch`;
  });
  const body = table.createTBody();
  for (const r of rows) {
    const tr = body.insertRow();
    cell(tr, (r.mine ? "* " : "") + r.name + (r.rebooting ? " (rebooting)" : ""), textColorFor(r.color));
    hpCell(tr, r.hp, r.maxHp);
    cell(tr, String(r.kills));
    cell(tr, String(r.deaths));
    cell(tr, String(r.coins));
    cell(tr, String(r.score));
  }
}
