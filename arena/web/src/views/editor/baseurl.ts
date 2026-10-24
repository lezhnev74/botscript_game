import { resolveBaseUrl } from "./prompt";

/**
 * Starts loading the configured base URL now and returns a synchronous getter.
 * Until the load succeeds (or if it fails) the getter answers the page origin,
 * so a click handler can build text without a network wait inside the user gesture.
 */
export function cacheBaseUrl(load: () => Promise<string>, origin: string): () => string {
  let current = resolveBaseUrl("", origin);
  load().then(
    (configured) => { current = resolveBaseUrl(configured, origin); },
    () => undefined,
  );
  return () => current;
}
