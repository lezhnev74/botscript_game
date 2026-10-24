import { readdirSync, readFileSync } from "node:fs";
import { join } from "node:path";
import { describe, expect, it } from "vitest";

function importsOf(dir: string): [string, string][] {
  return readdirSync(dir)
    .filter((f) => f.endsWith(".ts") && !f.endsWith(".test.ts"))
    .flatMap((f) => [...readFileSync(join(dir, f), "utf8").matchAll(/from\s+["']([^"']+)["']/g)].map((m): [string, string] => [f, m[1]!]));
}

// PLAN section 4: views depend only on the store and the Renderer; render depends on nothing.
describe("layering", () => {
  it.each(importsOf(__dirname))("views/tv/%s imports only store, render or siblings (%s)", (_f, spec) => {
    expect(/^\.\/|^\.\.\/\.\.\/(store|render)\//.test(spec)).toBe(true);
  });
  it.each(importsOf(join(__dirname, "../../render")))("render/%s imports only siblings (%s)", (_f, spec) => {
    expect(spec.startsWith("./")).toBe(true);
  });
});
