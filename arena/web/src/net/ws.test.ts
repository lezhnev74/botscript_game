import { describe, expect, it } from "vitest";
import type { Message } from "../protocol/protocol";
import { parseMessage } from "./parse";
import { WsClient, type Scheduler, type SocketLike } from "./ws";
import type { Status } from "./source";

class FakeSocket implements SocketLike {
  onopen: (() => void) | null = null;
  onmessage: ((ev: { data: unknown }) => void) | null = null;
  onclose: (() => void) | null = null;
  closed = false;
  close() { this.closed = true; this.onclose?.(); }
  open() { this.onopen?.(); }
  send(data: unknown) { this.onmessage?.({ data }); }
  drop() { this.onclose?.(); }
}

class FakeScheduler {
  pending: { fn: () => void; ms: number; cancelled: boolean }[] = [];
  schedule: Scheduler = (fn, ms) => {
    const t = { fn, ms, cancelled: false };
    this.pending.push(t);
    return () => { t.cancelled = true; };
  };
  /** Fires the oldest live timer. */
  fire() { this.pending.find((t) => !t.cancelled && (t.cancelled = true))?.fn(); }
  live() { return this.pending.filter((t) => !t.cancelled); }
}

const beatMsg = (beat: number): Message => ({
  type: "beat",
  frame: { beat, width: 1, height: 1, grid: ["."], bots: [], fallen: [], rebooting: [], bullets: [], coins: [], maxHp: 4, reload: 3 },
  events: [], board: [], remaining: -1, gameOver: false, beatMs: 0,
});

function setup(random: () => number = () => 1) {
  const sockets: FakeSocket[] = [];
  const sched = new FakeScheduler();
  const messages: Message[] = [];
  const statuses: Status[] = [];
  const client = new WsClient({
    url: "ws://x/ws",
    createSocket: () => { const s = new FakeSocket(); sockets.push(s); return s; },
    schedule: sched.schedule,
    baseDelayMs: 100,
    maxDelayMs: 500,
    random,
  });
  client.start({ onMessage: (m) => messages.push(m), onStatus: (s) => statuses.push(s) });
  return { client, sockets, sched, messages, statuses };
}

describe("parseMessage", () => {
  it("accepts snapshot and beat messages", () => {
    expect(parseMessage(JSON.stringify(beatMsg(3)))?.frame.beat).toBe(3);
  });
  it.each([
    ["not json", "{"],
    ["not an object", "5"],
    ["unknown type", JSON.stringify({ ...beatMsg(1), type: "weird" })],
    ["no frame", JSON.stringify({ type: "beat", events: [], board: [] })],
    ["events not an array", JSON.stringify({ ...beatMsg(1), events: null })],
    ["binary frame", new ArrayBuffer(2)],
  ])("rejects %s", (_n, data) => expect(parseMessage(data)).toBeNull());
});

describe("WsClient", () => {
  it("reports connecting, open and delivers parsed messages", () => {
    const { sockets, messages, statuses } = setup();
    expect(statuses).toEqual(["connecting"]);
    sockets[0]!.open();
    sockets[0]!.send(JSON.stringify(beatMsg(1)));
    expect(statuses).toEqual(["connecting", "open"]);
    expect(messages.map((m) => m.frame.beat)).toEqual([1]);
  });
  it("drops garbage messages", () => {
    const { sockets, messages } = setup();
    sockets[0]!.open();
    sockets[0]!.send("garbage");
    expect(messages).toEqual([]);
  });
  it("reconnects with exponential backoff capped at the max", () => {
    const { sockets, sched, statuses } = setup();
    for (const want of [100, 200, 400, 500, 500]) {
      sockets.at(-1)!.drop();
      expect(sched.live().map((t) => t.ms)).toEqual([want]);
      sched.fire();
    }
    expect(sockets).toHaveLength(6);
    expect(statuses.filter((s) => s === "closed")).toHaveLength(5);
  });
  it("jitters the delay between half and the full backoff", () => {
    const { sockets, sched } = setup(() => 0);
    sockets[0]!.drop();
    expect(sched.live().map((t) => t.ms)).toEqual([50]);
  });
  it("resets the backoff once a connection opened", () => {
    const { sockets, sched } = setup();
    sockets[0]!.drop(); sched.fire();
    sockets[1]!.drop(); sched.fire();
    sockets[2]!.open();
    sockets[2]!.drop();
    expect(sched.live().map((t) => t.ms)).toEqual([100]);
  });
  it("stop closes the socket and does not reconnect", () => {
    const { client, sockets, sched } = setup();
    sockets[0]!.open();
    client.stop();
    expect(sockets[0]!.closed).toBe(true);
    expect(sched.live()).toEqual([]);
  });
  it("stop cancels a pending reconnect", () => {
    const { client, sockets, sched } = setup();
    sockets[0]!.drop();
    client.stop();
    expect(sched.live()).toEqual([]);
    expect(sockets).toHaveLength(1);
  });
});
