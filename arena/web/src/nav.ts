import { navHref } from "./route";

const LINKS: [string, string][] = [["TV", "/"], ["My bot", "/bot"], ["Leaderboard", "/leaderboard"]];

/** A small fixed link bar to switch between the views (keeps ?fixture). */
export function mountNav(parent: HTMLElement, search: string): void {
  const nav = document.createElement("nav");
  nav.style.cssText = "position:fixed;right:10px;bottom:6px;font:12px monospace;opacity:.7;z-index:10";
  for (const [label, path] of LINKS) {
    const a = document.createElement("a");
    a.href = navHref(path, search);
    a.textContent = label;
    a.style.cssText = "color:#9cf;margin-left:12px";
    nav.append(a);
  }
  parent.append(nav);
}
