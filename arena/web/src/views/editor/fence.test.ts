import { describe, expect, it } from "vitest";
import { stripFence } from "./fence";

describe("stripFence", () => {
  it("leaves plain code alone", () => {
    expect(stripFence("function loop() {}\n")).toBe("function loop() {}\n");
  });
  it("takes the content of a fenced block and drops text around it", () => {
    expect(stripFence("Here you go:\n```js\nfunction loop() {}\n```\nEnjoy!")).toBe("function loop() {}\n");
  });
  it("handles a fence without a language and without a closing fence", () => {
    expect(stripFence("```\nwait();\n")).toBe("wait();\n");
  });
  it("uses only the first block", () => {
    expect(stripFence("```js\na\n```\n```js\nb\n```")).toBe("a\n");
  });
  it("handles CRLF", () => {
    expect(stripFence("```js\r\nwait();\r\n```")).toBe("wait();\n");
  });
});
