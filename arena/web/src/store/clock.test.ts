import { describe, expect, it } from "vitest";
import { formatRemaining } from "./clock";

describe("formatRemaining", () => {
  it("formats beats left as mm:ss with the beat length", () => {
    expect(formatRemaining(90, 1000)).toBe("01:30");
    expect(formatRemaining(1, 1000)).toBe("00:01");
    expect(formatRemaining(4, 250)).toBe("00:01");
  });
  it("rounds partial seconds up and shows 00:00 only at zero", () => {
    expect(formatRemaining(3, 500)).toBe("00:02");
    expect(formatRemaining(0, 1000)).toBe("00:00");
  });
  it("lets minutes pass 59", () => {
    expect(formatRemaining(5400, 1000)).toBe("90:00");
  });
  it("is null with no timer or an unknown beat length", () => {
    expect(formatRemaining(-1, 1000)).toBeNull();
    expect(formatRemaining(10, 0)).toBeNull();
  });
});
