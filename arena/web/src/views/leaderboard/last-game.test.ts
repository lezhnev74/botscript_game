import { describe, expect, it } from "vitest";
import { lastGameLoader, type BoardEntry } from "./last-game";

const row = (bot: string): BoardEntry => ({ bot, name: bot, score: 1, kills: 0, deaths: 0, coins: 0 });
const flush = (): Promise<void> => new Promise((r) => setTimeout(r, 0));

function rig(loads: (BoardEntry[] | Error)[]) {
  const shown: BoardEntry[][] = [];
  let calls = 0;
  const feed = lastGameLoader(
    () => {
      const v = loads[Math.min(calls++, loads.length - 1)]!;
      return v instanceof Error ? Promise.reject(v) : Promise.resolve(v);
    },
    (rows) => shown.push(rows),
  );
  return { feed, shown, calls: () => calls };
}

describe("lastGameLoader", () => {
  it("loads once at the start", async () => {
    const r = rig([[row("a")]]);
    await flush();
    expect(r.shown).toEqual([[row("a")]]);
  });
  it("does not reload while the game runs", async () => {
    const r = rig([[row("a")]]);
    r.feed({ gameOver: false });
    r.feed({ gameOver: false });
    expect(r.calls()).toBe(1);
  });
  it("reloads once when the game ends, not on every beat of the stopped game", async () => {
    const r = rig([[row("a")], [row("b")]]);
    r.feed({ gameOver: false });
    r.feed({ gameOver: true });
    r.feed({ gameOver: true });
    await flush();
    expect(r.calls()).toBe(2);
    expect(r.shown.at(-1)).toEqual([row("b")]);
  });
  it("reloads again at the next game over", async () => {
    const r = rig([[], [], []]);
    r.feed({ gameOver: true });
    r.feed({ gameOver: false });
    r.feed({ gameOver: true });
    expect(r.calls()).toBe(3);
  });
  it("keeps the old rows when a load fails", async () => {
    const r = rig([[row("a")], new Error("down")]);
    await flush();
    r.feed({ gameOver: true });
    await flush();
    expect(r.shown).toEqual([[row("a")]]);
  });
});
