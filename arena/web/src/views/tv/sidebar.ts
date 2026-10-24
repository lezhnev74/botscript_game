import type { ViewModel } from "../../store/reducer";
import { colorOf } from "./color";

export interface SidebarRow {
  id: string;
  name: string;
  color: string;
  hp: number;
  maxHp: number;
  kills: number;
  deaths: number;
  score: number;
  coins: number;
  mine: boolean;
  /** Off the map while a new program restarts. */
  rebooting: boolean;
}

type Base = Pick<SidebarRow, "id" | "name" | "hp" | "rebooting">;

function presentBots(vm: ViewModel): Base[] {
  const alive = vm.bots.map((b) => ({ id: b.id, name: b.name, hp: b.hp, rebooting: false }));
  const dead = vm.fallen.map((f) => ({ id: f.id, name: f.name, hp: 0, rebooting: false }));
  const away = vm.rebooting.map((r) => ({ id: r.bot, name: r.name, hp: r.hp, rebooting: true }));
  return [...alive, ...dead, ...away];
}

/**
 * Sidebar rows: bots in the arena (alive, waiting to respawn or rebooting), in board
 * order; present bots without a board entry follow with zero stats.
 */
export function sidebarRows(vm: ViewModel): SidebarRow[] {
  const present = new Map(presentBots(vm).map((b) => [b.id, b]));
  const known = vm.board.filter((e) => present.has(e.bot));
  const ranked = known.map((e) => ({ base: present.get(e.bot)!, kills: e.kills, deaths: e.deaths, score: e.score, coins: e.coins }));
  const unranked = [...present.values()]
    .filter((b) => !known.some((e) => e.bot === b.id))
    .map((base) => ({ base, kills: 0, deaths: 0, score: 0, coins: 0 }));
  return [...ranked, ...unranked].map(({ base, ...stats }) => ({
    ...base, ...stats, color: colorOf(vm, base.id), mine: base.id === vm.myBotId, maxHp: vm.maxHp,
  }));
}
