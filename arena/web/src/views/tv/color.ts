import type { ViewModel } from "../../store/reducer";

/** Drawn for a bot the frame no longer lists (it left while its bullet still flies). */
export const FALLBACK_COLOR = "#9aa0a6";

/** A bot's color: `look.color` from the roster, found among the live, fallen and rebooting bots. */
export function colorOf(vm: Pick<ViewModel, "bots" | "fallen" | "rebooting">, id: string): string {
  const b = vm.bots.find((x) => x.id === id) ?? vm.fallen.find((x) => x.id === id) ?? vm.rebooting.find((x) => x.bot === id);
  return b?.look.color ?? FALLBACK_COLOR;
}
