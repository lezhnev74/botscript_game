// @vitest-environment jsdom
import { describe, expect, it } from "vitest";
import { mountAddBot, showAddBot } from "./addbot";
import { ApiError } from "./net/rest";

describe("showAddBot", () => {
  it.each([
    [{ fixture: false, hasToken: false, rejected: false }, true],
    [{ fixture: false, hasToken: true, rejected: false }, false],
    [{ fixture: false, hasToken: true, rejected: true }, true],
    [{ fixture: true, hasToken: false, rejected: false }, false],
    [{ fixture: true, hasToken: true, rejected: true }, false],
  ])("%j -> %s", (s, want) => {
    expect(showAddBot(s)).toBe(want);
  });
});

function mount(search: string, token: boolean, check: () => Promise<unknown>) {
  document.body.replaceChildren();
  const calls = { cleared: 0, dropped: 0 };
  const tokens = { get: () => (token ? { token: "t" } : null), clear: () => void calls.cleared++ };
  mountAddBot(document.body, { search, tokens, checkBot: check, onDropped: () => void calls.dropped++ });
  return { a: document.getElementById("add-my-bot") as HTMLAnchorElement, calls };
}
const flush = () => new Promise((r) => setTimeout(r, 0));

describe("mountAddBot", () => {
  it("shows a link to /bot without a token", () => {
    const { a } = mount("", false, () => Promise.reject(new Error("unused")));
    expect(a.style.display).toBe("");
    expect(a.getAttribute("href")).toBe("/bot");
  });
  it("is hidden in fixture mode", () => {
    const { a } = mount("?fixture=x", false, () => Promise.resolve());
    expect(a.style.display).toBe("none");
  });
  it("stays hidden while the token is accepted or the check fails for other reasons", async () => {
    const ok = mount("", true, () => Promise.resolve({}));
    const net = mount("", true, () => Promise.reject(new ApiError("network", 0, "x")));
    await flush();
    expect(ok.a.style.display).toBe("none");
    expect(net.a.style.display).toBe("none");
    expect(ok.calls.cleared + net.calls.cleared).toBe(0);
  });
  it("drops a rejected token and shows the button", async () => {
    const { a, calls } = mount("", true, () => Promise.reject(new ApiError("unauthorized", 401, "no")));
    expect(a.style.display).toBe("none");
    await flush();
    expect(a.style.display).toBe("");
    expect(calls).toEqual({ cleared: 1, dropped: 1 });
  });
});
