import { ArenaController, type ArenaApi, type ArenaModel } from "./arena-controller";
import { H2, HINT, PRIMARY, SECONDARY, TONES, button, el, CARD } from "./dom-kit";

export interface ArenaCard {
  element: HTMLElement;
  controller: ArenaController;
}

/** The status card at the top of the my bot page: in the arena (time left, Leave) or not (Return). */
export function createArenaCard(api: ArenaApi, hooks: { onUnauthorized(): void }): ArenaCard {
  const card = el("section", "", "arena-card", CARD + " space-y-3");
  const title = el("h2", "", "arena-title", H2);
  const stay = el("p", "", "arena-stay", HINT);
  const note = el("p", "", "arena-note", HINT);
  const info = el("p", "", "arena-info");
  const leave = button("Leave the arena", "leave-arena", SECONDARY, () => void ctl.leave(() => confirm("Take your bot out of the arena? Its program, look and score are kept.")));
  const back = button("Return to Arena", "return-arena", PRIMARY, () => void ctl.returnToArena());
  const row = el("div", "", "", "flex flex-wrap items-center gap-3");
  row.append(leave, back);
  const rebootText = el("p", "", "reboot-text", "font-mono text-sm text-emerald-300");
  const rebootBar = el("div", "", "reboot-bar", "h-3 w-full overflow-hidden rounded-sm border border-emerald-700 bg-emerald-950");
  const rebootFill = el("div", "", "reboot-fill", "h-full bg-emerald-400");
  rebootFill.style.backgroundImage = "repeating-linear-gradient(90deg, rgba(0,0,0,0.35) 0 2px, transparent 2px 6px)";
  rebootBar.append(rebootFill);
  card.append(title, rebootText, rebootBar, stay, note, row, info);

  const render = (m: ArenaModel): void => {
    card.hidden = m.view === "loading";
    const inside = m.view === "in";
    const rebooting = inside && m.reboot !== null;
    title.textContent = rebooting ? "Rebooting..." : inside ? "Your bot is in the arena" : "Your bot is not in the arena";
    rebootText.textContent = m.reboot ? (m.reboot.secondsLeft > 0 ? `Your bot is off the map and returns in ${m.reboot.secondsLeft} s` : "Your bot is returning...") : "";
    rebootText.hidden = !rebooting;
    rebootBar.hidden = !rebooting;
    rebootFill.style.width = `${Math.round(100 * (m.reboot?.progress ?? 0))}%`;
    stay.textContent = m.stayText;
    stay.hidden = m.stayText === "";
    note.textContent = m.note;
    note.hidden = m.note === "";
    leave.hidden = !inside;
    back.hidden = inside;
    leave.disabled = m.busy;
    back.disabled = m.busy;
    info.textContent = m.message?.text ?? "";
    info.className = TONES[m.message?.tone ?? "info"];
    info.hidden = m.message === null;
  };

  const ctl = new ArenaController({ api, onChange: render, onUnauthorized: hooks.onUnauthorized });
  return { element: card, controller: ctl };
}
