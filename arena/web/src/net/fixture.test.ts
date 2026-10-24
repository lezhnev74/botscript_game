import { describe, expect, it } from "vitest";
import type { Message } from "../protocol/protocol";
import { FixturePlayer, loadFixture, type Ticker } from "./fixture";
import type { Status } from "./source";

class ManualTicker implements Ticker {
  fn: (() => void) | null = null;
  start(fn: () => void) { this.fn = fn; }
  stop() { this.fn = null; }
  tick(n = 1) { for (let i = 0; i < n; i++) this.fn?.(); }
}

const msg = (type: string, beat: number): Message => ({
  type,
  frame: { beat, width: 1, height: 1, grid: ["."], bots: [], fallen: [], rebooting: [], bullets: [], coins: [], maxHp: 4, reload: 3 },
  events: [], board: [], remaining: -1, gameOver: false, beatMs: 0,
});
const script = [msg("snapshot", 0), msg("beat", 1), msg("beat", 2)];

function play(loop = false) {
  const ticker = new ManualTicker();
  const player = new FixturePlayer(script, ticker, { loop });
  const got: number[] = [];
  const statuses: Status[] = [];
  player.start({ onMessage: (m) => got.push(m.frame.beat), onStatus: (s) => statuses.push(s) });
  return { ticker, player, got, statuses };
}

describe("FixturePlayer", () => {
  it("sends the snapshot at once, then one message per tick", () => {
    const { ticker, got, statuses } = play();
    expect(got).toEqual([0]);
    expect(statuses).toEqual(["open"]);
    ticker.tick();
    expect(got).toEqual([0, 1]);
    ticker.tick();
    expect(got).toEqual([0, 1, 2]);
  });
  it("closes at the end when not looping", () => {
    const { ticker, got, statuses } = play();
    ticker.tick(5);
    expect(got).toEqual([0, 1, 2]);
    expect(statuses.at(-1)).toBe("closed");
    expect(ticker.fn).toBeNull();
  });
  it("loops back to the snapshot like a reconnect", () => {
    const { ticker, got } = play(true);
    ticker.tick(4);
    expect(got).toEqual([0, 1, 2, 0, 1]);
  });
  it("stop halts delivery", () => {
    const { ticker, player, got } = play();
    player.stop();
    ticker.tick(2);
    expect(got).toEqual([0]);
  });
});

describe("loadFixture", () => {
  it("fetches fixtures/<name>.json and validates messages", async () => {
    const urls: string[] = [];
    const f = (url: string) => { urls.push(url); return Promise.resolve(new Response(JSON.stringify(script))); };
    expect(await loadFixture("duel", f)).toEqual(script);
    expect(urls).toEqual(["/fixtures/duel.json"]);
  });
  it("rejects a missing or malformed fixture", async () => {
    await expect(loadFixture("x", () => Promise.resolve(new Response("", { status: 404 })))).rejects.toThrow(/x/);
    await expect(loadFixture("x", () => Promise.resolve(new Response(JSON.stringify([{ nope: 1 }]))))).rejects.toThrow(/x/);
  });
});
