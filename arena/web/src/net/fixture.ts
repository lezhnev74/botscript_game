import type { Message } from "../protocol/protocol";
import { isMessage } from "./parse";
import type { Handlers, MessageSource } from "./source";

/** Calls fn on every beat; pluggable so tests run instantly. */
export interface Ticker {
  start(fn: () => void): void;
  stop(): void;
}

export class IntervalTicker implements Ticker {
  private id: ReturnType<typeof setInterval> | null = null;
  constructor(private readonly ms: number) {}
  start(fn: () => void): void { this.stop(); this.id = setInterval(fn, this.ms); }
  stop(): void { if (this.id !== null) clearInterval(this.id); this.id = null; }
}

export interface FixtureOptions {
  /** Start over with the snapshot after the last message (like a reconnect). */
  loop?: boolean;
}

/** Plays a recorded message sequence as if it were the WebSocket. */
export class FixturePlayer implements MessageSource {
  private next = 0;
  private handlers: Handlers | null = null;

  constructor(
    private readonly messages: Message[],
    private readonly ticker: Ticker,
    private readonly opts: FixtureOptions = {},
  ) {}

  start(handlers: Handlers): void {
    this.handlers = handlers;
    this.next = 0;
    handlers.onStatus("open");
    this.emit();
    this.ticker.start(() => this.step());
  }

  stop(): void {
    this.ticker.stop();
    this.handlers = null;
  }

  private step(): void {
    if (this.next < this.messages.length) return this.emit();
    if (this.opts.loop) { this.next = 0; return this.emit(); }
    const h = this.handlers;
    this.stop();
    h?.onStatus("closed");
  }

  private emit(): void {
    const m = this.messages[this.next++];
    if (m) this.handlers?.onMessage(m);
  }
}

export type FetchFn = (url: string) => Promise<Response>;

/** Loads fixtures/<name>.json (exported by cmd/replay) and checks every message. */
export async function loadFixture(
  name: string,
  fetchFn: FetchFn,
  urlOf: (name: string) => string = (n) => `/fixtures/${n}.json`,
): Promise<Message[]> {
  const res = await fetchFn(urlOf(name));
  if (!res.ok) throw new Error(`fixture ${name}: HTTP ${res.status}`);
  const data: unknown = await res.json();
  if (!Array.isArray(data) || !data.every(isMessage)) throw new Error(`fixture ${name}: not a message list`);
  return data;
}
