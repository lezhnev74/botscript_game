import { describe, expect, it } from "vitest";
import { navHref, routeOf } from "./route";

describe("routeOf", () => {
  it("maps paths to views", () => {
    expect(routeOf("/")).toBe("tv");
    expect(routeOf("/bot")).toBe("editor");
    expect(routeOf("/leaderboard")).toBe("leaderboard");
    expect(routeOf("/admin")).toBe("admin");
  });
  it("ignores a trailing slash and falls back to the TV", () => {
    expect(routeOf("/bot/")).toBe("editor");
    expect(routeOf("/nope")).toBe("tv");
  });
});

describe("navHref", () => {
  it("keeps the fixture query", () => {
    expect(navHref("/bot", "?fixture=duel_5x5&beat=100")).toBe("/bot?fixture=duel_5x5&beat=100");
  });
  it("drops unrelated params", () => {
    expect(navHref("/", "?x=1")).toBe("/");
  });
});
