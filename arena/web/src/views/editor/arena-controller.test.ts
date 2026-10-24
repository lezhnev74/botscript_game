import { describe, expect, it } from "vitest";
import { ArenaController, formatStay, type ArenaBot, type ArenaModel } from "./arena-controller";
import type { Failure } from "./controller";

class Fail extends Error implements Failure {
  constructor(readonly kind: string, message = kind, readonly retryBeats?: number) { super(message); }
}

const IN: ArenaBot = { arena: { in: true, leftBy: "", stayLeftBeats: 86399 }, beatMs: 1000 };
const OUT: ArenaBot = { arena: { in: false, leftBy: "player", stayLeftBeats: -1 }, beatMs: 1000 };

function setup(bot: ArenaBot = IN, over: { leave?: () => Promise<void>; back?: () => Promise<void>; get?: () => Promise<ArenaBot> } = {}) {
  let cur = bot;
  const calls: string[] = [];
  const models: ArenaModel[] = [];
  let lost = 0;
  const ctl = new ArenaController({
    api: {
      getBot: over.get ?? (async () => cur),
      leaveArena: over.leave ?? (async () => { calls.push("leave"); cur = OUT; }),
      returnToArena: over.back ?? (async () => { calls.push("return"); cur = IN; }),
    },
    onChange: (m) => models.push(m),
    onUnauthorized: () => { lost++; },
  });
  return { ctl, calls, last: () => models[models.length - 1]!, lost: () => lost };
}

describe("formatStay", () => {
  it("shows hours and minutes, rounded down", () => {
    expect(formatStay(86399, 1000)).toBe("23 h 59 min");
    expect(formatStay(86400, 1000)).toBe("24 h");
    expect(formatStay(3600 + 120, 1000)).toBe("1 h 2 min");
    expect(formatStay(600, 1000)).toBe("10 min");
    expect(formatStay(30, 1000)).toBe("less than a minute");
    expect(formatStay(0, 1000)).toBe("less than a minute");
  });
});

describe("load", () => {
  it("shows the stay left of a bot in the arena", async () => {
    const { ctl, last } = setup();
    await ctl.load();
    expect(last()).toMatchObject({ view: "in", stayText: "leaves automatically in 23 h 59 min", busy: false, message: null });
  });
  it("hides the time when there is no limit", async () => {
    const { ctl, last } = setup({ arena: { in: true, leftBy: "", stayLeftBeats: -1 }, beatMs: 1000 });
    await ctl.load();
    expect(last()).toMatchObject({ view: "in", stayText: "" });
  });
  it("says a detached bot is not in the arena", async () => {
    const { ctl, last } = setup(OUT);
    await ctl.load();
    expect(last()).toMatchObject({ view: "out", note: "" });
  });
  it("tells that the stay ran out", async () => {
    const { ctl, last } = setup({ arena: { in: false, leftBy: "time", stayLeftBeats: -1 }, beatMs: 1000 });
    await ctl.load();
    expect(last().note).toBe("It left automatically after 24 hours.");
  });
  it("a lost token goes to the login flow", async () => {
    const { ctl, lost } = setup(IN, { get: async () => { throw new Fail("unauthorized"); } });
    await ctl.load();
    expect(lost()).toBe(1);
  });
  it("an offline load keeps the view and says so", async () => {
    const { ctl, last } = setup(IN, { get: async () => { throw new Fail("network", "offline"); } });
    await ctl.load();
    expect(last().message).toEqual({ tone: "error", text: "offline" });
  });
});

describe("leave", () => {
  it("asks first and does nothing on no", async () => {
    const { ctl, calls } = setup();
    await ctl.load();
    await ctl.leave(() => false);
    expect(calls).toEqual([]);
  });
  it("leaves on yes and refreshes to the detached state", async () => {
    const { ctl, calls, last } = setup();
    await ctl.load();
    await ctl.leave(() => true);
    expect(calls).toEqual(["leave"]);
    expect(last()).toMatchObject({ view: "out", busy: false });
  });
  it("shows how long to wait on 429", async () => {
    const { ctl, last } = setup(IN, { leave: async () => { throw new Fail("rate_limited", "slow", 3); } });
    await ctl.load();
    await ctl.leave(() => true);
    expect(last().view).toBe("in");
    expect(last().message).toEqual({ tone: "error", text: "Too many changes: try again in 3 beat(s)." });
  });
  it("is not callable while a request runs", async () => {
    let n = 0;
    let release: () => void = () => undefined;
    const { ctl } = setup(IN, { leave: () => { n++; return new Promise<void>((r) => { release = r; }); } });
    await ctl.load();
    const first = ctl.leave(() => true);
    await ctl.leave(() => true);
    release();
    await first;
    expect(n).toBe(1);
  });
});

describe("return", () => {
  it("brings the bot back and refreshes", async () => {
    const { ctl, calls, last } = setup(OUT);
    await ctl.load();
    await ctl.returnToArena();
    expect(calls).toEqual(["return"]);
    expect(last()).toMatchObject({ view: "in", message: null });
  });
  it("shows the server's words when the arena is full", async () => {
    const { ctl, last } = setup(OUT, { back: async () => { throw new Fail("full", "arena is full"); } });
    await ctl.load();
    await ctl.returnToArena();
    expect(last().view).toBe("out");
    expect(last().message).toEqual({ tone: "error", text: "arena is full" });
  });
  it("shows the wait on 429 and goes to login on 401", async () => {
    const wait = setup(OUT, { back: async () => { throw new Fail("rate_limited", "slow", 2); } });
    await wait.ctl.load();
    await wait.ctl.returnToArena();
    expect(wait.last().message?.text).toBe("Too many changes: try again in 2 beat(s).");
    const gone = setup(OUT, { back: async () => { throw new Fail("unauthorized"); } });
    await gone.ctl.load();
    await gone.ctl.returnToArena();
    expect(gone.lost()).toBe(1);
  });
});

describe("poll", () => {
  it("reloads every 30 beats", async () => {
    let gets = 0;
    const { ctl } = setup(IN, { get: async () => { gets++; return IN; } });
    await ctl.load();
    for (let i = 0; i < 29; i++) await ctl.poll();
    expect(gets).toBe(1);
    await ctl.poll();
    expect(gets).toBe(2);
  });
});

describe("reboot", () => {
  const rebooting = (left: number): ArenaBot => ({ arena: { in: true, leftBy: "", stayLeftBeats: -1 }, beatMs: 1500, rebootLeftBeats: left, rebootTotalBeats: 10 });

  it("shows the seconds left and how far the reboot is", async () => {
    let bot = rebooting(10);
    const { ctl, last } = setup(IN, { get: async () => bot });
    await ctl.load();
    expect(last().reboot).toEqual({ secondsLeft: 15, progress: 0 });
    bot = rebooting(4);
    await ctl.load();
    expect(last().reboot).toEqual({ secondsLeft: 6, progress: 0.6 });
  });

  it("is null for a bot that is not rebooting, detached, or from a server without reboots", async () => {
    const none = setup(IN);
    await none.ctl.load();
    expect(none.last().reboot).toBeNull();
    const idle = setup(rebooting(-1));
    await idle.ctl.load();
    expect(idle.last().reboot).toBeNull();
    const out = setup({ ...OUT, rebootLeftBeats: 5 });
    await out.ctl.load();
    expect(out.last().reboot).toBeNull();
  });

  it("refreshes on every beat while rebooting, until the bot is back", async () => {
    let bot = rebooting(2);
    let gets = 0;
    const { ctl, last } = setup(IN, { get: async () => { gets++; return bot; } });
    await ctl.load();
    await ctl.poll();
    bot = rebooting(1);
    await ctl.poll();
    expect(gets).toBe(3);
    bot = rebooting(-1);
    await ctl.poll();
    expect(last().reboot).toBeNull();
    await ctl.poll();
    expect(gets).toBe(4); // back to the slow refresh
  });

  it("uses the server's total, so a page opened mid-reboot shows the right fraction", async () => {
    const { ctl, last } = setup(IN, { get: async () => rebooting(4) });
    await ctl.load();
    expect(last().reboot).toEqual({ secondsLeft: 6, progress: 0.6 });
  });

  it("reports a bot still off the map after its countdown as returning, not as in the arena", async () => {
    const { ctl, last } = setup(IN, { get: async () => rebooting(0) });
    await ctl.load();
    expect(last().reboot).toEqual({ secondsLeft: 0, progress: 1 });
  });
});
