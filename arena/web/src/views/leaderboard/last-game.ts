/** One board line as the leaderboard view needs it (structurally the protocol's LeaderboardEntry). */
export interface BoardEntry {
  bot: string;
  name: string;
  score: number;
  kills: number;
  deaths: number;
  coins: number;
}

/**
 * Keeps the "Last game" table current: loads the board once at the start and
 * again every time a game ends (gameOver turns true), because the server has
 * archived a new final board by then. A failed load keeps the old rows.
 * Returns the function to call with every view model.
 */
export function lastGameLoader(
  load: () => Promise<BoardEntry[]>,
  onRows: (rows: BoardEntry[]) => void,
): (vm: { gameOver: boolean }) => void {
  const fetchRows = (): void => {
    load().then(onRows, () => undefined);
  };
  let wasOver = false;
  fetchRows();
  return (vm) => {
    if (vm.gameOver && !wasOver) fetchRows();
    wasOver = vm.gameOver;
  };
}
