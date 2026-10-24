import { formatRemaining } from "../../store/clock";
import type { ViewModel } from "../../store/reducer";
import { buildRows, type BoardRow } from "../../store/standings";

export interface Overlay {
  /** "mm:ss" while a timer runs, else null. */
  clock: string | null;
  gameOver: boolean;
  /** The final board, shown only when the game is over (empty before). */
  rows: BoardRow[];
}

/** What the TV draws over the arena: the countdown, or "Game over" with the final board. */
export function overlayOf(vm: Pick<ViewModel, "remaining" | "beatMs" | "gameOver" | "board" | "myBotId">): Overlay {
  if (vm.gameOver) return { clock: null, gameOver: true, rows: buildRows(vm) };
  return { clock: formatRemaining(vm.remaining, vm.beatMs), gameOver: false, rows: [] };
}
