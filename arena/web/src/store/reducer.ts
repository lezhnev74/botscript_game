import type { Bot, Bullet, Cell, Event, FallenBot, LeaderboardEntry, Message, RebootingBot, Sighting } from "../protocol/protocol";
import { TypeSnapshot } from "../protocol/protocol";

/** How many events the feed keeps (oldest dropped first). */
export const FEED_LIMIT = 100;

export interface Grid {
  width: number;
  height: number;
  rows: string[];
}

/** A scan x-ray: the cone cells and what the scan saw, as the engine reported it (never recomputed). */
export interface ScanRecord {
  bot: string;
  dir: string;
  cells: Cell[];
  seen: Sighting[];
}

/** What the views render: everything derived from the protocol messages. */
export interface ViewModel {
  /** True once a message with a world has arrived. */
  synced: boolean;
  beat: number;
  grid: Grid | null;
  /** Live bots with their badge data (num, line, act, editedBeat, hp, ...). */
  bots: Bot[];
  fallen: FallenBot[];
  /** Bots off the map while their new program restarts (cell = where they left). */
  rebooting: RebootingBot[];
  bullets: Bullet[];
  /** Bullets of the previous beat (empty after a snapshot): lets the TV fly vanished ones into their wall. */
  prevBullets: Bullet[];
  coins: Cell[];
  /** HP of a full health bar (the hp rule); 0 before the first message. */
  maxHp: number;
  /** Beats of a full reload (the reload rule); 0 before the first message. */
  reload: number;
  /** Scan records of the latest beat. */
  scans: ScanRecord[];
  /** Recent events, oldest first, at most FEED_LIMIT. */
  events: Event[];
  board: LeaderboardEntry[];
  myBotId: string | null;
  /** Beats left on the game timer; -1 when no timer runs. */
  remaining: number;
  /** True from the beat the timer ran out until the admin starts the next game. */
  gameOver: boolean;
  /** Length of a beat in ms (0 = unknown): turns `remaining` into a clock. */
  beatMs: number;
}

export function initialState(): ViewModel {
  return {
    synced: false, beat: 0, grid: null, bots: [], fallen: [], rebooting: [], bullets: [], prevBullets: [], coins: [], maxHp: 0, reload: 0,
    scans: [], events: [], board: [], myBotId: null, remaining: -1, gameOver: false, beatMs: 0,
  };
}

/** Records which bot is mine (from POST /api/join). */
export function setMyBot(state: ViewModel, id: string | null): ViewModel {
  return { ...state, myBotId: id };
}

function scansOf(events: Event[]): ScanRecord[] {
  return events
    .filter((e) => e.type === "Scanned")
    .map((e) => ({ bot: e.args[0] ?? "", dir: e.args[1] ?? "", cells: e.cells ?? [], seen: e.seen ?? [] }));
}

function worldOf(m: Message): Omit<ViewModel, "events" | "myBotId" | "prevBullets"> {
  const f = m.frame;
  return {
    synced: true, beat: f.beat, grid: { width: f.width, height: f.height, rows: f.grid },
    bots: f.bots, fallen: f.fallen, rebooting: f.rebooting, bullets: f.bullets, coins: f.coins, maxHp: f.maxHp, reload: f.reload,
    scans: scansOf(m.events), board: m.board, remaining: m.remaining, gameOver: m.gameOver, beatMs: m.beatMs,
  };
}

/** A beat of another game: the admin applied a map, so the world has another size or the beat count started over. */
function isNewGame(state: ViewModel, msg: Message): boolean {
  const g = state.grid;
  if (!state.synced || !g) return false;
  return msg.frame.beat <= state.beat || msg.frame.width !== g.width || msg.frame.height !== g.height;
}

/**
 * Folds one WebSocket message into the view model. A snapshot replaces the
 * world and clears the feed (reconnect); a beat appends its events. A beat that
 * starts a new game (a new map: the beat count restarts or the size changes)
 * restarts the feed with its own events and clears the old bullets too.
 */
export function reduce(state: ViewModel, msg: Message): ViewModel {
  const fresh = msg.type === TypeSnapshot || isNewGame(state, msg);
  const events = [...(fresh ? [] : state.events), ...msg.events].slice(-FEED_LIMIT);
  const prevBullets = fresh ? [] : state.bullets;
  return { ...worldOf(msg), prevBullets, events, myBotId: state.myBotId };
}
