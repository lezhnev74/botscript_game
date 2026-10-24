// @vitest-environment jsdom
import { describe, expect, it } from "vitest";
import { createArenaCard } from "./arena-dom";
import type { ArenaBot } from "./arena-controller";

const card = (bot: ArenaBot) => {
  const c = createArenaCard({ getBot: async () => bot, leaveArena: async () => undefined, returnToArena: async () => undefined }, { onUnauthorized: () => undefined });
  document.body.replaceChildren(c.element);
  return c;
};
const q = (id: string): HTMLElement => document.getElementById(id)!;

describe("arena card reboot", () => {
  it("shows Rebooting with a progress bar and the seconds left", async () => {
    const c = card({ arena: { in: true, leftBy: "", stayLeftBeats: -1 }, beatMs: 1000, rebootLeftBeats: 4, rebootTotalBeats: 10 });
    await c.controller.load();
    expect(q("arena-title").textContent).toBe("Rebooting...");
    expect(q("reboot-text").textContent).toBe("Your bot is off the map and returns in 4 s");
    expect(q("reboot-bar").hidden).toBe(false);
    expect(q("reboot-fill").style.width).toBe("60%");
  });

  it("says returning for a bot still off the map after its countdown", async () => {
    const c = card({ arena: { in: true, leftBy: "", stayLeftBeats: -1 }, beatMs: 1000, rebootLeftBeats: 0, rebootTotalBeats: 10 });
    await c.controller.load();
    expect(q("arena-title").textContent).toBe("Rebooting...");
    expect(q("reboot-text").textContent).toBe("Your bot is returning...");
  });

  it("is the plain in-arena card when the bot is not rebooting", async () => {
    const c = card({ arena: { in: true, leftBy: "", stayLeftBeats: -1 }, beatMs: 1000, rebootLeftBeats: -1 });
    await c.controller.load();
    expect(q("arena-title").textContent).toBe("Your bot is in the arena");
    expect(q("reboot-bar").hidden).toBe(true);
    expect(q("reboot-text").hidden).toBe(true);
  });
});
