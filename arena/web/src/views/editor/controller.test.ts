import { describe, expect, it } from "vitest";
import { EditorController, canSave, isDirty, outcomeOf, rebootSeconds, type EditorApi, type EditorModel, type Failure } from "./controller";

class Fail extends Error implements Failure {
  constructor(readonly kind: string, message = kind, readonly errors?: Failure["errors"], readonly retryBeats?: number) {
    super(message);
  }
}

function setup(over: Partial<EditorApi> = {}, hasToken = true) {
  const calls: string[] = [];
  const api: EditorApi = {
    join: async (n) => { calls.push(`join:${n}`); return { botId: "me" }; },
    getBot: async () => ({ name: "Ann", source: "function loop() {}", look: { color: "#112233", body: "circle", eyes: "dots", mouth: "smile", hat: "none" } }),
    putBot: async (s) => { calls.push(`put:${s}`); },
    ...over,
  };
  let token = hasToken;
  const models: EditorModel[] = [];
  const joined: (string | null)[] = [];
  const ctl = new EditorController({
    api,
    session: { has: () => token, clear: () => { token = false; calls.push("clear"); } },
    onJoined: (id) => { joined.push(id); token = id !== null; },
    onChange: (m) => models.push(m),
  });
  return { ctl, calls, models, joined, last: () => models[models.length - 1]! };
}

describe("outcomeOf", () => {
  it("maps API failures to UI outcomes", () => {
    expect(outcomeOf(new Fail("unauthorized"))).toEqual({ type: "relogin" });
    expect(outcomeOf(new Fail("full", "arena is full"))).toEqual({ type: "full", message: "arena is full" });
    expect(outcomeOf(new Fail("validation", "v", [{ line: 1, col: 1, msg: "m", hint: "h" }]))).toEqual({
      type: "errors", errors: [{ line: 1, col: 1, msg: "m", hint: "h" }],
    });
    expect(outcomeOf(new Fail("rate_limited", "slow", undefined, 3))).toEqual({ type: "wait", beats: 3 });
    expect(outcomeOf(new Fail("network", "offline"))).toEqual({ type: "failed", message: "offline" });
    expect(outcomeOf("weird")).toEqual({ type: "failed", message: "unexpected error" });
  });
});

describe("rebootSeconds", () => {
  it("rounds the beats up to seconds and is 0 without a reboot or a beat length", () => {
    expect([rebootSeconds(10, 1000), rebootSeconds(3, 300), rebootSeconds(0, 1000), rebootSeconds(-1, 1000), rebootSeconds(5, 0)]).toEqual([10, 1, 0, 0, 0]);
  });
});

describe("start", () => {
  it("asks for a name when there is no token", async () => {
    const { ctl, last } = setup({}, false);
    await ctl.start();
    expect(last().phase).toBe("need-name");
  });
  it("loads my bot when a token exists and reports it as mine", async () => {
    const { ctl, last, joined } = setup();
    await ctl.start();
    expect(last()).toMatchObject({ phase: "ready", botName: "Ann", source: "function loop() {}" });
    expect(joined).toEqual([]);
    expect(last().look).toEqual({ color: "#112233", body: "circle", eyes: "dots", mouth: "smile", hat: "none" });
  });
  it("renamed updates the title name only", async () => {
    const { ctl, last } = setup();
    await ctl.start();
    const look = last().look;
    ctl.renamed("Bob");
    expect(last().botName).toBe("Bob");
    expect(last().look).toBe(look);
  });
  it("expire (a lost token seen elsewhere) drops the token and asks for a name", async () => {
    const { ctl, last, calls, joined } = setup();
    await ctl.start();
    ctl.expire();
    expect(calls).toContain("clear");
    expect(joined).toEqual([null]);
    expect(last().phase).toBe("need-name");
  });
  it("401 on load clears the token and asks for a name again", async () => {
    const { ctl, last, calls, joined } = setup({ getBot: async () => { throw new Fail("unauthorized"); } });
    await ctl.start();
    expect(calls).toContain("clear");
    expect(joined).toEqual([null]);
    expect(last().phase).toBe("need-name");
    expect(last().message).toMatch(/new bot/i);
  });
});

describe("offline", () => {
  const down = async () => { throw new Fail("network", "offline"); };
  it("keeps the token and shows offline (not the join form) on a transient load failure", async () => {
    const { ctl, last, calls, joined } = setup({ getBot: down });
    await ctl.start();
    expect(last().phase).toBe("offline");
    expect(last().message).toMatch(/retrying/);
    expect(calls).not.toContain("clear");
    expect(joined).toEqual([]);
  });
  it("retries on the beat poll and on retry(), and recovers", async () => {
    let up = false;
    const { ctl, last } = setup({ getBot: async () => { if (!up) throw new Fail("http", "503"); return { name: "Ann", source: "s", look: { color: "#112233", body: "circle", eyes: "dots", mouth: "smile", hat: "none" } }; } });
    await ctl.start();
    await ctl.poll();
    expect(last().phase).toBe("offline");
    up = true;
    await ctl.retry();
    expect(last()).toMatchObject({ phase: "ready", botName: "Ann" });
  });
});

describe("full retry", () => {
  it("retry() joins again with the same name", async () => {
    let full = true;
    const { ctl, last, calls } = setup({ join: async (n) => { if (full) throw new Fail("full", "arena is full"); return { botId: n }; } }, false);
    await ctl.join("Ann");
    expect(last().phase).toBe("full");
    full = false;
    await ctl.retry();
    expect(last().phase).toBe("ready");
    expect(calls.filter((c) => c === "clear")).toEqual([]);
  });
});

describe("join", () => {
  it("joins with the name, marks the bot mine and loads the template", async () => {
    const { ctl, last, calls, joined } = setup({}, false);
    await ctl.join("  Ann ");
    expect(calls).toContain("join:Ann");
    expect(joined).toEqual(["me"]);
    expect(last().phase).toBe("ready");
  });
  it("rejects an empty name without a call", async () => {
    const { ctl, last, calls } = setup({}, false);
    await ctl.join("   ");
    expect(calls).toEqual([]);
    expect(last().phase).toBe("need-name");
    expect(last().message).toMatch(/name/i);
  });
  it("409 full shows the arena is full", async () => {
    const { ctl, last } = setup({ join: async () => { throw new Fail("full", "arena is full"); } }, false);
    await ctl.join("Ann");
    expect(last()).toMatchObject({ phase: "full", message: "arena is full" });
  });
  it("other failures return to the name prompt with the message", async () => {
    const { ctl, last } = setup({ join: async () => { throw new Fail("network", "offline"); } }, false);
    await ctl.join("Ann");
    expect(last()).toMatchObject({ phase: "need-name", message: "offline" });
  });
});

describe("save", () => {
  it("PUTs the source and reports saved", async () => {
    const { ctl, last, calls } = setup();
    await ctl.start();
    await ctl.save("function loop() { wait(); }");
    expect(calls).toContain("put:function loop() { wait(); }");
    expect(last().save).toEqual({ kind: "saved" });
    expect(last().source).toBe("function loop() { wait(); }");
  });
  it("says the bot is rebooting, and for how long, when the server reports a reboot", async () => {
    const bot = { name: "Ann", source: "x", look: { color: "#112233", body: "circle", eyes: "dots", mouth: "smile", hat: "none" } };
    let left = -1;
    const { ctl, last } = setup({ getBot: async () => ({ ...bot, rebootLeftBeats: left, beatMs: 1500 }) });
    await ctl.start();
    left = 7;
    await ctl.save("function loop() { wait(); }");
    expect(last().save).toEqual({ kind: "saved", rebootSeconds: 11 }); // 7 beats of 1.5 s, rounded up
    left = -1;
    await ctl.save("function loop() { wait(); wait(); }");
    expect(last().save).toEqual({ kind: "saved" });
  });
  it("is plain saved when the reboot state cannot be read", async () => {
    let fail = false;
    const { ctl, last } = setup({ getBot: async () => { if (fail) throw new Fail("network", "offline"); return { name: "Ann", source: "x", look: { color: "#112233", body: "circle", eyes: "dots", mouth: "smile", hat: "none" } }; } });
    await ctl.start();
    fail = true;
    await ctl.save("function loop() { wait(); }");
    expect(last().save).toEqual({ kind: "saved" });
  });
  it("422 keeps the text and exposes the errors", async () => {
    const errors = [{ line: 2, col: 3, msg: "m", hint: "h" }];
    const { ctl, last } = setup({ putBot: async () => { throw new Fail("validation", "v", errors); } });
    await ctl.start();
    await ctl.save("bad");
    expect(last().save).toEqual({ kind: "errors", errors });
    expect(last().source).toBe("bad");
  });
  it("429 shows the retry beats", async () => {
    const { ctl, last } = setup({ putBot: async () => { throw new Fail("rate_limited", "x", undefined, 2); } });
    await ctl.start();
    await ctl.save("x");
    expect(last().save).toEqual({ kind: "wait", beats: 2 });
  });
  it("401 clears the token and returns to the name prompt", async () => {
    const { ctl, last, calls, joined } = setup({ putBot: async () => { throw new Fail("unauthorized"); } });
    await ctl.start();
    await ctl.save("x");
    expect(calls).toContain("clear");
    expect(joined).toEqual([null]);
    expect(last().phase).toBe("need-name");
  });
  it("strips an AI answer's code fence before saving", async () => {
    const { ctl, last, calls } = setup();
    await ctl.start();
    await ctl.save("Sure!\n```js\nfunction loop() { wait(); }\n```");
    expect(calls).toContain("put:function loop() { wait(); }\n");
    expect(last().source).toBe("function loop() { wait(); }\n");
  });
  it("keeps the draft through a 401: restored and saved after the rejoin", async () => {
    let stale = true;
    const { ctl, last, calls } = setup({ putBot: async (s) => { if (stale) { stale = false; throw new Fail("unauthorized"); } calls.push(`put:${s}`); } });
    await ctl.start();
    await ctl.save("my draft");
    expect(last().phase).toBe("need-name");
    expect(last().source).toBe("my draft");
    await ctl.join("Ann");
    expect(last()).toMatchObject({ phase: "ready", source: "my draft", save: { kind: "saved" } });
    expect(calls).toContain("put:my draft");
  });
  it("shows saving while the request is open", async () => {
    let release: () => void = () => undefined;
    const { ctl, models } = setup({ putBot: () => new Promise<void>((r) => { release = r; }) });
    await ctl.start();
    const p = ctl.save("x");
    expect(models[models.length - 1]!.save).toEqual({ kind: "saving" });
    release();
    await p;
  });
});

describe("dirty and save state", () => {
  it("is clean after load: Save disabled", async () => {
    const { ctl, last } = setup();
    await ctl.start();
    expect(isDirty(last())).toBe(false);
    expect(canSave(last())).toBe(false);
  });
  it("typing makes it dirty and enables Save; typing back makes it clean", async () => {
    const { ctl, last } = setup();
    await ctl.start();
    ctl.edit("function loop() { wait(); }");
    expect(isDirty(last())).toBe(true);
    expect(canSave(last())).toBe(true);
    ctl.edit("function loop() {}");
    expect(isDirty(last())).toBe(false);
    expect(canSave(last())).toBe(false);
  });
  it("Save is disabled while saving", async () => {
    let release: () => void = () => undefined;
    const { ctl, last } = setup({ putBot: () => new Promise<void>((r) => { release = r; }) });
    await ctl.start();
    ctl.edit("x");
    const p = ctl.save("x");
    expect(canSave(last())).toBe(false);
    release();
    await p;
  });
  it("a successful save updates the reference: clean, saved message state", async () => {
    const { ctl, last } = setup();
    await ctl.start();
    ctl.edit("function loop() { wait(); }");
    await ctl.save("function loop() { wait(); }");
    expect(last().save).toEqual({ kind: "saved" });
    expect(last().savedSource).toBe("function loop() { wait(); }");
    expect(isDirty(last())).toBe(false);
    expect(canSave(last())).toBe(false);
  });
  it("editing after a save clears the success state and makes it dirty again", async () => {
    const { ctl, last } = setup();
    await ctl.start();
    await ctl.save("a");
    ctl.edit("ab");
    expect(last().save).toEqual({ kind: "idle" });
    expect(isDirty(last())).toBe(true);
  });
  it("a failed save keeps the box dirty and the reference unchanged", async () => {
    const { ctl, last } = setup({ putBot: async () => { throw new Fail("validation", "v", []); } });
    await ctl.start();
    await ctl.save("bad");
    expect(last().savedSource).toBe("function loop() {}");
    expect(canSave(last())).toBe(true);
  });
  it("loading a template into the box keeps the saved reference", async () => {
    const { ctl, last } = setup();
    await ctl.start();
    ctl.edit("template source");
    expect(last().savedSource).toBe("function loop() {}");
    expect(isDirty(last())).toBe(true);
  });
});

describe("edit keeps unresolved save results", () => {
  it("keeps validation errors while typing", async () => {
    const errs = [{ line: 2, col: 1, msg: "bad", hint: "" }];
    const { ctl, last } = setup({ putBot: async () => { throw new Fail("validation", "v", errs); } });
    await ctl.start();
    await ctl.save("bad");
    ctl.edit("bad2");
    expect(last().save).toEqual({ kind: "errors", errors: errs });
  });
  it("keeps wait and failed results while typing", async () => {
    const w = setup({ putBot: async () => { throw new Fail("rate_limited", "r", undefined, 3); } });
    await w.ctl.start();
    await w.ctl.save("a");
    w.ctl.edit("ab");
    expect(w.last().save).toEqual({ kind: "wait", beats: 3 });
    const f = setup({ putBot: async () => { throw new Fail("boom", "nope"); } });
    await f.ctl.start();
    await f.ctl.save("a");
    f.ctl.edit("ab");
    expect(f.last().save).toEqual({ kind: "failed", message: "nope" });
  });
});

describe("loadPreset", () => {
  it("asks when dirty and loads on yes", async () => {
    const { ctl, last } = setup();
    await ctl.start();
    ctl.edit("mine");
    const asked: string[] = [];
    ctl.loadPreset("preset", (q) => { asked.push(q); return true; });
    expect(asked).toHaveLength(1);
    expect(last().source).toBe("preset");
  });
  it("leaves the box alone when the confirm is declined", async () => {
    const { ctl, last } = setup();
    await ctl.start();
    ctl.edit("mine");
    ctl.loadPreset("preset", () => false);
    expect(last().source).toBe("mine");
  });
  it("does not ask when clean", async () => {
    const { ctl, last } = setup();
    await ctl.start();
    let asked = 0;
    ctl.loadPreset("preset", () => { asked++; return false; });
    expect(asked).toBe(0);
    expect(last().source).toBe("preset");
  });
  it("does nothing before the editor is ready", () => {
    const { ctl, models } = setup();
    ctl.loadPreset("preset", () => true);
    expect(models).toHaveLength(0);
  });
});

