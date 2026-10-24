import { describe, expect, it } from "vitest";
import { BODIES, EYES, HATS, MOUTHS } from "../../render/shapes";
import { LookController, PREVIEW_STEPS, fieldOf, previewFace, tileLook, tileValues, type LookModel, type LookValue } from "./look-controller";
import type { Failure } from "./controller";

class Fail extends Error implements Failure {
  constructor(readonly kind: string, message = kind, readonly retryBeats?: number) { super(message); }
}

const LOOK: LookValue = { color: "#336699", body: "circle", eyes: "dots", mouth: "smile", hat: "none" };

function setup(put: (name: string, look: LookValue) => Promise<void> = async () => undefined, rand: () => number = Math.random) {
  const models: LookModel[] = [];
  const saved: [string, LookValue][] = [];
  let lost = 0;
  const ctl = new LookController({
    api: { putProfile: put },
    onChange: (m) => models.push(m),
    onSaved: (n, l) => saved.push([n, l]),
    onUnauthorized: () => { lost++; },
    random: rand,
  });
  ctl.load("Ann", LOOK);
  return { ctl, saved, last: () => models[models.length - 1]!, lost: () => lost };
}

describe("tiles", () => {
  it("lists the values from the render lists", () => {
    expect(tileValues("body")).toEqual([...BODIES]);
    expect(tileValues("eyes")).toEqual([...EYES]);
    expect(tileValues("mouth")).toEqual([...MOUTHS]);
    expect(tileValues("hat")).toEqual([...HATS]);
  });
  it("a tile shows the current look with one part replaced", () => {
    expect(tileLook(LOOK, "hat", "crown")).toEqual({ ...LOOK, hat: "crown" });
  });
});

describe("dirty and edits", () => {
  it("is clean after load and dirty after any edit; reverting makes it clean again", () => {
    const { ctl, last } = setup();
    expect(last().dirty).toBe(false);
    expect(last().canSave).toBe(false);
    ctl.setPart("eyes", "hearts");
    expect(last().look.eyes).toBe("hearts");
    expect(last().dirty).toBe(true);
    expect(last().canSave).toBe(true);
    ctl.setPart("eyes", "dots");
    expect(last().dirty).toBe(false);
    ctl.setColor("#ff0000");
    expect(last().dirty).toBe(true);
    ctl.setColor("#336699");
    ctl.setName("Bob");
    expect(last().dirty).toBe(true);
    ctl.setName("  Ann ");
    expect(last().dirty).toBe(false);
  });
  it("randomize changes something and stays valid", () => {
    for (const r of [() => 0, () => 0.999999, Math.random, () => 0.5]) {
      const { ctl, last } = setup(undefined, r);
      ctl.randomize();
      const l = last().look;
      expect(l).not.toEqual(LOOK);
      expect(l.color).toMatch(/^#[0-9a-f]{6}$/);
      expect(BODIES).toContain(l.body);
      expect(EYES).toContain(l.eyes);
      expect(MOUTHS).toContain(l.mouth);
      expect(HATS).toContain(l.hat);
      expect(last().canSave).toBe(true);
    }
  });
  it("randomize bumps the color when the roll equals the current look", () => {
    const { ctl, last } = setup(undefined, () => 0);
    ctl.load("Ann", { color: "#000000", body: BODIES[0], eyes: EYES[0], mouth: MOUTHS[0], hat: HATS[0] });
    ctl.randomize();
    expect(last().look.color).toBe("#000001");
  });
});

describe("save", () => {
  it("sends the trimmed name and look, then is clean with a success state", async () => {
    const sent: [string, LookValue][] = [];
    const { ctl, last, saved } = setup(async (n, l) => { sent.push([n, l]); });
    ctl.setName(" Bob ");
    ctl.setPart("hat", "crown");
    const p = ctl.save();
    expect(last().save.kind).toBe("saving");
    expect(last().canSave).toBe(false);
    await p;
    expect(sent).toEqual([["Bob", { ...LOOK, hat: "crown" }]]);
    expect(last().save.kind).toBe("saved");
    expect(last().dirty).toBe(false);
    expect(last().name).toBe("Bob");
    expect(saved).toEqual([["Bob", { ...LOOK, hat: "crown" }]]);
    ctl.setPart("hat", "bow");
    expect(last().save.kind).toBe("idle");
  });
  it("does nothing when clean", async () => {
    let n = 0;
    const { ctl } = setup(async () => { n++; });
    await ctl.save();
    expect(n).toBe(0);
  });
  it("rejects an empty name locally", async () => {
    let n = 0;
    const { ctl, last } = setup(async () => { n++; });
    ctl.setName("  ");
    await ctl.save();
    expect(n).toBe(0);
    expect(last().field).toEqual({ field: "name", message: "Enter a name." });
  });
  it("shows a 400 field error next to the right control and keeps it dirty", async () => {
    const { ctl, last } = setup(async () => { throw new Fail("http", "bad look: body"); });
    ctl.setPart("body", "square");
    await ctl.save();
    expect(last().field).toEqual({ field: "body", message: "bad look: body" });
    expect(last().save.kind).toBe("idle");
    expect(last().dirty).toBe(true);
    ctl.setPart("body", "ghost");
    expect(last().field).toBeNull();
  });
  it("maps a bad name answer to the name field", async () => {
    const { ctl, last } = setup(async () => { throw new Fail("http", "name must be 1 to 24 characters"); });
    ctl.setName("x");
    await ctl.save();
    expect(last().field?.field).toBe("name");
  });
  it("reports a rate limit, a failure, and a lost session", async () => {
    let e: unknown = new Fail("rate_limited", "slow", 3);
    const { ctl, last, lost } = setup(async () => { throw e; });
    ctl.setName("Bob");
    await ctl.save();
    expect(last().save).toEqual({ kind: "wait", beats: 3 });
    e = new Fail("network", "offline");
    await ctl.save();
    expect(last().save).toEqual({ kind: "failed", message: "offline" });
    e = new Fail("unauthorized");
    await ctl.save();
    expect(lost()).toBe(1);
  });
});

describe("fieldOf", () => {
  it("reads the field from the server message", () => {
    expect(fieldOf("bad look: eyes")).toBe("eyes");
    expect(fieldOf("name must be 1 to 24 characters")).toBe("name");
    expect(fieldOf("boom")).toBeNull();
  });
});

describe("previewFace", () => {
  it("cycles resting, hit, kill, dead", () => {
    expect(PREVIEW_STEPS).toBe(4);
    expect(previewFace(LOOK, 0)).toEqual({ eyes: "dots", mouth: "smile" });
    expect(previewFace(LOOK, 1)).toEqual({ eyes: "squint", mouth: "o" });
    expect(previewFace(LOOK, 2)).toEqual({ eyes: "wide", mouth: "grin" });
    expect(previewFace(LOOK, 3)).toEqual({ eyes: "x", mouth: "flat" });
    expect(previewFace(LOOK, 4)).toEqual(previewFace(LOOK, 0));
  });
});

describe("edits during a save", () => {
  const pending = () => {
    let done!: () => void;
    const put = () => new Promise<void>((r) => { done = r; });
    return { put, finish: () => done() };
  };
  it("a tile edit made while saving stays dirty and is kept", async () => {
    const { put, finish } = pending();
    const { ctl, last, saved } = setup(put);
    ctl.setName("Bob");
    const p = ctl.save();
    ctl.setPart("hat", "crown");
    finish();
    await p;
    expect(last().dirty).toBe(true);
    expect(last().savedName).toBe("Bob");
    expect(last().savedLook).toEqual(LOOK);
    expect(last().look.hat).toBe("crown");
    expect(saved).toEqual([["Bob", LOOK]]);
  });
  it("a name typed while saving is kept and stays dirty", async () => {
    const { put, finish } = pending();
    const { ctl, last, saved } = setup(put);
    ctl.setName("Bob");
    const p = ctl.save();
    ctl.setName("Carl");
    finish();
    await p;
    expect(last().name).toBe("Carl");
    expect(last().savedName).toBe("Bob");
    expect(last().dirty).toBe(true);
    expect(saved).toEqual([["Bob", LOOK]]);
  });
});
