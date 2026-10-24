import { describe, expect, it } from "vitest";
import { AdminClient } from "./admin";
import { ApiError } from "./rest";

function fake(status: number, json: unknown = {}) {
  const calls: { url: string; init: RequestInit }[] = [];
  const fetchFn = async (url: string, init?: RequestInit) => {
    calls.push({ url, init: init ?? {} });
    return new Response(status === 204 ? null : JSON.stringify(json), { status });
  };
  return { client: new AdminClient(fetchFn), calls };
}

describe("AdminClient", () => {
  it("logs in with a JSON password and the same-origin cookie", async () => {
    const { client, calls } = fake(204);
    await client.login("pw");
    expect(calls[0]!.url).toBe("/api/admin/login");
    expect(calls[0]!.init.method).toBe("POST");
    expect(calls[0]!.init.body).toBe('{"password":"pw"}');
    expect(calls[0]!.init.credentials).toBe("same-origin");
  });
  it("posts kick, min-bots, reset-board and logout", async () => {
    const { client, calls } = fake(204);
    await client.kick("b-1");
    await client.setMinBots(3);
    await client.resetBoard();
    await client.logout();
    expect(calls.map((c) => [c.init.method, c.url, c.init.body])).toEqual([
      ["POST", "/api/admin/kick", '{"id":"b-1"}'],
      ["POST", "/api/admin/min-bots", '{"n":3}'],
      ["POST", "/api/admin/reset-board", undefined],
      ["POST", "/api/admin/logout", undefined],
    ]);
  });
  it("previews a map for a size and applies one", async () => {
    const preview = { seed: 42, map: "###\n#.#\n###\n" };
    const { client, calls } = fake(200, preview);
    expect(await client.previewMap(19, 13)).toEqual(preview);
    expect(await client.previewMap(19, 13, 5)).toEqual(preview);
    expect(await client.previewMap(19, 13, 0, 7)).toEqual(preview);
    await client.applyMap(preview.map);
    expect(calls.map((c) => [c.init.method, c.url, c.init.body])).toEqual([
      ["POST", "/api/admin/map/preview", '{"width":19,"height":13}'],
      ["POST", "/api/admin/map/preview", '{"width":19,"height":13,"walls":5}'],
      ["POST", "/api/admin/map/preview", '{"width":19,"height":13,"walls":0,"seed":7}'],
      ["POST", "/api/admin/map/apply", '{"map":"###\\n#.#\\n###\\n"}'],
    ]);
  });
  it("sets and cancels the timer and starts a game", async () => {
    const { client, calls } = fake(204);
    await client.setTimer(2.5);
    await client.setTimer(0);
    await client.startGame();
    expect(calls.map((c) => [c.init.method, c.url, c.init.body])).toEqual([
      ["POST", "/api/admin/timer", '{"minutes":2.5}'],
      ["POST", "/api/admin/timer", '{"minutes":0}'],
      ["POST", "/api/admin/start", undefined],
    ]);
  });
  it("reads the status", async () => {
    const status = { bots: [], minBots: 5, beat: 7, mapWidth: 19, mapHeight: 13, running: true, remaining: -1, beatMs: 1000 };
    const { client, calls } = fake(200, status);
    expect(await client.status()).toEqual(status);
    expect(calls[0]!.init.method).toBe("GET");
  });
  it("maps failures to ApiError kinds", async () => {
    await expect(fake(401, { error: "wrong password" }).client.login("x")).rejects.toMatchObject({ kind: "unauthorized", status: 401 });
    await expect(fake(429, { error: "slow down" }).client.login("x")).rejects.toMatchObject({ kind: "rate_limited" });
    await expect(fake(404, { error: "no such endpoint" }).client.status()).rejects.toMatchObject({ kind: "http", status: 404 });
  });
  it("reports a network failure", async () => {
    const client = new AdminClient(() => Promise.reject(new Error("down")));
    await expect(client.status()).rejects.toBeInstanceOf(ApiError);
    await expect(client.status()).rejects.toMatchObject({ kind: "network" });
  });
});
