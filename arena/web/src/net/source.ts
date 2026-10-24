import type { Message } from "../protocol/protocol";

export type Status = "connecting" | "open" | "closed";

export interface Handlers {
  onMessage(msg: Message): void;
  onStatus(status: Status): void;
}

/** What the app consumes: the live WebSocket client and the fixture player both implement it. */
export interface MessageSource {
  start(handlers: Handlers): void;
  stop(): void;
}
