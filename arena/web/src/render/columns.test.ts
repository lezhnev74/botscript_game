import { describe, expect, it } from "vitest";
import { ICON_COLUMNS, LEADERBOARD_COLUMNS, TV_COLUMNS, isIconColumn } from "./columns";

describe("score table columns", () => {
  it("TV sidebar: name, HP, kills, deaths, coins, score - no state", () => {
    expect(TV_COLUMNS.map(([c]) => c)).toEqual(["name", "HP", "kills", "deaths", "coins", "score"]);
  });
  it("leaderboard: rank, name, score, kills, deaths, coins", () => {
    expect([...LEADERBOARD_COLUMNS]).toEqual(["#", "name", "score", "kills", "deaths", "coins"]);
  });
  it("kills, deaths, coins are icon columns; the rest are text", () => {
    expect([...ICON_COLUMNS]).toEqual(["kills", "deaths", "coins"]);
    const all = [...TV_COLUMNS.map(([c]) => c), ...LEADERBOARD_COLUMNS];
    expect([...new Set(all.filter(isIconColumn))].sort()).toEqual(["coins", "deaths", "kills"]);
    expect(all.filter((c) => !isIconColumn(c)).sort()).toEqual(["#", "HP", "name", "name", "score", "score"]);
  });
});
