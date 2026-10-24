import { describe, expect, it } from "vitest";
import { overlayOf } from "./overlay";

const e = (bot: string, score: number) => ({ bot, name: bot.toUpperCase(), score, kills: 0, deaths: 0, coins: 0 });
const vm = { remaining: -1, beatMs: 1000, gameOver: false, board: [e("a", 1), e("b", 5)], myBotId: null };

describe("overlayOf", () => {
  it("shows nothing without a timer", () => {
    expect(overlayOf(vm)).toEqual({ clock: null, gameOver: false, rows: [] });
  });
  it("shows the countdown while a timer runs and keeps the board hidden", () => {
    expect(overlayOf({ ...vm, remaining: 125 })).toEqual({ clock: "02:05", gameOver: false, rows: [] });
  });
  it("shows no clock when the beat length is unknown", () => {
    expect(overlayOf({ ...vm, remaining: 125, beatMs: 0 }).clock).toBeNull();
  });
  it("shows game over with the final board, best first, and no clock", () => {
    const o = overlayOf({ ...vm, remaining: 0, gameOver: true });
    expect(o.gameOver).toBe(true);
    expect(o.clock).toBeNull();
    expect(o.rows.map((r) => [r.rank, r.id])).toEqual([[1, "b"], [2, "a"]]);
  });
  it("marks my bot in the final board", () => {
    const o = overlayOf({ ...vm, gameOver: true, myBotId: "a" });
    expect(o.rows.find((r) => r.mine)?.id).toBe("a");
  });
});
