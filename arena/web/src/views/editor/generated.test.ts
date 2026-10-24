import { describe, expect, it } from "vitest";
import { LANGUAGE_MD, VISION_MD } from "../../generated/docs";
import { TEMPLATES } from "../../generated/templates";
import { buildPrompt } from "./prompt";

describe("generated templates and docs", () => {
  it("offers the five house bots, each with a loop()", () => {
    expect(TEMPLATES.map((t) => t.name)).toEqual(["Rookie", "Lighthouse", "Tracker", "Forager", "Hound"]);
    TEMPLATES.forEach((t) => expect(t.source).toContain("function loop()"));
  });
  it("builds a prompt from the real docs with the language, scoring rules and every example", () => {
    const text = buildPrompt({ language: LANGUAGE_MD, vision: VISION_MD, examples: TEMPLATES, baseUrl: "https://arena.example.com" });
    expect(text).toContain("# BOTSCRIPT - the bot language");
    expect(text).toContain("## 9. Scoring");
    expect(text).toContain("## 3. World");
    expect(text).not.toContain("## 11. Build plan");
    TEMPLATES.forEach((t) => expect(text).toContain(`### ${t.name}`));
  });
});
