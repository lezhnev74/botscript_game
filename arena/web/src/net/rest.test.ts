import { describe, expect, it } from "vitest";
import { ApiClient, ApiError, memoryTokens, type FetchLike } from "./rest";

interface Call { url: string; init: RequestInit }

function fakeFetch(status: number, body: unknown, calls: Call[] = []): FetchLike {
  return (url, init) => {
    calls.push({ url, init: init ?? {} });
    const text = body === undefined ? "" : typeof body === "string" ? body : JSON.stringify(body);
    return Promise.resolve(new Response(status === 204 ? null : text, { status }));
  };
}

function client(status: number, body: unknown, token: string | null = "tok") {
  const calls: Call[] = [];
  const tokens = memoryTokens();
  if (token) tokens.set({ token, botId: "bot1" });
  return { api: new ApiClient(fakeFetch(status, body, calls), tokens), calls, tokens };
}

const headerOf = (c: Call, name: string) => new Headers(c.init.headers).get(name);

describe("join", () => {
  it("posts the name and keeps token and bot id", async () => {
    const { api, calls, tokens } = client(200, { token: "T", botId: "b7" }, null);
    expect(await api.join("Ann")).toEqual({ token: "T", botId: "b7" });
    expect(calls[0]!.url).toBe("/api/join");
    expect(calls[0]!.init.method).toBe("POST");
    expect(JSON.parse(calls[0]!.init.body as string)).toEqual({ name: "Ann" });
    expect(tokens.get()).toEqual({ token: "T", botId: "b7" });
  });
  it("409 when the arena is full", async () => {
    const { api } = client(409, { error: "arena is full" }, null);
    await expect(api.join("Ann")).rejects.toMatchObject({ kind: "full", status: 409, message: "arena is full" });
  });
});

describe("authenticated calls", () => {
  it("send the bearer token", async () => {
    const { api, calls } = client(200, { id: "bot1", name: "Ann", source: "x" });
    expect(await api.getBot()).toEqual({ id: "bot1", name: "Ann", source: "x" });
    expect(calls[0]!.url).toBe("/api/bot");
    expect(headerOf(calls[0]!, "Authorization")).toBe("Bearer tok");
  });
  it("putBot sends the raw source and resolves on 204", async () => {
    const { api, calls } = client(204, undefined);
    await api.putBot("function loop() {}");
    expect(calls[0]!.init.method).toBe("PUT");
    expect(calls[0]!.init.body).toBe("function loop() {}");
  });
  it("putProfile sends the name and look as JSON and resolves on 204", async () => {
    const { api, calls } = client(204, undefined);
    const look = { color: "#112233", body: "ghost", eyes: "x", mouth: "o", hat: "bow" };
    await api.putProfile("Ann", look);
    expect(calls[0]!.url).toBe("/api/bot/profile");
    expect(calls[0]!.init.method).toBe("PUT");
    expect(JSON.parse(calls[0]!.init.body as string)).toEqual({ name: "Ann", look });
    expect(headerOf(calls[0]!, "Content-Type")).toBe("application/json");
    expect(headerOf(calls[0]!, "Authorization")).toBe("Bearer tok");
  });
  it("leaveArena and returnToArena POST with the token and resolve on 204", async () => {
    const { api, calls } = client(204, undefined);
    await api.leaveArena();
    await api.returnToArena();
    expect(calls.map((c) => [c.url, c.init.method])).toEqual([["/api/bot/leave", "POST"], ["/api/bot/return", "POST"]]);
    expect(headerOf(calls[0]!, "Authorization")).toBe("Bearer tok");
  });
  it("returnToArena on a full arena fails with kind full and the server's words", async () => {
    const { api } = client(409, { error: "arena is full" });
    await expect(api.returnToArena()).rejects.toMatchObject({ kind: "full", message: "arena is full" });
  });
  it("leaderboard parses JSON", async () => {
    const l = client(200, [{ bot: "a", name: "A", score: 1, kills: 0, deaths: 0, coins: 1 }], null);
    expect(await l.api.leaderboard()).toHaveLength(1);
    expect(l.calls[0]!.url).toBe("/api/leaderboard");
    expect(headerOf(l.calls[0]!, "Authorization")).toBeNull();
  });
  it("reads the last game board without a token", async () => {
    const l = client(200, [{ bot: "a", name: "A", score: 9, kills: 1, deaths: 0, coins: 0 }], null);
    expect((await l.api.lastBoard())[0]!.score).toBe(9);
    expect(l.calls[0]!.url).toBe("/api/leaderboard/last");
    expect(headerOf(l.calls[0]!, "Authorization")).toBeNull();
  });
  it("fail locally with unauthorized when there is no token", async () => {
    const { api, calls } = client(200, {}, null);
    await expect(api.getBot()).rejects.toMatchObject({ kind: "unauthorized" });
    expect(calls).toHaveLength(0);
  });
});

describe("typed errors", () => {
  it("401 unauthorized", async () => {
    const { api } = client(401, { error: "join first" });
    await expect(api.getBot()).rejects.toMatchObject({ kind: "unauthorized", status: 401, message: "join first" });
  });
  it("403 forbidden", async () => {
    const { api } = client(403, { error: "that is not your bot" });
    await expect(api.getBot()).rejects.toMatchObject({ kind: "forbidden" });
  });
  it("422 validation carries line, col, msg and hint", async () => {
    const errors = [{ line: 2, col: 5, msg: "no let", hint: "use num[i]" }];
    const { api } = client(422, { errors });
    const err = await api.putBot("let x").catch((e: unknown) => e);
    expect(err).toBeInstanceOf(ApiError);
    expect(err).toMatchObject({ kind: "validation", status: 422, errors });
  });
  it("429 carries retryBeats", async () => {
    const { api } = client(429, { error: "slow down", retryBeats: 7 });
    await expect(api.putBot("x")).rejects.toMatchObject({ kind: "rate_limited", retryBeats: 7 });
  });
  it("413 too large and other statuses", async () => {
    await expect(client(413, { error: "big" }).api.putBot("x")).rejects.toMatchObject({ kind: "too_large" });
    await expect(client(500, { error: "internal error" }).api.getBot()).rejects.toMatchObject({ kind: "http", status: 500 });
  });
  it("survives a non-JSON error body", async () => {
    const { api } = client(502, "<html>bad gateway</html>");
    await expect(api.getBot()).rejects.toMatchObject({ kind: "http", status: 502 });
  });
  it("a network failure is a network error", async () => {
    const api = new ApiClient(() => Promise.reject(new TypeError("down")), memoryTokens());
    await expect(api.leaderboard()).rejects.toMatchObject({ kind: "network" });
  });
});

describe("config", () => {
  it("reads the base URL without a token", async () => {
    const { api, calls } = client(200, { baseUrl: "https://a.example" }, null);
    expect(await api.config()).toEqual({ baseUrl: "https://a.example" });
    expect(calls[0]!.url).toBe("/api/config");
    expect(headerOf(calls[0]!, "Authorization")).toBeNull();
  });
});
