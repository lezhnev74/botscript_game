import { stripFence } from "./fence";
import type { SrcError } from "./markers";
import type { LookValue } from "./look-value";

/** An API failure as the editor sees it (structurally the net layer's ApiError). */
export interface Failure {
  kind: string;
  message: string;
  errors?: SrcError[];
  retryBeats?: number;
}

export interface EditorApi {
  join(name: string): Promise<{ botId: string }>;
  /** My bot; rebootLeftBeats (-1 = not rebooting) and beatMs tell how long a save keeps it off the map. */
  getBot(): Promise<{ name: string; source: string; look: LookValue; rebootLeftBeats?: number; beatMs?: number }>;
  putBot(source: string): Promise<void>;
}

/** The stored token: has() says whether we hold one, clear() forgets it (a lost token means a new bot). */
export interface Session {
  has(): boolean;
  clear(): void;
}

export type Phase = "need-name" | "joining" | "ready" | "full" | "offline";

export type SaveState =
  | { kind: "idle" }
  | { kind: "saving" }
  /** rebootSeconds: the bot is rebooting and returns in that many seconds (absent: the program runs from the next beat). */
  | { kind: "saved"; rebootSeconds?: number }
  | { kind: "errors"; errors: SrcError[] }
  | { kind: "wait"; beats: number }
  | { kind: "failed"; message: string };

export interface EditorModel {
  phase: Phase;
  botName: string;
  /** My bot's saved look as loaded; a new object only on a (re)load. */
  look: LookValue | null;
  /** The text in the editor box. */
  source: string;
  /** The last program the server accepted (what the bot runs). */
  savedSource: string;
  save: SaveState;
  message: string;
}

export type Outcome =
  | { type: "relogin" }
  | { type: "full"; message: string }
  | { type: "errors"; errors: SrcError[] }
  | { type: "wait"; beats: number }
  | { type: "failed"; message: string };

const isFailure = (e: unknown): e is Failure => typeof e === "object" && e !== null && "kind" in e;

/** Maps a thrown API error to what the UI does about it. */
export function outcomeOf(e: unknown): Outcome {
  if (!isFailure(e)) return { type: "failed", message: "unexpected error" };
  switch (e.kind) {
    case "unauthorized": return { type: "relogin" };
    case "full": return { type: "full", message: e.message };
    case "validation": return { type: "errors", errors: e.errors ?? [] };
    case "rate_limited": return { type: "wait", beats: e.retryBeats ?? 1 };
    default: return { type: "failed", message: e.message };
  }
}

/** Whole seconds (rounded up) of `beats` beats; 0 when the bot is not rebooting (beats < 1) or the beat length is unknown. */
export function rebootSeconds(beats: number, beatMs: number): number {
  return beats > 0 && beatMs > 0 ? Math.ceil((beats * beatMs) / 1000) : 0;
}

const LOST = "Your session was lost: pick a name to get a new bot.";

export interface ControllerDeps {
  api: EditorApi;
  session: Session;
  /** Called with my bot id after a join, null when the token was dropped. */
  onJoined(botId: string | null): void;
  onChange(model: EditorModel): void;
}

/** True when the box differs from the saved program. */
export const isDirty = (m: EditorModel): boolean => m.source !== m.savedSource;

/** Save is allowed only for a ready editor with unsaved changes and no request in flight. */
export const canSave = (m: EditorModel): boolean => m.phase === "ready" && isDirty(m) && m.save.kind !== "saving";

/** The editor's join / save flow; the DOM glue only renders models and forwards clicks. */
export class EditorController {
  private model: EditorModel = { phase: "need-name", botName: "", look: null, source: "", savedSource: "", save: { kind: "idle" }, message: "" };
  private lastName = "";
  private draft: string | null = null;

  constructor(private readonly deps: ControllerDeps) {}

  async start(): Promise<void> {
    if (!this.deps.session.has()) return this.set({ phase: "need-name" });
    await this.load();
  }

  async join(rawName: string): Promise<void> {
    const name = rawName.trim();
    this.lastName = name;
    if (name === "") return this.set({ phase: "need-name", message: "Enter a name." });
    this.set({ phase: "joining", message: "" });
    try {
      const { botId } = await this.deps.api.join(name);
      this.deps.onJoined(botId);
    } catch (e) {
      return this.joinFailed(outcomeOf(e));
    }
    await this.load();
    await this.restoreDraft();
  }

  /** The "Retry" button: reload my bot when offline, join again with the same name when the arena was full. */
  async retry(): Promise<void> {
    if (this.model.phase === "offline") await this.load();
    else if (this.model.phase === "full") await this.join(this.lastName);
  }

  async save(raw: string): Promise<void> {
    const source = stripFence(raw);
    this.set({ source, save: { kind: "saving" } });
    try {
      await this.deps.api.putBot(source);
      const rebootSeconds = await this.rebootSeconds();
      this.set({ savedSource: source, save: rebootSeconds > 0 ? { kind: "saved", rebootSeconds } : { kind: "saved" } });
    } catch (e) {
      this.saveFailed(outcomeOf(e), source);
    }
  }

  /** Seconds until my bot is back from the reboot a save started; 0 when it is not rebooting (or this cannot be read). */
  private async rebootSeconds(): Promise<number> {
    try {
      const bot = await this.deps.api.getBot();
      return rebootSeconds(bot.rebootLeftBeats ?? -1, bot.beatMs ?? 0);
    } catch {
      return 0;
    }
  }

  /** The profile was saved: the page title follows the new name. */
  renamed(name: string): void {
    this.set({ botName: name });
  }

  /** The server no longer knows my token (seen while saving the profile). */
  expire(): void {
    this.relogin();
  }

  /** The beat poll: only retries a failed load. */
  async poll(): Promise<void> {
    if (this.model.phase === "offline") await this.retry();
  }

  /** The player typed in (or a preset filled) the box. Clears only the "saved" success; errors stay until the next save. */
  edit(source: string): void {
    if (source === this.model.source) return;
    const save: SaveState = this.model.save.kind === "saved" ? { kind: "idle" } : this.model.save;
    this.set({ source, save });
  }

  /** Puts a preset in the box; asks first (via confirm) when that would drop unsaved changes. */
  loadPreset(source: string, confirm: (question: string) => boolean): void {
    if (this.model.phase !== "ready") return;
    if (isDirty(this.model) && !confirm("Replace your unsaved changes with this preset?")) return;
    this.edit(source);
  }

  private async load(): Promise<void> {
    try {
      const bot = await this.deps.api.getBot();
      this.set({ phase: "ready", botName: bot.name, look: bot.look, source: bot.source, savedSource: bot.source, message: "" });
    } catch (e) {
      const o = outcomeOf(e);
      if (o.type === "relogin") this.relogin();
      else this.set({ phase: "offline", message: "Offline - retrying..." });
    }
  }

  private async restoreDraft(): Promise<void> {
    if (this.draft === null || this.model.phase !== "ready") return;
    const d = this.draft;
    this.draft = null;
    await this.save(d);
  }

  private joinFailed(o: Outcome): void {
    if (o.type === "full") this.set({ phase: "full", message: o.message });
    else this.set({ phase: "need-name", message: o.type === "failed" ? o.message : "Could not join." });
  }

  private saveFailed(o: Outcome, source: string): void {
    switch (o.type) {
      case "relogin": this.draft = source; return this.relogin();
      case "errors": return this.set({ save: { kind: "errors", errors: o.errors } });
      case "wait": return this.set({ save: { kind: "wait", beats: o.beats } });
      default: return this.set({ save: { kind: "failed", message: "message" in o ? o.message : "save failed" } });
    }
  }

  private relogin(): void {
    this.deps.session.clear();
    this.deps.onJoined(null);
    this.set({ phase: "need-name", save: { kind: "idle" }, message: LOST + (this.draft === null ? "" : " Your draft is kept.") });
  }

  private set(patch: Partial<EditorModel>): void {
    this.model = { ...this.model, ...patch };
    this.deps.onChange(this.model);
  }
}
