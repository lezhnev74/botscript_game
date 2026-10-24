import { readdirSync, readFileSync } from "node:fs";
import { join } from "node:path";
import { describe, expect, it } from "vitest";

// PLAN section 4: the store depends only on the protocol.
describe("store layering", () => {
  const dir = __dirname;
  const sources = readdirSync(dir).filter((f) => f.endsWith(".ts") && !f.endsWith(".test.ts"));

  it("has sources to check", () => expect(sources.length).toBeGreaterThan(0));

  it.each(sources)("%s imports only the protocol or siblings", (file) => {
    const text = readFileSync(join(dir, file), "utf8");
    const specs = [...text.matchAll(/from\s+["']([^"']+)["']/g)].map((m) => m[1]!);
    for (const spec of specs) {
      expect(spec.startsWith("./") || spec.startsWith("../protocol")).toBe(true);
    }
  });
});
