import { describe, expect, it } from "vitest";
import { faceOf } from "./face";

const look = { eyes: "dots", mouth: "smile" };
const rest = { eyes: "dots", mouth: "smile" };

describe("faceOf", () => {
  it("shows the resting face without reactions", () => {
    expect(faceOf(look, { dead: false }, 10)).toEqual(rest);
  });
  it("winces for one beat after a hit", () => {
    expect(faceOf(look, { dead: false, hitBeat: 10 }, 10)).toEqual({ eyes: "squint", mouth: "o" });
    expect(faceOf(look, { dead: false, hitBeat: 10 }, 11)).toEqual(rest);
  });
  it("grins with wide eyes for two beats after a kill", () => {
    const k = { dead: false, killBeat: 10 };
    expect(faceOf(look, k, 10)).toEqual({ eyes: "wide", mouth: "grin" });
    expect(faceOf(look, k, 11)).toEqual({ eyes: "wide", mouth: "grin" });
    expect(faceOf(look, k, 12)).toEqual(rest);
  });
  it("ignores events from the future (a rewound game)", () => {
    expect(faceOf(look, { dead: false, hitBeat: 10, killBeat: 10 }, 5)).toEqual(rest);
  });
  it("precedence: dead > hit > kill", () => {
    expect(faceOf(look, { dead: true, hitBeat: 10, killBeat: 10 }, 10)).toEqual({ eyes: "x", mouth: "flat" });
    expect(faceOf(look, { dead: false, hitBeat: 10, killBeat: 10 }, 10)).toEqual({ eyes: "squint", mouth: "o" });
  });
  it("the respawn shield brings back the resting face", () => {
    expect(faceOf(look, { dead: false, shielded: true, hitBeat: 10, killBeat: 10 }, 10)).toEqual(rest);
  });
});
