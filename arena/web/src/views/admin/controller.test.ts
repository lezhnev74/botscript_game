import { describe, expect, it } from "vitest";
import { AdminController, type AdminStatus, parseCount, parseMinutes, parseSize, rowsOf, type AdminApi, type AdminModel, type Failure } from "./controller";

const status: AdminStatus = {
  bots: [
    { id: "house-1", name: "WobbleBot1", house: true, score: 4 },
    { id: "b-1", name: "Ann", house: false, score: 12 },
    { id: "house-2", name: "WobbleBot2", house: true, score: 0 },
    { id: "b-2", name: "Bob", house: false, score: 7 },
  ],
  minBots: 5,
  beat: 42,
  mapWidth: 19,
  mapHeight: 13,
  running: true,
  remaining: -1,
  beatMs: 1000,
};

/** A walled room of the size (the preview the fake server returns). */
const roomOf = (w: number, h: number): string =>
  Array.from({ length: h }, (_, y) => (y === 0 || y === h - 1 ? "#".repeat(w) : `#${".".repeat(w - 2)}#`)).join("\n") + "\n";

const fail = (kind: string, status: number, message = kind): Failure => ({ kind, status, message });

function rig(overrides: Partial<AdminApi> = {}) {
  const calls: string[] = [];
  const api: AdminApi = {
    login: async (pw) => void calls.push(`login ${pw}`),
    logout: async () => void calls.push("logout"),
    status: async () => (calls.push("status"), status),
    kick: async (id) => void calls.push(`kick ${id}`),
    resetBoard: async () => void calls.push("reset"),
    setMinBots: async (n) => void calls.push(`min ${n}`),
    previewMap: async (w, h, walls) => (calls.push(`preview ${w}x${h} ${walls}%`), { seed: calls.length, map: roomOf(w, h) }),
    applyMap: async (map) => void calls.push(`apply ${map.split("\n").length - 1} rows`),
    setTimer: async (m) => void calls.push(`timer ${m}`),
    startGame: async () => void calls.push("start"),
    ...overrides,
  };
  const models: AdminModel[] = [];
  const c = new AdminController({ api, onChange: (m) => models.push(m) });
  return { c, calls, models };
}

describe("parseCount", () => {
  it("accepts whole numbers, 0 included, with spaces", () => {
    expect(parseCount("0")).toBe(0);
    expect(parseCount(" 12 ")).toBe(12);
  });
  it("rejects everything else", () => {
    for (const bad of ["", "-1", "1.5", "x", "1e3", "12345"]) expect(parseCount(bad)).toBeNull();
  });
});

describe("parseMinutes", () => {
  it("accepts positive numbers with up to two decimals", () => {
    expect(parseMinutes("10")).toBe(10);
    expect(parseMinutes(" 0.5 ")).toBe(0.5);
    expect(parseMinutes("1440")).toBe(1440);
  });
  it("rejects zero, negatives, junk and more than a day", () => {
    for (const bad of ["", "0", "0.00", "-1", "x", "1e3", "1.234", "1441", "1,5"]) expect(parseMinutes(bad)).toBeNull();
  });
});

describe("parseSize", () => {
  it("accepts sizes inside the limits", () => {
    expect(parseSize("19", "13")).toEqual({ width: 19, height: 13 });
    expect(parseSize(" 9 ", "7")).toEqual({ width: 9, height: 7 });
    expect(parseSize("41", "31")).toEqual({ width: 41, height: 31 });
  });
  it("rejects junk and sizes outside 9..41 by 7..31", () => {
    for (const [w, h] of [["8", "13"], ["42", "13"], ["19", "6"], ["19", "32"], ["x", "13"], ["19", ""], ["1.5", "9"], ["-9", "9"]]) {
      expect(parseSize(w!, h!)).toBeNull();
    }
  });
});

describe("rowsOf", () => {
  it("lists humans first and lets only them be kicked", () => {
    const rows = rowsOf(status);
    expect(rows.map((r) => r.id)).toEqual(["b-1", "b-2", "house-1", "house-2"]);
    expect(rows.map((r) => r.canKick)).toEqual([true, true, false, false]);
    expect(rows[0]).toEqual({ id: "b-1", name: "Ann", kind: "human", score: 12, canKick: true });
    expect(rows[2]!.kind).toBe("house");
  });
});

describe("AdminController", () => {
  it("shows the panel when the status answers", async () => {
    const { c } = rig();
    await c.refresh();
    expect(c.state).toMatchObject({ phase: "panel", minBots: 5, beat: 42 });
    expect(c.state.rows).toHaveLength(4);
  });
  it("shows the login form on 401", async () => {
    const { c } = rig({ status: async () => Promise.reject(fail("unauthorized", 401)) });
    await c.refresh();
    expect(c.state.phase).toBe("login");
  });
  it("says the admin is disabled on a first 404", async () => {
    const { c } = rig({ status: async () => Promise.reject(fail("http", 404)) });
    await c.refresh();
    expect(c.state.phase).toBe("disabled");
  });
  it("keeps the panel and shows the message on a later network error", async () => {
    let down = false;
    const { c } = rig({ status: async () => (down ? Promise.reject(fail("network", 0, "offline")) : status) });
    await c.refresh();
    down = true;
    await c.refresh();
    expect(c.state).toMatchObject({ phase: "panel", message: "offline" });
    expect(c.state.rows).toHaveLength(4);
  });
  it("logs in, then loads the panel", async () => {
    const { c, calls } = rig();
    await c.login("pw");
    expect(calls).toEqual(["login pw", "status"]);
    expect(c.state.phase).toBe("panel");
  });
  it("explains a wrong password and a rate limit, and stays on the form", async () => {
    const wrong = rig({ login: async () => Promise.reject(fail("unauthorized", 401)) });
    await wrong.c.login("x");
    expect(wrong.c.state).toMatchObject({ phase: "login", message: "Wrong password." });
    const limited = rig({ login: async () => Promise.reject(fail("rate_limited", 429)) });
    await limited.c.login("x");
    expect(limited.c.state.message).toMatch(/wait a minute/);
    const odd = rig({ login: async () => Promise.reject(new Error("boom")) });
    await odd.c.login("x");
    expect(odd.c.state.message).toBe("unexpected error");
  });
  it("shows the server message for other login failures", async () => {
    const { c } = rig({ login: async () => Promise.reject(fail("network", 0, "offline")) });
    await c.login("x");
    expect(c.state).toMatchObject({ phase: "login", message: "offline" });
  });
  it("kicks, then refreshes the table", async () => {
    const { c, calls } = rig();
    await c.kick("b-1");
    expect(calls).toEqual(["kick b-1", "status"]);
  });
  it("resets the board, then refreshes", async () => {
    const { c, calls } = rig();
    await c.resetBoard();
    expect(calls).toEqual(["reset", "status"]);
  });
  it("sets the house-bot count from valid input only", async () => {
    const { c, calls } = rig();
    await c.setMinBots("x");
    expect(calls).toEqual([]);
    expect(c.state.message).toMatch(/whole number/);
    await c.setMinBots("3");
    expect(calls).toEqual(["min 3", "status"]);
    expect(c.state.message).toBe("");
  });
  it("shows a command failure and does not refresh", async () => {
    const { c, calls } = rig({ kick: async () => Promise.reject(fail("http", 409, "house bots cannot be kicked")) });
    await c.kick("house-1");
    expect(c.state.message).toBe("house bots cannot be kicked");
    expect(calls).toEqual([]);
  });
  it("returns to the login form when the session is lost mid-command", async () => {
    const { c } = rig({ resetBoard: async () => Promise.reject(fail("unauthorized", 401)) });
    await c.resetBoard();
    expect(c.state.phase).toBe("login");
  });
  it("copes with a non-API error", async () => {
    const { c } = rig({ status: async () => Promise.reject(new Error("x")) });
    await c.refresh();
    expect(c.state.message).toBe("unexpected error");
  });
  it("logs out to the login form even when the call fails", async () => {
    const { c } = rig({ logout: async () => Promise.reject(fail("network", 0)) });
    await c.refresh();
    await c.logout();
    expect(c.state).toMatchObject({ phase: "login", rows: [] });
  });

  describe("map", () => {
    it("starts from the size of the map in play and no preview", async () => {
      const { c } = rig();
      await c.refresh();
      expect(c.state).toMatchObject({ mapWidth: 19, mapHeight: 13, sizeW: "19", sizeH: "13", preview: null });
    });
    it("keeps a size the admin typed across refreshes", async () => {
      const { c } = rig();
      await c.refresh();
      c.editSize("25", "15");
      await c.refresh();
      expect(c.state).toMatchObject({ sizeW: "25", sizeH: "15" });
    });
    it("does not re-render while the size is typed", async () => {
      const { c, models } = rig();
      await c.refresh();
      const n = models.length;
      c.editSize("25", "15");
      expect(models.length).toBe(n);
    });
    it("generates a preview for the typed size", async () => {
      const { c, calls } = rig();
      await c.generate("11", "9");
      expect(calls).toEqual(["preview 11x9 3%"]);
      expect(c.state.preview).toMatchObject({ width: 11, height: 9, seed: 1 });
      expect(c.state.preview!.rows).toHaveLength(9);
      expect(c.state).toMatchObject({ sizeW: "11", sizeH: "9", message: "" });
    });
    it("generates with the typed wall percent and regenerates with it", async () => {
      const { c, calls } = rig();
      await c.generate("11", "9", "0");
      await c.regenerate();
      expect(calls).toEqual(["preview 11x9 0%", "preview 11x9 0%"]);
      expect(c.state.preview).toMatchObject({ walls: 0 });
      expect(c.state.walls).toBe("0");
    });
    it("keeps the typed wall percent across refreshes", async () => {
      const { c } = rig();
      await c.refresh();
      c.editSize("19", "13", "10");
      await c.refresh();
      expect(c.state.walls).toBe("10");
    });
    it("refuses a bad wall percent without calling the server", async () => {
      for (const walls of ["16", "-1", "x", ""]) {
        const { c, calls } = rig();
        await c.generate("11", "9", walls);
        expect(calls).toEqual([]);
        expect(c.state.message).toMatch(/0 to 15/);
      }
    });
    it("refuses a bad size without calling the server", async () => {
      const { c, calls } = rig();
      await c.generate("5", "9");
      expect(calls).toEqual([]);
      expect(c.state.message).toMatch(/9 to 41.*7 to 31/);
    });
    it("regenerates with the same size and a new seed", async () => {
      const { c, calls } = rig();
      await c.generate("11", "9");
      const first = c.state.preview!.seed;
      await c.regenerate();
      expect(calls).toEqual(["preview 11x9 3%", "preview 11x9 3%"]);
      expect(c.state.preview!.seed).not.toBe(first);
    });
    it("regenerate before any preview asks for a size first", async () => {
      const { c, calls } = rig();
      await c.regenerate();
      expect(calls).toEqual([]);
      expect(c.state.message).toMatch(/Generate/);
    });
    it("applies the previewed map, then refreshes and keeps no stale preview", async () => {
      const { c, calls } = rig();
      await c.generate("11", "9");
      await c.applyMap();
      expect(calls).toEqual(["preview 11x9 3%", "apply 9 rows", "status"]);
      expect(c.state.preview).toBeNull();
      expect(c.state.message).toMatch(/next beat/);
    });
    it("keeps the applied size in the inputs although the status still shows the old map", async () => {
      const { c } = rig();
      await c.refresh();
      await c.generate("11", "9");
      await c.applyMap();
      expect([c.state.sizeW, c.state.sizeH]).toEqual(["11", "9"]);
    });
    it("applies nothing without a preview", async () => {
      const { c, calls } = rig();
      await c.applyMap();
      expect(calls).toEqual([]);
      expect(c.state.message).toMatch(/Generate/);
    });
    it("shows why a map was refused and keeps the preview", async () => {
      const { c } = rig({ applyMap: async () => Promise.reject(fail("http", 409, "the map has fewer floor cells than there are bots")) });
      await c.generate("11", "9");
      await c.applyMap();
      expect(c.state.message).toMatch(/fewer floor cells/);
      expect(c.state.preview).not.toBeNull();
    });
    it("shows a failed preview and goes back to login on a lost session", async () => {
      const bad = rig({ previewMap: async () => Promise.reject(fail("http", 400, "map size too big")) });
      await bad.c.generate("11", "9");
      expect(bad.c.state.message).toBe("map size too big");
      const lost = rig({ previewMap: async () => Promise.reject(fail("unauthorized", 401)) });
      await lost.c.generate("11", "9");
      expect(lost.c.state.phase).toBe("login");
    });
  });

  describe("game timer", () => {
    const over: AdminStatus = { ...status, running: false, remaining: 0 };

    it("shows the clock of a running timer and that the game runs", async () => {
      const { c } = rig({ status: async () => ({ ...status, remaining: 125 }) });
      await c.refresh();
      expect(c.state).toMatchObject({ running: true, clock: "02:05" });
    });
    it("has no clock without a timer", async () => {
      const { c } = rig();
      await c.refresh();
      expect(c.state).toMatchObject({ running: true, clock: null });
    });
    it("shows a finished game as not running", async () => {
      const { c } = rig({ status: async () => over });
      await c.refresh();
      expect(c.state).toMatchObject({ running: false, clock: "00:00" });
    });
    it("sets the timer from valid minutes only, then refreshes", async () => {
      const { c, calls } = rig();
      await c.setTimer("x");
      expect(calls).toEqual([]);
      expect(c.state.message).toMatch(/minutes/);
      await c.setTimer("2.5");
      expect(calls).toEqual(["timer 2.5", "status"]);
    });
    it("cancels the timer with 0 minutes", async () => {
      const { c, calls } = rig();
      await c.cancelTimer();
      expect(calls).toEqual(["timer 0", "status"]);
    });
    it("starts a new game, then refreshes", async () => {
      const { c, calls } = rig();
      await c.startGame();
      expect(calls).toEqual(["start", "status"]);
    });
    it("shows why the server refused", async () => {
      const { c, calls } = rig({ startGame: async () => Promise.reject(fail("http", 409, "the game is running")) });
      await c.startGame();
      expect(c.state.message).toBe("the game is running");
      expect(calls).toEqual([]);
    });
  });
});
