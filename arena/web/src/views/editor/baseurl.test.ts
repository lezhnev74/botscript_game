import { describe, expect, it, vi } from "vitest";
import { cacheBaseUrl } from "./baseurl";

describe("cacheBaseUrl", () => {
  it("loads once at creation and answers synchronously without further calls", async () => {
    let resolve: (v: string) => void = () => undefined;
    const load = vi.fn(() => new Promise<string>((r) => { resolve = r; }));
    const get = cacheBaseUrl(load, "http://page:1");
    expect(load).toHaveBeenCalledTimes(1);
    expect(get()).toBe("http://page:1");
    resolve("https://a.example/");
    await Promise.resolve();
    expect(get()).toBe("https://a.example");
    get(); get();
    expect(load).toHaveBeenCalledTimes(1);
  });
  it("keeps the origin when the load fails or is empty", async () => {
    const failing = cacheBaseUrl(() => Promise.reject(new Error("offline")), "http://page:1");
    const empty = cacheBaseUrl(() => Promise.resolve(""), "http://page:1/");
    await new Promise((r) => setTimeout(r, 0));
    expect(failing()).toBe("http://page:1");
    expect(empty()).toBe("http://page:1");
  });
});
