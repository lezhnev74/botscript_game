import { describe, expect, it } from "vitest";
import { buildRows } from "./standings";

const e = (bot: string, score: number, kills = 0, deaths = 0, coins = 0) => ({ bot, name: bot.toUpperCase(), score, kills, deaths, coins });

describe("buildRows", () => {
  it("sorts by score, then kills, then fewer deaths, then name", () => {
    const rows = buildRows({ board: [e("b", 3), e("a", 3), e("c", 9), e("d", 3, 1), e("f", 3, 1, 2)], myBotId: null });
    expect(rows.map((r) => r.id)).toEqual(["c", "d", "f", "a", "b"]);
  });
  it("puts fewer deaths first when score and kills tie, even against name order", () => {
    const rows = buildRows({ board: [e("a", 3, 1, 5), e("z", 3, 1, 0)], myBotId: null });
    expect(rows.map((r) => r.id)).toEqual(["z", "a"]);
  });
  it("does not share a rank when only kills or only deaths differ", () => {
    expect(buildRows({ board: [e("a", 5, 2), e("b", 5, 1)], myBotId: null }).map((r) => r.rank)).toEqual([1, 2]);
    expect(buildRows({ board: [e("a", 5, 1, 0), e("b", 5, 1, 1)], myBotId: null }).map((r) => r.rank)).toEqual([1, 2]);
  });
  it("ranks with ties sharing a rank", () => {
    const rows = buildRows({ board: [e("a", 5, 1), e("b", 5, 1), e("c", 1)], myBotId: null });
    expect(rows.map((r) => r.rank)).toEqual([1, 1, 3]);
  });
  it("carries the stats and marks my bot", () => {
    const [row] = buildRows({ board: [e("a", 4, 1, 2, 3)], myBotId: "a" });
    expect(row).toEqual({ rank: 1, id: "a", name: "A", score: 4, kills: 1, deaths: 2, coins: 3, mine: true });
  });
  it("does not mutate the board and handles an empty one", () => {
    const board = [e("b", 1), e("a", 2)];
    buildRows({ board, myBotId: null });
    expect(board[0]?.bot).toBe("b");
    expect(buildRows({ board: [], myBotId: null })).toEqual([]);
  });
});
