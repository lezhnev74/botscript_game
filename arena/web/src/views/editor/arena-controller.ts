import { outcomeOf, rebootSeconds } from "./controller";
import type { Tone } from "./dom-kit";

/** The part of GET /api/bot the arena card uses (structurally the protocol's BotBody). */
export interface ArenaBot {
  arena: { in: boolean; leftBy: string; stayLeftBeats: number };
  beatMs: number;
  /** Beats until the bot is back from a reboot, -1 when it is not rebooting. */
  rebootLeftBeats?: number;
  /** The length of a full reboot in beats (the bar's full length). */
  rebootTotalBeats?: number;
}

export interface ArenaApi {
  getBot(): Promise<ArenaBot>;
  leaveArena(): Promise<void>;
  returnToArena(): Promise<void>;
}

export interface ArenaModel {
  /** "loading" until the first answer. */
  view: "loading" | "in" | "out";
  /** Time left of the stay, e.g. "leaves automatically in 23 h 59 min"; "" when there is no limit. */
  stayText: string;
  /** Extra line for a detached bot, e.g. that its stay ran out. */
  note: string;
  busy: boolean;
  message: { tone: Tone; text: string } | null;
  /** Set while the bot is off the map for a reboot: the seconds left (0 = the countdown is over, it is returning) and how far it is (0..1). */
  reboot: { secondsLeft: number; progress: number } | null;
}

export interface ArenaDeps {
  api: ArenaApi;
  onChange(m: ArenaModel): void;
  /** The token is no longer valid. */
  onUnauthorized(): void;
}

/** Beats between two refreshes while the page stays open (every beat while rebooting). */
const REFRESH_BEATS = 30;

/** "23 h 59 min", "24 h", "10 min": the time of `beats` beats, rounded down to minutes. */
export function formatStay(beats: number, beatMs: number): string {
  const minutes = Math.floor((beats * beatMs) / 60000);
  const h = Math.floor(minutes / 60);
  const m = minutes % 60;
  if (h === 0) return m === 0 ? "less than a minute" : `${m} min`;
  return m === 0 ? `${h} h` : `${h} h ${m} min`;
}

/** The leave / return flow of the my bot page; the DOM glue renders models and forwards clicks. */
export class ArenaController {
  private model: ArenaModel = { view: "loading", stayText: "", note: "", busy: false, message: null, reboot: null };
  private polls = 0;

  constructor(private readonly deps: ArenaDeps) {}

  /** Reads the bot's arena state (on page load, after each action, every 30 beats). */
  async load(): Promise<void> {
    this.polls = 0;
    try {
      this.show(await this.deps.api.getBot());
    } catch (e) {
      this.failed(e);
    }
  }

  /** The beat poll: refreshes now and then, so an automatic leave shows up, and every beat during a reboot. */
  async poll(): Promise<void> {
    if ((this.model.reboot !== null || ++this.polls >= REFRESH_BEATS) && !this.model.busy) await this.load();
  }

  /** The "Leave the arena" button; asks first. */
  async leave(confirm: () => boolean): Promise<void> {
    if (this.model.busy || !confirm()) return;
    await this.act(() => this.deps.api.leaveArena());
  }

  /** The "Return to Arena" button. */
  async returnToArena(): Promise<void> {
    if (this.model.busy) return;
    await this.act(() => this.deps.api.returnToArena());
  }

  private async act(call: () => Promise<void>): Promise<void> {
    this.set({ busy: true, message: null });
    try {
      await call();
    } catch (e) {
      this.failed(e);
      return;
    }
    await this.load();
  }

  private show(bot: ArenaBot): void {
    const a = bot.arena;
    const stayText = a.in && a.stayLeftBeats >= 0 ? `leaves automatically in ${formatStay(a.stayLeftBeats, bot.beatMs)}` : "";
    const note = !a.in && a.leftBy === "time" ? "It left automatically after 24 hours." : "";
    this.set({ view: a.in ? "in" : "out", stayText, note, busy: false, message: null, reboot: this.rebootOf(bot) });
  }

  /** The reboot countdown of a bot in the arena, null when it is not rebooting. 0 beats left = still off the map (no free cell yet). */
  private rebootOf(bot: ArenaBot): ArenaModel["reboot"] {
    const left = bot.rebootLeftBeats ?? -1;
    if (!bot.arena.in || left < 0) return null;
    const total = Math.max(bot.rebootTotalBeats ?? 0, left, 1);
    return { secondsLeft: rebootSeconds(left, bot.beatMs), progress: 1 - left / total };
  }

  private failed(e: unknown): void {
    const o = outcomeOf(e);
    if (o.type === "relogin") {
      this.set({ busy: false });
      return this.deps.onUnauthorized();
    }
    const text = o.type === "wait" ? `Too many changes: try again in ${o.beats} beat(s).` : "message" in o ? o.message : "request failed";
    this.set({ busy: false, message: { tone: "error", text } });
  }

  private set(patch: Partial<ArenaModel>): void {
    this.model = { ...this.model, ...patch };
    this.deps.onChange(this.model);
  }
}
