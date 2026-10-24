import type { ViewModel } from "../../store/reducer";
import { LEADERBOARD_COLUMNS as COLUMNS, isIconColumn } from "../../render/columns";
import { headerIcon } from "../../render/icons";
import { buildRows, type BoardRow } from "../../store/standings";
import { lastGameLoader, type BoardEntry } from "./last-game";

export interface LeaderboardStore {
  state: ViewModel;
  subscribe(fn: (s: ViewModel) => void): () => void;
}

function cell(tr: HTMLTableRowElement, text: string): void {
  const td = tr.insertCell();
  td.textContent = text;
  td.style.padding = "4px 12px";
}

function drawRows(table: HTMLTableElement, rows: BoardRow[]): void {
  table.replaceChildren();
  const head = table.createTHead().insertRow();
  COLUMNS.forEach((c) => {
    cell(head, isIconColumn(c) ? "" : c);
    if (isIconColumn(c)) head.cells[head.cells.length - 1]!.append(headerIcon(c));
  });
  const body = table.createTBody();
  for (const r of rows) {
    const tr = body.insertRow();
    if (r.mine) tr.style.cssText = "background:#2a3b1f;color:#bfef45";
    [String(r.rank), (r.mine ? "* " : "") + r.name, String(r.score), String(r.kills), String(r.deaths), String(r.coins)].forEach((t) =>
      cell(tr, t),
    );
  }
}

const TABLE_CSS = "font:16px monospace;border-collapse:collapse";

/** Mounts the leaderboard table, and under it the latest finished game's table, into root; returns an unmount function. */
export function mountLeaderboard(root: HTMLElement, store: LeaderboardStore, loadLast: () => Promise<BoardEntry[]>): () => void {
  root.style.cssText = "padding:16px";
  const title = document.createElement("h2");
  title.textContent = "Leaderboard";
  const table = document.createElement("table");
  table.id = "leaderboard";
  table.style.cssText = TABLE_CSS;
  const lastTitle = document.createElement("h2");
  lastTitle.textContent = "Last game";
  const last = document.createElement("table");
  last.id = "last-game";
  last.style.cssText = TABLE_CSS;
  lastTitle.hidden = true;
  root.replaceChildren(title, table, lastTitle, last);
  const feed = lastGameLoader(loadLast, (entries) => {
    lastTitle.hidden = entries.length === 0;
    drawRows(last, buildRows({ board: entries, myBotId: store.state.myBotId }));
  });
  const draw = (vm: ViewModel): void => {
    drawRows(table, buildRows(vm));
    feed(vm);
  };
  const off = store.subscribe(draw);
  draw(store.state);
  return off;
}
