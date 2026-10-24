// @vitest-environment jsdom
import { describe, expect, it } from "vitest";
import { renderSidebar } from "./sidebar-dom";
import type { SidebarRow } from "./sidebar";

const row = (id: string, color: string): SidebarRow => ({ id, name: id, color, hp: 1, maxHp: 4, kills: 0, deaths: 0, score: 0, coins: 0, mine: false, rebooting: false });

describe("renderSidebar names", () => {
  it("lifts a near-black name color and keeps a bright one", () => {
    const table = document.createElement("table");
    renderSidebar(table, [row("dark", "#000000"), row("bright", "#ff8040")]);
    const [dark, bright] = [...table.tBodies[0]!.rows].map((r) => r.cells[0]!.style.color);
    expect(dark).not.toBe("rgb(0, 0, 0)");
    expect(dark).not.toBe("");
    expect(bright).toBe("rgb(255, 128, 64)");
  });
});

describe("renderSidebar rebooting", () => {
  it("labels a rebooting bot in its row", () => {
    const table = document.createElement("table");
    renderSidebar(table, [{ ...row("A", "#ff8040"), rebooting: true }, row("B", "#ff8040")]);
    const names = [...table.tBodies[0]!.rows].map((r) => r.cells[0]!.textContent);
    expect(names).toEqual(["A (rebooting)", "B"]);
  });
});
