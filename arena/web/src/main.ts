import "./editor.css";
import { parseBeatMs } from "./net/beat";
import { FixturePlayer, IntervalTicker, loadFixture } from "./net/fixture";
import { AdminClient } from "./net/admin";
import { ApiClient, storageTokens } from "./net/rest";
import type { MessageSource } from "./net/source";
import { WsClient, type SocketLike } from "./net/ws";
import { Canvas2dRenderer } from "./render/canvas2d";
import { TweenRenderer } from "./render/tween";
import { Store } from "./store/store";
import { LANGUAGE_MD } from "./generated/docs";
import { TEMPLATES } from "./generated/templates";
import { mountAddBot } from "./addbot";
import { mountNav } from "./nav";
import { routeOf, type Route } from "./route";
import { mountAdmin } from "./views/admin/admin-dom";
import { mountEditor } from "./views/editor/editor-dom";
import { mountLeaderboard } from "./views/leaderboard/leaderboard-dom";
import { mountTv } from "./views/tv/tv";

/** How long a bot glides from one cell to the next, then how long hit/death effects run; together shorter than a beat. */
const MOVE_MS = 600;
const EFFECT_MS = 350;

// ?fixture=name plays web/fixtures/<name>.json instead of the live WebSocket.
const fixtureUrls = import.meta.glob<string>("../fixtures/*.json", { query: "?url", import: "default", eager: true });
const urlOf = (name: string) => fixtureUrls[`../fixtures/${name}.json`] ?? `/fixtures/${name}.json`;

async function makeSource(params: URLSearchParams): Promise<MessageSource> {
  const name = params.get("fixture");
  if (name) {
    const messages = await loadFixture(name, (u) => fetch(u), urlOf);
    return new FixturePlayer(messages, new IntervalTicker(parseBeatMs(params.get("beat"))), { loop: true });
  }
  const proto = location.protocol === "https:" ? "wss:" : "ws:";
  return new WsClient({ url: `${proto}//${location.host}/ws`, createSocket: (u) => new WebSocket(u) as unknown as SocketLike });
}

function mountView(route: Route, root: HTMLElement, store: Store): void {
  const tokens = storageTokens(localStorage);
  switch (route) {
    case "editor": {
      const api = new ApiClient((u, init) => fetch(u, init), tokens);
      const session = { has: () => tokens.get() !== null, clear: () => tokens.clear() };
      const baseUrl = () => api.config().then((c) => c.baseUrl, () => "");
      mountEditor(root, store, { api, session, templates: TEMPLATES, docs: { language: LANGUAGE_MD }, baseUrl });
      return;
    }
    case "admin":
      return; // mounted by main() before the store is wired
    case "leaderboard":
      mountLeaderboard(root, store, () => new ApiClient((u, init) => fetch(u, init), tokens).lastBoard());
      return;
    default:
      mountTv(root, store, (canvas) => new TweenRenderer(new Canvas2dRenderer(canvas), MOVE_MS, EFFECT_MS));
  }
}

async function main(): Promise<void> {
  const root = document.getElementById("app") as HTMLElement;
  const store = new Store();
  store.setMyBot(storageTokens(localStorage).get()?.botId ?? null);
  if (routeOf(location.pathname) === "admin") {
    // The admin page needs no WebSocket, nav or "Add my bot" button.
    mountAdmin(root, new AdminClient((u, init) => fetch(u, init)));
    return;
  }
  mountView(routeOf(location.pathname), root, store);
  mountNav(document.body, location.search);
  const route = routeOf(location.pathname);
  if (route !== "editor") {
    const tokens = storageTokens(localStorage);
    const api = new ApiClient((u, init) => fetch(u, init), tokens);
    mountAddBot(document.body, { search: location.search, tokens, checkBot: () => api.getBot(), onDropped: () => store.setMyBot(null) });
  }
  const source = await makeSource(new URLSearchParams(location.search));
  source.start({ onMessage: (m) => store.dispatch(m), onStatus: () => undefined });
}

main().catch((e: unknown) => {
  const out = document.getElementById("app");
  if (out) out.textContent = String(e);
});
