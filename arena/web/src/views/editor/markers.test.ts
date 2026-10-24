import { describe, expect, it } from "vitest";
import { countLines, formatError, markLines } from "./markers";

const err = (line: number, col = 1, msg = "bad", hint = "fix it") => ({ line, col, msg, hint });

describe("countLines", () => {
  it("counts lines, empty text has one", () => {
    expect(countLines("")).toBe(1);
    expect(countLines("a\nb")).toBe(2);
    expect(countLines("a\nb\n")).toBe(3);
  });
});

describe("markLines", () => {
  it("groups errors by line, ordered by line then column", () => {
    const marks = markLines([err(3, 9), err(1), err(3, 2)], 5);
    expect(marks.map((m) => [m.line, m.errors.map((x) => x.col)])).toEqual([[1, [1]], [3, [2, 9]]]);
  });
  it("clamps lines outside the text into range", () => {
    const marks = markLines([err(0), err(99)], 4);
    expect(marks.map((m) => m.line)).toEqual([1, 4]);
  });
  it("is empty without errors", () => {
    expect(markLines([], 3)).toEqual([]);
  });
});

describe("formatError", () => {
  it("shows position, message and hint", () => {
    expect(formatError(err(2, 5, "let is not allowed", "use num[0]"))).toBe("L2:5 let is not allowed - use num[0]");
  });
  it("omits an empty hint", () => {
    expect(formatError(err(2, 5, "oops", ""))).toBe("L2:5 oops");
  });
});
