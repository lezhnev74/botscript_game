import { describe, expect, it } from "vitest";
import { LANGUAGE_MD } from "../../generated/docs";
import { TEMPLATES } from "../../generated/templates";
import { buildPrompt } from "./prompt";

describe("generated templates and docs", () => {
  it("offers the five house bots, each with a loop()", () => {
    expect(TEMPLATES.map((t) => t.name)).toEqual(["Rookie", "Lighthouse", "Tracker", "Forager", "Hound"]);
    TEMPLATES.forEach((t) => expect(t.source).toContain("function loop()"));
  });
  const build = (baseUrl: string) => buildPrompt({ language: LANGUAGE_MD, examples: TEMPLATES, baseUrl });
  it("builds a prompt from the real docs: sections 1-8, no vision, every example once", () => {
    const text = build("https://arena.example.com");
    for (const h of ["## 1. World rules", "## 3. Execution", "## 8. Allowed syntax"]) expect(text).toContain(h);
    for (const h of ["## 9. Bots", "## 10. Phone", "## 11. TV", "## 12. Implementation", "## 13. Open"]) expect(text).not.toContain(h);
    expect(text).not.toContain("[agreed]");
    expect(text).not.toMatch(/HP 3/);
    expect(text).toContain("kill +3");
    TEMPLATES.forEach((t) => {
      expect(text).toContain(`### ${t.name}`);
      expect(text.split(t.source.trimEnd()).length - 1).toBe(1);
    });
  });
  it("keeps the interrupt, missed-beat, heard and kill wording", () => {
    const text = build("https://arena.example.com");
    ["A hit beats a bump", "No handler = no reaction", "its code continues", "reset on respawn", "grazing a wall corner still sees", "not per cell walked",
      "kill +3, or +1 for a repeat kill of the same victim"].forEach((w) => expect(text).toContain(w));
  });
  it("does not repeat that -1 is a wait", () => {
    expect(build("https://arena.example.com")).not.toContain("is safe: -1");
  });
  it("stays within 14000 characters", () => {
    expect(build("https://arena.example.com").length).toBeLessThanOrEqual(14000);
  });
});
