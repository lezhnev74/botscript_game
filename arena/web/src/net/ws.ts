import { parseMessage } from "./parse";
import type { Handlers, MessageSource } from "./source";

/** The part of the browser WebSocket the client uses (fakeable). */
export interface SocketLike {
  onopen: (() => void) | null;
  onmessage: ((ev: { data: unknown }) => void) | null;
  onclose: (() => void) | null;
  close(): void;
}

/** Runs fn after ms; returns a cancel function. */
export type Scheduler = (fn: () => void, ms: number) => () => void;

export const browserScheduler: Scheduler = (fn, ms) => {
  const id = setTimeout(fn, ms);
  return () => clearTimeout(id);
};

export interface WsOptions {
  url: string;
  createSocket(url: string): SocketLike;
  schedule?: Scheduler;
  baseDelayMs?: number;
  maxDelayMs?: number;
  /** Source of jitter in [0,1); Math.random by default. */
  random?: () => number;
}

/** WebSocket client: parses messages, reconnects with exponential backoff. */
export class WsClient implements MessageSource {
  private handlers: Handlers | null = null;
  private socket: SocketLike | null = null;
  private cancelRetry: (() => void) | null = null;
  private failures = 0;
  private readonly schedule: Scheduler;
  private readonly base: number;
  private readonly max: number;
  private readonly random: () => number;

  constructor(private readonly opts: WsOptions) {
    this.schedule = opts.schedule ?? browserScheduler;
    this.base = opts.baseDelayMs ?? 500;
    this.max = opts.maxDelayMs ?? 10_000;
    this.random = opts.random ?? Math.random;
  }

  start(handlers: Handlers): void {
    this.handlers = handlers;
    this.connect();
  }

  stop(): void {
    this.handlers = null;
    this.cancelRetry?.();
    this.cancelRetry = null;
    const s = this.socket;
    this.socket = null;
    if (s) { s.onclose = null; s.close(); }
  }

  private connect(): void {
    this.cancelRetry = null;
    this.handlers?.onStatus("connecting");
    const s = this.opts.createSocket(this.opts.url);
    this.socket = s;
    s.onopen = () => { this.failures = 0; this.handlers?.onStatus("open"); };
    s.onmessage = (ev) => {
      const msg = parseMessage(ev.data);
      if (msg) this.handlers?.onMessage(msg);
    };
    s.onclose = () => this.lost();
  }

  private lost(): void {
    if (!this.handlers) return;
    this.socket = null;
    this.handlers.onStatus("closed");
    const delay = Math.min(this.base * 2 ** this.failures, this.max) * (0.5 + 0.5 * this.random());
    this.failures++;
    this.cancelRetry = this.schedule(() => this.connect(), delay);
  }
}
