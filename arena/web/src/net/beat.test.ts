import { describe, expect, it } from "vitest";
import { DEFAULT_BEAT_MS, MIN_BEAT_MS, parseBeatMs } from "./beat";

describe("parseBeatMs", () => {
  it.each([
    [null, DEFAULT_BEAT_MS],
    ["", DEFAULT_BEAT_MS],
    ["abc", DEFAULT_BEAT_MS],
    ["0", DEFAULT_BEAT_MS],
    ["-5", DEFAULT_BEAT_MS],
    ["10", MIN_BEAT_MS],
    ["250", 250],
  ])("%s -> %s", (raw, want) => expect(parseBeatMs(raw)).toBe(want));
});
