import { AdminController, WALL_LIMITS, type AdminApi, type AdminModel } from "./controller";

const REFRESH_MS = 3000;

type Child = Node | string;

function el<K extends keyof HTMLElementTagNameMap>(tag: K, css: string, ...kids: Child[]): HTMLElementTagNameMap[K] {
  const e = document.createElement(tag);
  e.style.cssText = css;
  e.append(...kids);
  return e;
}

function button(label: string, onClick: () => void): HTMLButtonElement {
  const b = el("button", "margin-left:8px;font:14px monospace", label);
  b.type = "button";
  b.addEventListener("click", onClick);
  return b;
}

function loginForm(c: AdminController, message: string): HTMLElement {
  const input = el("input", "font:16px monospace");
  input.type = "password";
  input.autocomplete = "current-password";
  input.placeholder = "admin password";
  const form = el("form", "margin:12px 0", input, button("Log in", () => void c.login(input.value)));
  form.addEventListener("submit", (ev) => {
    ev.preventDefault();
    void c.login(input.value);
  });
  return el("div", "", form, el("p", "color:#f88", message));
}

function table(c: AdminController, m: AdminModel): HTMLTableElement {
  const t = el("table", "font:14px monospace;border-collapse:collapse;margin:12px 0");
  const head = t.createTHead().insertRow();
  for (const h of ["id", "name", "kind", "score", ""]) head.insertCell().textContent = h;
  const body = t.createTBody();
  for (const r of m.rows) {
    const tr = body.insertRow();
    for (const v of [r.id, r.name, r.kind, String(r.score)]) tr.insertCell().textContent = v;
    if (r.canKick) tr.insertCell().append(button("Kick", () => void c.kick(r.id)));
  }
  for (const cell of t.querySelectorAll("th,td")) (cell as HTMLElement).style.padding = "3px 10px";
  return t;
}

const CELL_PX = 10;

/** The previewed room as a plain canvas grid: walls light, floor dark. */
function previewCanvas(rows: string[], width: number, height: number): HTMLCanvasElement {
  const canvas = el("canvas", "display:block;margin:8px 0;border:1px solid #444");
  canvas.width = width * CELL_PX;
  canvas.height = height * CELL_PX;
  const ctx = canvas.getContext("2d");
  if (!ctx) return canvas;
  ctx.fillStyle = "#1c2230";
  ctx.fillRect(0, 0, canvas.width, canvas.height);
  ctx.fillStyle = "#8a90a0";
  rows.forEach((row, y) => [...row].forEach((ch, x) => ch === "#" && ctx.fillRect(x * CELL_PX, y * CELL_PX, CELL_PX, CELL_PX)));
  return canvas;
}

function sizeInput(value: string): HTMLInputElement {
  const i = el("input", "width:3.5em;font:14px monospace");
  i.value = value;
  i.inputMode = "numeric";
  return i;
}

function mapBox(c: AdminController, m: AdminModel): HTMLElement {
  const w = sizeInput(m.sizeW);
  const h = sizeInput(m.sizeH);
  const walls = sizeInput(m.walls);
  const sync = () => c.editSize(w.value, h.value, walls.value);
  for (const i of [w, h, walls]) i.addEventListener("input", sync);
  const gen = button("Generate", () => void c.generate(w.value, h.value, walls.value));
  const box = el("div", "margin:12px 0;padding:8px;border:1px solid #444");
  box.append(el("p", "", `Map in play: ${m.mapWidth}x${m.mapHeight}`), el("p", "", "Size ", w, " x ", h, ", walls ", walls, `% (0-${WALL_LIMITS.max})`, gen));
  const p = m.preview;
  if (!p) return box;
  const apply = button("Apply", () => {
    if (confirm("Apply this map? Everybody respawns on the new map and the beat count restarts. Scores are kept.")) void c.applyMap();
  });
  apply.style.marginLeft = "0";
  box.append(previewCanvas(p.rows, p.width, p.height), el("p", "", `${p.width}x${p.height}, walls ${p.walls}%, seed ${p.seed}`, button("Regenerate", () => void c.regenerate())), el("p", "", apply));
  return box;
}

function gameBox(c: AdminController, m: AdminModel): HTMLElement {
  const minutes = el("input", "width:4em;font:14px monospace");
  minutes.inputMode = "decimal";
  const status = m.running ? (m.clock === null ? "running, no timer" : `running, ${m.clock} left`) : "game over";
  const start = button("Start new game", () => {
    if (confirm("Start a new game? The live board is emptied; players and programs stay.")) void c.startGame();
  });
  start.disabled = m.running;
  start.style.marginLeft = "0";
  return el(
    "div", "margin:12px 0;padding:8px;border:1px solid #444",
    el("p", "", `Game: ${status}`),
    el("p", "", "Timer ", minutes, " minutes", button("Set", () => void c.setTimer(minutes.value)), button("Cancel timer", () => void c.cancelTimer())),
    el("p", "", start),
  );
}

function panel(c: AdminController, m: AdminModel): HTMLElement {
  const count = el("input", "width:4em;font:14px monospace");
  count.value = String(m.minBots);
  count.inputMode = "numeric";
  const houses = el("p", "", "House bots fill the arena up to ", count, " bots", button("Set", () => void c.setMinBots(count.value)));
  const reset = button("Reset leaderboard", () => {
    if (confirm("Wipe the whole leaderboard? The game keeps running.")) void c.resetBoard();
  });
  reset.disabled = !m.running;
  reset.style.marginLeft = "0";
  return el("div", "", el("p", "", `Beat ${m.beat}`, button("Log out", () => void c.logout())), table(c, m), gameBox(c, m), houses, el("p", "", reset), mapBox(c, m), el("p", "color:#f88", m.message));
}

function render(root: HTMLElement, c: AdminController, m: AdminModel): void {
  const title = el("h2", "", "Arena admin");
  switch (m.phase) {
    case "checking":
      root.replaceChildren(title, "Loading...");
      return;
    case "disabled":
      root.replaceChildren(title, "Admin is disabled: the server has no ARENA_ADMIN_PASSWORD.");
      return;
    case "login":
      root.replaceChildren(title, loginForm(c, m.message));
      return;
    default:
      root.replaceChildren(title, panel(c, m));
  }
}

/** Mounts the admin page into root; returns an unmount function. */
export function mountAdmin(root: HTMLElement, api: AdminApi): () => void {
  root.style.cssText = "padding:16px;font:16px monospace";
  // The count input is rebuilt on every render; skip a refresh while it has focus so typing is not lost.
  const c: AdminController = new AdminController({ api, onChange: (m) => render(root, c, m) });
  const typing = () => root.contains(document.activeElement) && document.activeElement instanceof HTMLInputElement;
  const timer = setInterval(() => {
    if (c.state.phase === "panel" && !typing()) void c.refresh();
  }, REFRESH_MS);
  void c.refresh();
  return () => clearInterval(timer);
}
