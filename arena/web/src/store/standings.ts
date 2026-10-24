import type { ViewModel } from "./reducer";

export interface BoardRow {
  rank: number;
  id: string;
  name: string;
  score: number;
  kills: number;
  deaths: number;
  coins: number;
  mine: boolean;
}

type Entry = ViewModel["board"][number];

function compare(a: Entry, b: Entry): number {
  return b.score - a.score || b.kills - a.kills || a.deaths - b.deaths || a.name.localeCompare(b.name) || a.bot.localeCompare(b.bot);
}

const sameStanding = (a: Entry, b: Entry): boolean => a.score === b.score && a.kills === b.kills && a.deaths === b.deaths;

/** Leaderboard rows: best first, ties (same score, kills, deaths) share a rank. */
export function buildRows(vm: Pick<ViewModel, "board" | "myBotId">): BoardRow[] {
  const sorted = [...vm.board].sort(compare);
  let rank = 0;
  return sorted.map((e, i) => {
    const prev = sorted[i - 1];
    if (!prev || !sameStanding(prev, e)) rank = i + 1;
    return { rank, id: e.bot, name: e.name, score: e.score, kills: e.kills, deaths: e.deaths, coins: e.coins, mine: e.bot === vm.myBotId };
  });
}
