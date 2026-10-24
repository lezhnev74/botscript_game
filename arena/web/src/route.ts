export type Route = "tv" | "editor" | "leaderboard" | "admin";

const ROUTES: Record<string, Route> = { "/": "tv", "/bot": "editor", "/leaderboard": "leaderboard", "/admin": "admin" };

/** Maps a pathname to a view; unknown paths show the TV. */
export function routeOf(pathname: string): Route {
  const path = pathname.length > 1 ? pathname.replace(/\/+$/, "") : pathname;
  return ROUTES[path] ?? "tv";
}

/** A nav link that keeps the ?fixture (and ?beat) params of the current page. */
export function navHref(path: string, search: string): string {
  const p = new URLSearchParams(search);
  const keep = new URLSearchParams();
  for (const k of ["fixture", "beat"]) {
    const v = p.get(k);
    if (v !== null) keep.set(k, v);
  }
  const q = keep.toString();
  return q === "" ? path : `${path}?${q}`;
}
