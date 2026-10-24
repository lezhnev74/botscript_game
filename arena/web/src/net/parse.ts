import type { Message } from "../protocol/protocol";
import { TypeBeat, TypeSnapshot } from "../protocol/protocol";

const isObject = (v: unknown): v is Record<string, unknown> => typeof v === "object" && v !== null;

/** Shape check of a decoded message: enough to trust the typed access in the store. */
export function isMessage(v: unknown): v is Message {
  if (!isObject(v) || (v.type !== TypeSnapshot && v.type !== TypeBeat)) return false;
  const f = v.frame;
  return isObject(f) && typeof f.beat === "number" && Array.isArray(f.grid) && Array.isArray(f.bots) &&
    Array.isArray(v.events) && Array.isArray(v.board);
}

/** Decodes one WebSocket frame; null when it is not a valid message. */
export function parseMessage(data: unknown): Message | null {
  if (typeof data !== "string") return null;
  try {
    const v: unknown = JSON.parse(data);
    return isMessage(v) ? v : null;
  } catch {
    return null;
  }
}
