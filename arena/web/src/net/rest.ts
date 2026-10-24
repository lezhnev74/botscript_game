import type { BotBody, ConfigBody, JoinResponse, LeaderboardEntry, Look, ProfileRequest, SourceError } from "../protocol/protocol";

export type ErrorKind =
  | "unauthorized" | "forbidden" | "validation" | "rate_limited" | "full" | "too_large" | "http" | "network";

/** A failed REST call; `kind` is what the UI switches on. */
export class ApiError extends Error {
  constructor(
    readonly kind: ErrorKind,
    readonly status: number,
    message: string,
    readonly errors: SourceError[] = [],
    readonly retryBeats = 0,
  ) {
    super(message);
    this.name = "ApiError";
  }
}

export interface Credentials {
  token: string;
  botId: string;
}

export interface TokenStore {
  get(): Credentials | null;
  set(c: Credentials): void;
  clear(): void;
}

export function memoryTokens(): TokenStore {
  let c: Credentials | null = null;
  return { get: () => c, set: (v) => { c = v; }, clear: () => { c = null; } };
}

const KEY = "arena.credentials";

/** Token store over localStorage (or any Storage). */
export function storageTokens(storage: Pick<Storage, "getItem" | "setItem" | "removeItem">): TokenStore {
  return {
    get() {
      try {
        const v: unknown = JSON.parse(storage.getItem(KEY) ?? "null");
        const c = v as Partial<Credentials> | null;
        return c && typeof c.token === "string" && typeof c.botId === "string" ? { token: c.token, botId: c.botId } : null;
      } catch {
        return null;
      }
    },
    set: (c) => storage.setItem(KEY, JSON.stringify(c)),
    clear: () => storage.removeItem(KEY),
  };
}

export type FetchLike = (url: string, init?: RequestInit) => Promise<Response>;

const KINDS: Record<number, ErrorKind> = {
  401: "unauthorized", 403: "forbidden", 409: "full", 413: "too_large", 422: "validation", 429: "rate_limited",
};

export async function toError(res: Response): Promise<ApiError> {
  const body = (await res.json().catch(() => ({}))) as { error?: string; errors?: SourceError[]; retryBeats?: number };
  const kind = KINDS[res.status] ?? "http";
  const message = body.error ?? `HTTP ${res.status}`;
  return new ApiError(kind, res.status, message, body.errors ?? [], body.retryBeats ?? 0);
}

export class ApiClient {
  constructor(
    private readonly fetchFn: FetchLike,
    private readonly tokens: TokenStore,
  ) {}

  async join(name: string): Promise<JoinResponse> {
    const res = await this.call("POST", "/api/join", { body: JSON.stringify({ name }), type: "application/json" });
    const out = (await res.json()) as JoinResponse;
    this.tokens.set({ token: out.token, botId: out.botId });
    return out;
  }

  async getBot(): Promise<BotBody> {
    return (await this.authed("GET", "/api/bot")).json() as Promise<BotBody>;
  }

  async putBot(source: string): Promise<void> {
    await this.authed("PUT", "/api/bot", { body: source, type: "text/plain" });
  }

  /** Renames my bot and sets its look (PUT /api/bot/profile). */
  async putProfile(name: string, look: Look): Promise<void> {
    const body: ProfileRequest = { name, look };
    await this.authed("PUT", "/api/bot/profile", { body: JSON.stringify(body), type: "application/json" });
  }

  /** Takes my bot out of the arena; it stays registered (POST /api/bot/leave). */
  async leaveArena(): Promise<void> {
    await this.authed("POST", "/api/bot/leave");
  }

  /** Brings my detached bot back into the arena (POST /api/bot/return). */
  async returnToArena(): Promise<void> {
    await this.authed("POST", "/api/bot/return");
  }

  async leaderboard(): Promise<LeaderboardEntry[]> {
    return (await this.call("GET", "/api/leaderboard")).json() as Promise<LeaderboardEntry[]>;
  }

  /** The final board of the latest finished game (empty before any). */
  async lastBoard(): Promise<LeaderboardEntry[]> {
    return (await this.call("GET", "/api/leaderboard/last")).json() as Promise<LeaderboardEntry[]>;
  }

  /** The server's configuration (the public base URL, "" when not set). */
  async config(): Promise<ConfigBody> {
    return (await this.call("GET", "/api/config")).json() as Promise<ConfigBody>;
  }

  private authed(method: string, url: string, req: Req = {}): Promise<Response> {
    const c = this.tokens.get();
    if (!c) return Promise.reject(new ApiError("unauthorized", 401, "join first"));
    return this.call(method, url, { ...req, token: c.token });
  }

  private async call(method: string, url: string, req: Req = {}): Promise<Response> {
    const headers = new Headers();
    if (req.token) headers.set("Authorization", `Bearer ${req.token}`);
    if (req.type) headers.set("Content-Type", req.type);
    let res: Response;
    try {
      res = await this.fetchFn(url, { method, headers, body: req.body });
    } catch (e) {
      throw new ApiError("network", 0, e instanceof Error ? e.message : "network error");
    }
    if (!res.ok) throw await toError(res);
    return res;
  }
}

interface Req {
  body?: string;
  type?: string;
  token?: string;
}
