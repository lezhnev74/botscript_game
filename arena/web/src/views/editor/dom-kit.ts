// Small DOM helpers and Tailwind class sets shared by the my bot page cards.

export const CARD = "rounded-xl border border-slate-700 bg-slate-900 p-6 shadow-sm";
export const H2 = "text-lg font-semibold text-slate-100";
export const HINT = "text-sm text-slate-400";
export const INPUT = "rounded-md border border-slate-600 bg-slate-950 px-3 py-2 text-sm text-slate-100 placeholder-slate-500 focus:border-indigo-400 focus:outline-none focus:ring-2 focus:ring-indigo-500/40";
export const PRIMARY = "appearance-none rounded-md border border-solid border-transparent bg-indigo-500 px-4 py-2 text-sm font-semibold text-white hover:bg-indigo-400 disabled:cursor-not-allowed disabled:bg-slate-700 disabled:text-slate-400";
export const SECONDARY = "rounded-md border border-slate-600 bg-slate-800 px-4 py-2 text-sm font-semibold text-slate-100 hover:bg-slate-700 disabled:cursor-not-allowed disabled:opacity-50";

export function el<K extends keyof HTMLElementTagNameMap>(tag: K, text = "", id = "", cls = ""): HTMLElementTagNameMap[K] {
  const e = document.createElement(tag);
  e.textContent = text;
  if (id) e.id = id;
  if (cls) e.className = cls;
  return e;
}

export function button(text: string, id: string, cls: string, onClick: () => void): HTMLButtonElement {
  const b = el("button", text, id, cls);
  b.type = "button";
  b.addEventListener("click", onClick);
  return b;
}

export type Tone = "ok" | "error" | "info";
export const TONES: Record<Tone, string> = {
  ok: "rounded-md border border-emerald-700 bg-emerald-950 px-3 py-2 text-sm text-emerald-300",
  error: "rounded-md border border-rose-700 bg-rose-950 px-3 py-2 text-sm text-rose-300",
  info: "text-sm text-slate-400",
};

export function section(title: string, hint: string): HTMLElement {
  const s = el("section", "", "", CARD + " space-y-4");
  s.append(el("h2", title, "", H2), el("p", hint, "", HINT));
  return s;
}
