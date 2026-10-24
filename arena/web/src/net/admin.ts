import type { AdminMapPreview, AdminStatus } from "../protocol/protocol";
import { ApiError, toError, type FetchLike } from "./rest";

/** REST client of the password-protected admin API; the session lives in an HttpOnly cookie the browser sends itself. */
export class AdminClient {
  constructor(private readonly fetchFn: FetchLike) {}

  async login(password: string): Promise<void> {
    await this.post("/api/admin/login", { password });
  }

  async logout(): Promise<void> {
    await this.post("/api/admin/logout");
  }

  async status(): Promise<AdminStatus> {
    return (await this.call("GET", "/api/admin/status")).json() as Promise<AdminStatus>;
  }

  async kick(id: string): Promise<void> {
    await this.post("/api/admin/kick", { id });
  }

  async resetBoard(): Promise<void> {
    await this.post("/api/admin/reset-board");
  }

  async setMinBots(n: number): Promise<void> {
    await this.post("/api/admin/min-bots", { n });
  }

  /** A generated room for the size with walls% interior walls; without walls the server's default, without a seed the server draws one. */
  async previewMap(width: number, height: number, walls?: number, seed?: number): Promise<AdminMapPreview> {
    const body = { width, height, ...(walls === undefined ? {} : { walls }), ...(seed === undefined ? {} : { seed }) };
    return (await this.post("/api/admin/map/preview", body)).json() as Promise<AdminMapPreview>;
  }

  /** Starts a new game on the map at the next beat (scores and players stay). */
  async applyMap(map: string): Promise<void> {
    await this.post("/api/admin/map/apply", { map });
  }

  /** Ends the game after this many minutes; 0 cancels the timer. */
  async setTimer(minutes: number): Promise<void> {
    await this.post("/api/admin/timer", { minutes });
  }

  /** Starts the next game (only after one has ended). */
  async startGame(): Promise<void> {
    await this.post("/api/admin/start");
  }

  private post(url: string, body?: object): Promise<Response> {
    return this.call("POST", url, body === undefined ? undefined : JSON.stringify(body));
  }

  private async call(method: string, url: string, body?: string): Promise<Response> {
    const headers = new Headers();
    if (body !== undefined) headers.set("Content-Type", "application/json");
    let res: Response;
    try {
      res = await this.fetchFn(url, { method, headers, body, credentials: "same-origin" });
    } catch (e) {
      throw new ApiError("network", 0, e instanceof Error ? e.message : "network error");
    }
    if (!res.ok) throw await toError(res);
    return res;
  }
}
