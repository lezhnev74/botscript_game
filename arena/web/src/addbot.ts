import { ApiError } from "./net/rest";
import { navHref } from "./route";

export interface AddBotState {
  /** Playing a ?fixture: there is no live arena to join. */
  fixture: boolean;
  hasToken: boolean;
  /** The server rejected the stored token (GET /api/bot -> 401). */
  rejected: boolean;
}

/** The "Add my bot" button shows only live, and only when this browser has no (valid) bot. */
export function showAddBot(s: AddBotState): boolean {
  return !s.fixture && (!s.hasToken || s.rejected);
}

export interface AddBotDeps {
  search: string;
  tokens: { get(): unknown; clear(): void };
  checkBot(): Promise<unknown>;
  /** Called when a stored token turned out to be rejected and was dropped. */
  onDropped(): void;
}

/** Whether the token is rejected; any other failure (offline, 5xx) keeps the token and counts as "has a bot". */
async function isRejected(deps: AddBotDeps): Promise<boolean> {
  try {
    await deps.checkBot();
    return false;
  } catch (e) {
    return e instanceof ApiError && e.kind === "unauthorized";
  }
}

/** Mounts the big "Add my bot" link into parent (bottom-right, clear of the arena); it stays hidden when not needed. */
export function mountAddBot(parent: HTMLElement, deps: AddBotDeps): void {
  const fixture = new URLSearchParams(deps.search).get("fixture") !== null;
  const hasToken = deps.tokens.get() !== null;
  const a = document.createElement("a");
  a.id = "add-my-bot";
  a.href = navHref("/bot", deps.search);
  a.textContent = "Add my bot";
  a.style.cssText =
    "position:fixed;right:16px;bottom:40px;z-index:10;padding:16px 28px;font:bold 22px monospace;" +
    "color:#111;background:#bfef45;border-radius:8px;text-decoration:none;box-shadow:0 2px 10px #000a";
  const set = (rejected: boolean): void => {
    a.style.display = showAddBot({ fixture, hasToken, rejected }) ? "" : "none";
  };
  set(false);
  parent.append(a);
  if (fixture || !hasToken) return;
  void isRejected(deps).then((rejected) => {
    if (!rejected) return;
    deps.tokens.clear();
    deps.onDropped();
    set(true);
  });
}
