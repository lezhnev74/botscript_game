import { formatRemaining } from "../../store/clock";

/** The status the panel shows (structurally the protocol's AdminStatus: views do not import the wire types). */
export interface AdminStatus {
  bots: { id: string; name: string; house: boolean; score: number }[];
  minBots: number;
  beat: number;
  mapWidth: number;
  mapHeight: number;
  /** False once the timer has ended the game, until the next one is started. */
  running: boolean;
  /** Beats left on the timer: -1 = no timer, 0 = over. */
  remaining: number;
  beatMs: number;
}

/** An API failure as the admin panel sees it (structurally the net layer's ApiError). */
export interface Failure {
  kind: string;
  status: number;
  message: string;
}

export interface AdminApi {
  login(password: string): Promise<void>;
  logout(): Promise<void>;
  status(): Promise<AdminStatus>;
  kick(id: string): Promise<void>;
  resetBoard(): Promise<void>;
  setMinBots(n: number): Promise<void>;
  previewMap(width: number, height: number, walls: number): Promise<{ seed: number; map: string }>;
  applyMap(map: string): Promise<void>;
  /** Ends the game after this many minutes; 0 cancels the timer. */
  setTimer(minutes: number): Promise<void>;
  startGame(): Promise<void>;
}

/** Room size limits of the generator (the server enforces the same). */
/** The longest game the timer accepts (the server enforces the same). */
const MAX_MINUTES = 24 * 60;

export const SIZE_LIMITS = { minW: 9, maxW: 41, minH: 7, maxH: 31 };

/** Interior wall share of a generated room in percent: the most allowed and the starting value (the server enforces the same). */
export const WALL_LIMITS = { max: 15, initial: 3 };

/** checking: first status request in flight; disabled: the server has no admin password; login: password form; panel: logged in. */
export type Phase = "checking" | "disabled" | "login" | "panel";

export interface BotRow {
  id: string;
  name: string;
  kind: "house" | "human";
  score: number;
  /** Only human bots can be kicked; house bots are controlled by the house-bot count. */
  canKick: boolean;
}

/** A generated room waiting for the admin's decision. */
export interface MapPreview {
  seed: number;
  width: number;
  height: number;
  /** Percent of interior cells walled. */
  walls: number;
  /** One string per row, '#' wall, '.' floor. */
  rows: string[];
  /** The map text exactly as the server made it (what Apply sends). */
  text: string;
}

export interface AdminModel {
  phase: Phase;
  rows: BotRow[];
  minBots: number;
  beat: number;
  /** Size of the map in play. */
  mapWidth: number;
  mapHeight: number;
  /** The size inputs' text; kept here so a refresh does not reset what the admin typed. */
  sizeW: string;
  sizeH: string;
  /** The wall percent input's text. */
  walls: string;
  preview: MapPreview | null;
  /** False once the timer has ended the game (the "Start new game" button is enabled only then). */
  running: boolean;
  /** The timer as "mm:ss", null when none runs. */
  clock: string | null;
  message: string;
}

const isFailure = (e: unknown): e is Failure => typeof e === "object" && e !== null && "kind" in e && "status" in e;

/** Parses the house-bot count input: a whole number, 0 or more. */
export function parseCount(text: string): number | null {
  const t = text.trim();
  return /^\d{1,4}$/.test(t) ? Number(t) : null;
}

/** Parses the timer input: minutes, up to two decimals, more than 0 and at most a day (0 is the Cancel button). */
export function parseMinutes(text: string): number | null {
  const t = text.trim();
  if (!/^\d{1,4}(\.\d{1,2})?$/.test(t)) return null;
  const m = Number(t);
  return m > 0 && m <= MAX_MINUTES ? m : null;
}

/** Parses the size inputs of the map generator; null outside the generator's limits. */
export function parseSize(w: string, h: string): { width: number; height: number } | null {
  const width = parseCount(w);
  const height = parseCount(h);
  if (width === null || height === null) return null;
  const { minW, maxW, minH, maxH } = SIZE_LIMITS;
  return width >= minW && width <= maxW && height >= minH && height <= maxH ? { width, height } : null;
}

/** Parses the wall percent input: a whole number, 0 to WALL_LIMITS.max. */
export function parseWalls(text: string): number | null {
  const n = parseCount(text);
  return n !== null && n <= WALL_LIMITS.max ? n : null;
}

/** Table rows of a status: humans first (those can be kicked), then house bots, each group in the server's order. */
export function rowsOf(s: AdminStatus): BotRow[] {
  const rows = s.bots.map((b): BotRow => ({ id: b.id, name: b.name, kind: b.house ? "house" : "human", score: b.score, canKick: !b.house }));
  return [...rows.filter((r) => r.canKick), ...rows.filter((r) => !r.canKick)];
}

const NEEDS_COUNT = "House bots: enter a whole number, 0 or more.";
const NEEDS_SIZE = `Map size: width ${SIZE_LIMITS.minW} to ${SIZE_LIMITS.maxW}, height ${SIZE_LIMITS.minH} to ${SIZE_LIMITS.maxH}.`;
const NEEDS_WALLS = `Walls: a whole number of percent, 0 to ${WALL_LIMITS.max}.`;
const NEEDS_PREVIEW = 'Press "Generate" first.';
const NEEDS_MINUTES = "Timer: enter minutes, more than 0 and at most 1440 (use Cancel to remove it).";
const APPLIED = "Map applied: the new game starts at the next beat.";

export interface ControllerDeps {
  api: AdminApi;
  onChange(model: AdminModel): void;
}

/** The admin panel flow: status check, login, refresh, kick, reset, house-bot count. The DOM glue only renders models and forwards clicks. */
export class AdminController {
  private model: AdminModel = {
    phase: "checking", rows: [], minBots: 0, beat: 0, mapWidth: 0, mapHeight: 0, sizeW: "", sizeH: "", walls: String(WALL_LIMITS.initial), preview: null, running: true, clock: null, message: "",
  };
  private sizeTouched = false;

  constructor(private readonly deps: ControllerDeps) {}

  get state(): AdminModel {
    return this.model;
  }

  /** Loads the status; the answer decides between panel, login form and "disabled". */
  async refresh(): Promise<void> {
    try {
      const s = await this.deps.api.status();
      const size = this.sizeTouched ? {} : { sizeW: String(s.mapWidth), sizeH: String(s.mapHeight) };
      const game = { running: s.running, clock: formatRemaining(s.remaining, s.beatMs) };
      this.set({ phase: "panel", rows: rowsOf(s), minBots: s.minBots, beat: s.beat, mapWidth: s.mapWidth, mapHeight: s.mapHeight, ...game, ...size });
    } catch (e) {
      this.fail(e, "");
    }
  }

  async login(password: string): Promise<void> {
    try {
      await this.deps.api.login(password);
    } catch (e) {
      this.set({ phase: "login", message: loginMessage(e) });
      return;
    }
    await this.refresh();
  }

  async logout(): Promise<void> {
    await this.deps.api.logout().catch(() => undefined);
    this.set({ phase: "login", rows: [], message: "" });
  }

  kick(id: string): Promise<void> {
    return this.act(() => this.deps.api.kick(id));
  }

  resetBoard(): Promise<void> {
    return this.act(() => this.deps.api.resetBoard());
  }

  setMinBots(text: string): Promise<void> {
    const n = parseCount(text);
    if (n === null) {
      this.set({ message: NEEDS_COUNT });
      return Promise.resolve();
    }
    return this.act(() => this.deps.api.setMinBots(n));
  }

  /** Ends the game after the typed number of minutes. */
  setTimer(text: string): Promise<void> {
    const m = parseMinutes(text);
    if (m === null) {
      this.set({ message: NEEDS_MINUTES });
      return Promise.resolve();
    }
    return this.act(() => this.deps.api.setTimer(m));
  }

  cancelTimer(): Promise<void> {
    return this.act(() => this.deps.api.setTimer(0));
  }

  /** Starts the next game: an empty board, the same players and programs. */
  startGame(): Promise<void> {
    return this.act(() => this.deps.api.startGame());
  }

  /** Remembers the size and wall inputs' text without re-rendering (the inputs are being typed in). */
  editSize(w: string, h: string, walls = this.model.walls): void {
    this.sizeTouched = true;
    this.model = { ...this.model, sizeW: w, sizeH: h, walls };
  }

  /** Asks the server for a room of the typed size and wall percent (the server draws a new seed every time). */
  async generate(w: string, h: string, wallsText = this.model.walls): Promise<void> {
    const size = parseSize(w, h);
    const walls = parseWalls(wallsText);
    this.editSize(w, h, wallsText);
    if (!size) return this.set({ message: NEEDS_SIZE });
    if (walls === null) return this.set({ message: NEEDS_WALLS });
    try {
      const p = await this.deps.api.previewMap(size.width, size.height, walls);
      this.set({ preview: { seed: p.seed, ...size, walls, rows: p.map.split("\n").filter((r) => r !== ""), text: p.map }, message: "" });
    } catch (e) {
      this.fail(e, "Failed");
    }
  }

  /** A new room of the previewed size and wall percent. */
  regenerate(): Promise<void> {
    const p = this.model.preview;
    if (!p) {
      this.set({ message: NEEDS_PREVIEW });
      return Promise.resolve();
    }
    return this.generate(String(p.width), String(p.height), String(p.walls));
  }

  /** Applies the previewed map: a new game on it at the next beat, scores and players kept. */
  async applyMap(): Promise<void> {
    const p = this.model.preview;
    if (!p) return this.set({ message: NEEDS_PREVIEW });
    try {
      await this.deps.api.applyMap(p.text);
    } catch (e) {
      this.fail(e, "Failed");
      return;
    }
    // The new map starts at the next beat, so the status still shows the old size: keep the applied one in the inputs.
    this.sizeTouched = true;
    this.set({ preview: null, message: APPLIED, sizeW: String(p.width), sizeH: String(p.height) });
    await this.refresh();
  }

  /** Runs a command, then refreshes the table; a failure shows its message (or the login form on a lost session). */
  private async act(command: () => Promise<void>): Promise<void> {
    try {
      await command();
    } catch (e) {
      this.fail(e, "Failed");
      return;
    }
    this.set({ message: "" });
    await this.refresh();
  }

  private fail(e: unknown, fallback: string): void {
    if (!isFailure(e)) return this.set({ message: fallback || "unexpected error" });
    if (e.status === 404 && this.model.phase === "checking") return this.set({ phase: "disabled", message: "" });
    if (e.kind === "unauthorized") return this.set({ phase: "login", rows: [], message: "" });
    // A refresh that fails on the network keeps the last table and says so.
    this.set({ message: e.message });
  }

  private set(patch: Partial<AdminModel>): void {
    this.model = { ...this.model, ...patch };
    this.deps.onChange(this.model);
  }
}

function loginMessage(e: unknown): string {
  if (!isFailure(e)) return "unexpected error";
  if (e.kind === "unauthorized") return "Wrong password.";
  if (e.kind === "rate_limited") return "Too many wrong passwords: wait a minute.";
  return e.message;
}
