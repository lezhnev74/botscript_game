import { describe, expect, it } from "vitest";
import { buildPrompt, extractSections, isPrivateUrl, resolveBaseUrl } from "./prompt";

const md = "# T\nintro\n## 1. One\nalpha\n## 2. Two\nbeta\n## 9. Nine\ngamma\n";

describe("extractSections", () => {
  it("keeps the numbered sections asked for, in document order", () => {
    expect(extractSections(md, [9, 1])).toBe("## 1. One\nalpha\n\n## 9. Nine\ngamma");
  });
  it("skips sections that do not exist", () => {
    expect(extractSections(md, [5])).toBe("");
  });
});

describe("buildPrompt", () => {
  const docs = { language: md, examples: [{ name: "Rookie", source: "function loop() {}" }], baseUrl: "https://arena.example.com" };
  const text = buildPrompt(docs);
  it("contains language sections 1-8 and the examples as code blocks", () => {
    expect(text).toContain("## 1. One\nalpha");
    expect(text).toContain("## 2. Two\nbeta");
    expect(text).not.toContain("gamma");
    expect(text).not.toContain("## 9.");
    expect(text).toContain("### Rookie\n```js\nfunction loop() {}\n```");
  });
  it("orders role, spec, examples, request", () => {
    const at = ["# Your role", "## 1. One", "### Rookie", "# Check your program", "# Your request"].map((s) => text.indexOf(s));
    expect(at.every((n) => n >= 0)).toBe(true);
    expect(at).toEqual([...at].sort((a, b) => a - b));
  });
  it("ends by asking what to optimise for before any code, then one code block", () => {
    const req = text.slice(text.indexOf("# Your request"));
    expect(req).toMatch(/Before you write any code, ask me what the bot should be optimised for/);
    expect(req).toMatch(/kills, survival, coins/);
    expect(req).toMatch(/single code block/i);
    expect(text.trimEnd().endsWith(req.trimEnd())).toBe(true);
  });
  it("states the 64 KB program limit", () => {
    expect(text).toContain("At most 64 KB.");
    expect(text).not.toContain("16 KB");
  });
  it("has no wish section and no long dashes", () => {
    expect(text).not.toMatch(/What I want/);
    expect(text).not.toMatch(/[—–]/);
  });
});

describe("buildPrompt with sections", () => {
  const lang = "# T\n\nintro line\n\n---\n\n" + [1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13].map((n) => `## ${n}. S${n}\nbody${n}\n`).join("\n");
  const text = buildPrompt({ language: lang, examples: [], baseUrl: "https://a.example" });
  it("takes sections 1-8 and none of 9-13", () => {
    for (let n = 1; n <= 8; n++) expect(text).toContain(`## ${n}. S${n}`);
    for (let n = 9; n <= 13; n++) expect(text).not.toContain(`S${n}`);
  });
  it("has no vision content", () => {
    expect(text).not.toContain("[agreed]");
    expect(text).not.toContain("Status");
    expect(text).not.toMatch(/# Game rules/);
  });
  it("keeps the intro but not the title", () => {
    expect(text).toContain("intro line");
    expect(text).not.toContain("# T\n");
  });
});

describe("check section", () => {
  const section = (baseUrl: string) => {
    const t = buildPrompt({ language: "L", examples: [], baseUrl });
    return t.slice(t.indexOf("# Check your program"), t.indexOf("# Your request"));
  };
  it("gives both URLs with the base URL filled in", () => {
    const s = section("https://arena.example.com");
    expect(s).toContain("POST https://arena.example.com/api/check");
    expect(s).toContain("GET https://arena.example.com/api/check?src=");
    expect(s).toMatch(/raw (request )?body/);
    expect(s).toMatch(/URL-encoded/);
  });
  it("explains both answers", () => {
    const s = section("https://a.example");
    expect(s).toContain('"ok": false');
    expect(s).toContain('"ok": true');
    expect(s).toMatch(/warnings/);
  });
  it("tells the AI to check every program, fix errors and act on warnings before the final code", () => {
    const s = section("https://a.example");
    expect(s).toMatch(/Test the program/);
    expect(s).toMatch(/fix all errors/i);
    expect(s).toMatch(/warnings/);
    expect(s).toMatch(/before (you )?give/);
  });
  it("is public: no private address note", () => {
    expect(section("https://arena.example.com")).not.toMatch(/private address/);
  });
  it("notes that a private address may be unreachable", () => {
    for (const u of ["http://localhost:8080", "http://192.168.1.5:8080", "http://10.0.0.2", "http://172.20.1.1:80"]) {
      const s = section(u);
      expect(s).toMatch(/private address/);
      expect(s).toMatch(/by hand/);
    }
  });
  it("uses short dashes only", () => {
    expect(section("http://localhost:8080")).not.toMatch(/[—–]/);
  });
});

describe("resolveBaseUrl", () => {
  it("prefers the configured URL and trims trailing slashes", () => {
    expect(resolveBaseUrl("https://a.example//", "http://page:1")).toBe("https://a.example");
  });
  it("falls back to the page origin", () => {
    expect(resolveBaseUrl("", "http://page:1")).toBe("http://page:1");
  });
});

describe("isPrivateUrl", () => {
  it.each([
    ["http://localhost:8080", true], ["http://127.0.0.1", true], ["http://10.1.2.3", true], ["http://192.168.0.9", true],
    ["http://172.16.0.1", true], ["http://172.31.255.1", true], ["http://[::1]:8080", true], ["http://host.local", true],
    ["http://172.15.0.1", false], ["http://172.32.0.1", false], ["https://arena.example.com", false], ["http://8.8.8.8", false],
    ["http://tvbox:8080", true], ["http://169.254.1.2", true], ["http://100.64.0.1", true], ["http://100.127.9.9", true],
    ["http://[fd12:3456::1]", true], ["http://[fc00::1]", true],
    ["http://100.63.0.1", false], ["http://100.128.0.1", false], ["http://[2001:db8::1]", false], ["https://example.com", false],
    ["http://[::ffff:7f00:1]", true], ["http://[::ffff:127.0.0.1]", true], ["http://[::ffff:c0a8:1]", true], ["http://[::ffff:808:808]", false],
    ["not a url", false],
  ])("%s -> %s", (url, want) => {
    expect(isPrivateUrl(url)).toBe(want);
  });
});
