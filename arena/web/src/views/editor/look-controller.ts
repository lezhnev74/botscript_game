import { faceOf, type Face } from "../../render/face";
import { BODIES, EYES, HATS, MOUTHS } from "../../render/shapes";
import { outcomeOf } from "./controller";
import type { LookValue } from "./look-value";

export type { LookValue };

export type Part = "body" | "eyes" | "mouth" | "hat";
export type Field = Part | "color" | "name";

export interface ProfileApi {
  putProfile(name: string, look: LookValue): Promise<void>;
}

export type LookSave =
  | { kind: "idle" }
  | { kind: "saving" }
  | { kind: "saved" }
  | { kind: "wait"; beats: number }
  | { kind: "failed"; message: string };

export interface LookModel {
  name: string;
  look: LookValue;
  savedName: string;
  savedLook: LookValue;
  dirty: boolean;
  canSave: boolean;
  save: LookSave;
  /** A rejected field with the server's words, shown next to its control. */
  field: { field: Field; message: string } | null;
}

export interface LookDeps {
  api: ProfileApi;
  onChange(m: LookModel): void;
  /** The server accepted this name and look. */
  onSaved(name: string, look: LookValue): void;
  /** The token is no longer valid. */
  onUnauthorized(): void;
  /** Cosmetic randomness (not the sim). */
  random?: () => number;
}

const LISTS: Record<Part, readonly string[]> = { body: BODIES, eyes: EYES, mouth: MOUTHS, hat: HATS };

/** The values a row of tiles offers, straight from the render lists. */
export const tileValues = (part: Part): string[] => [...LISTS[part]];

/** What a tile of `part` = `value` shows: the current look with that one part replaced. */
export const tileLook = (look: LookValue, part: Part, value: string): LookValue => ({ ...look, [part]: value });

/** The preview cycles resting, hit, kill, dead. */
export const PREVIEW_STEPS = 4;
export function previewFace(look: LookValue, step: number): Face {
  switch (step % PREVIEW_STEPS) {
    case 1: return faceOf(look, { dead: false, hitBeat: 0 }, 0);
    case 2: return faceOf(look, { dead: false, killBeat: 0 }, 0);
    case 3: return faceOf(look, { dead: true }, 0);
    default: return faceOf(look, { dead: false }, 0);
  }
}

/** Which control a server message about a rejected profile belongs to. */
export function fieldOf(message: string): Field | null {
  const m = /^bad look: (color|body|eyes|mouth|hat)$/.exec(message);
  if (m) return m[1] as Field;
  return message.startsWith("name must") ? "name" : null;
}

const same = (a: LookValue, b: LookValue): boolean =>
  a.color.toLowerCase() === b.color.toLowerCase() && a.body === b.body && a.eyes === b.eyes && a.mouth === b.mouth && a.hat === b.hat;

/** The name and look editing flow of the my bot page; the DOM glue renders models and forwards input. */
export class LookController {
  private name = "";
  private look: LookValue = { color: "#808080", body: "circle", eyes: "dots", mouth: "smile", hat: "none" };
  private savedName = "";
  private savedLook = this.look;
  private save_: LookSave = { kind: "idle" };
  private field: LookModel["field"] = null;

  constructor(private readonly deps: LookDeps) {}

  /** Sets the saved reference (from the server) and clears every message. */
  load(name: string, look: LookValue): void {
    this.name = name;
    this.look = look;
    this.savedName = name;
    this.savedLook = look;
    this.save_ = { kind: "idle" };
    this.field = null;
    this.emit();
  }

  setName(name: string): void { this.name = name; this.edited(); }
  setColor(color: string): void { this.look = { ...this.look, color }; this.edited(); }
  setPart(part: Part, value: string): void { this.look = tileLook(this.look, part, value); this.edited(); }

  /** Rolls a color and every part; the result always differs from the current look. */
  randomize(): void {
    const rnd = this.deps.random ?? Math.random;
    const pick = (list: readonly string[]): string => list[Math.min(list.length - 1, Math.floor(rnd() * list.length))]!;
    let rgb = Math.min(0xffffff, Math.floor(rnd() * 0x1000000));
    const roll = (): LookValue => ({ color: `#${rgb.toString(16).padStart(6, "0")}`, body: pick(BODIES), eyes: pick(EYES), mouth: pick(MOUTHS), hat: pick(HATS) });
    let next = roll();
    if (same(next, this.look)) {
      rgb = (rgb + 1) & 0xffffff;
      next = roll();
    }
    this.look = next;
    this.edited();
  }

  async save(): Promise<void> {
    if (!this.model().canSave) return;
    const name = this.name.trim();
    if (name === "") { this.field = { field: "name", message: "Enter a name." }; return this.emit(); }
    const look = this.look;
    this.save_ = { kind: "saving" };
    this.field = null;
    this.emit();
    try {
      await this.deps.api.putProfile(name, look);
      if (this.name.trim() === name) this.name = name; // only tidies whitespace, never drops an edit
      this.savedName = name;
      this.savedLook = look;
      this.save_ = { kind: "saved" };
      this.emit();
      this.deps.onSaved(name, look);
    } catch (e) {
      this.failed(e);
    }
  }

  private failed(e: unknown): void {
    const o = outcomeOf(e);
    if (o.type === "relogin") { this.save_ = { kind: "idle" }; this.emit(); return this.deps.onUnauthorized(); }
    const message = o.type === "failed" || o.type === "full" ? o.message : "";
    const field = fieldOf(message);
    this.field = field === null ? null : { field, message };
    if (o.type === "wait") this.save_ = { kind: "wait", beats: o.beats };
    else this.save_ = field === null ? { kind: "failed", message: message || "save failed" } : { kind: "idle" };
    this.emit();
  }

  private edited(): void {
    this.field = null;
    if (this.save_.kind === "saved" || this.save_.kind === "wait") this.save_ = { kind: "idle" };
    this.emit();
  }

  private model(): LookModel {
    const dirty = this.name.trim() !== this.savedName || !same(this.look, this.savedLook);
    return {
      name: this.name, look: this.look, savedName: this.savedName, savedLook: this.savedLook, dirty,
      canSave: dirty && this.save_.kind !== "saving", save: this.save_, field: this.field,
    };
  }

  private emit(): void { this.deps.onChange(this.model()); }
}
