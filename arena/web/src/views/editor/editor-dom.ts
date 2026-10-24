import type { ViewModel } from "../../store/reducer";
import { EditorController, canSave, isDirty, type EditorApi, type EditorModel, type SaveState, type Session } from "./controller";
import { cacheBaseUrl } from "./baseurl";
import { copyText } from "./copy";
import { createSourceBox } from "./gutter";
import { formatError } from "./markers";
import { CARD, H2, HINT, INPUT, PRIMARY, SECONDARY, TONES, button, el, section, type Tone } from "./dom-kit";
import { createArenaCard } from "./arena-dom";
import type { ArenaApi } from "./arena-controller";
import { createLookCard } from "./look-dom";
import type { ProfileApi } from "./look-controller";
import { buildPrompt, type Example, type PromptDocs } from "./prompt";

export interface EditorStore {
  state: ViewModel;
  subscribe(fn: (s: ViewModel) => void): () => void;
  setMyBot(id: string | null): void;
}

export interface EditorDeps {
  api: EditorApi & ProfileApi & ArenaApi;
  session: Session;
  templates: Example[];
  docs: Omit<PromptDocs, "examples" | "baseUrl">;
  /** The configured public base URL of the server, "" when there is none or it cannot be read. */
  baseUrl: () => Promise<string>;
}

function saveText(s: SaveState): { text: string; tone: Tone } {
  switch (s.kind) {
    case "saving": return { text: "Saving...", tone: "info" };
    case "saved": return { text: s.rebootSeconds ? `Saved - your bot is rebooting and returns in ${s.rebootSeconds} s` : "Saved - your bot runs it from the next beat", tone: "ok" };
    case "errors": return { text: `Not saved: ${s.errors.length} problem(s). Fix the marked lines.`, tone: "error" };
    case "wait": return { text: `Too many edits: try again in ${s.beats} beat(s).`, tone: "error" };
    case "failed": return { text: `Save failed: ${s.message}`, tone: "error" };
    default: return { text: "", tone: "info" };
  }
}

/** Mounts the "my bot" page (join, edit and save, presets, rulebook for AI); returns an unmount function. */
export function mountEditor(root: HTMLElement, store: EditorStore, deps: EditorDeps): () => void {
  root.className = "mx-auto max-w-3xl space-y-6 p-4 font-sans text-slate-200 sm:p-6";
  const message = el("p", "", "message", "text-sm text-amber-300");
  const joinBox = el("form", "", "join", CARD + " space-y-4");
  const name = el("input", "", "name", INPUT + " w-full sm:w-72");
  name.placeholder = "Your bot's name";
  name.maxLength = 24;
  const joinBtn = el("button", "Join", "join-btn", PRIMARY);
  joinBtn.type = "submit";
  const joinRow = el("div", "", "", "flex flex-wrap items-center gap-3");
  joinRow.append(name, joinBtn);
  joinBox.append(el("h2", "Join the arena", "", H2), el("p", "Pick a name to get your own bot.", "", HINT), joinRow);
  const retryBox = el("div", "", "retry", CARD);
  retryBox.append(button("Retry", "retry-btn", PRIMARY, () => void ctl.retry()));

  const editBox = el("div", "", "edit", "space-y-6");
  const title = el("h1", "", "title", "text-2xl font-bold text-slate-100");

  const manual = section("1. Edit my bot", "Write or paste BOTSCRIPT, then save. The box shows the program your bot runs now.");
  const box = createSourceBox();
  box.textarea.addEventListener("input", () => ctl.edit(box.textarea.value));
  const badge = el("span", "", "state-badge", "rounded-full px-2.5 py-0.5 text-xs font-medium");
  const saveBtn = button("Save", "save", PRIMARY, () => void ctl.save(box.textarea.value));
  const saveRow = el("div", "", "", "flex items-center gap-3");
  saveRow.append(saveBtn, badge);
  const saveInfo = el("p", "", "save-info");
  const errors = el("ul", "", "errors", "space-y-1 font-mono text-sm text-rose-300");
  manual.append(box.element, saveRow, saveInfo, errors);

  const preset = section("2. Start from a preset", "Fill the box with a ready-made bot, or get the rulebook to ask an AI to write one. Then save it above.");
  const pick = el("select", "", "templates", INPUT);
  deps.templates.forEach((t, i) => pick.append(new Option(t.name, String(i))));
  const loadTpl = button("Load into editor", "load-template", SECONDARY, () => {
    ctl.loadPreset(deps.templates[Number(pick.value)]?.source ?? "", (q) => confirm(q));
  });
  const tplRow = el("div", "", "", "flex flex-wrap items-center gap-3");
  tplRow.append(pick, loadTpl);
  const COPY_LABEL = "Copy rulebook for AI";
  const copyBtn = button(COPY_LABEL, "copy-prompt", SECONDARY, () => void copy());
  const baseUrl = cacheBaseUrl(deps.baseUrl, location.origin); // fetched once, so the click copies inside the user gesture
  const copy = async (): Promise<void> => {
    const text = buildPrompt({ ...deps.docs, examples: deps.templates, baseUrl: baseUrl() });
    copyBtn.textContent = (await copyText(text)) ? "Copied!" : "Copy failed";
    setTimeout(() => { copyBtn.textContent = COPY_LABEL; }, 2000);
  };
  const aiRow = el("div", "", "", "flex flex-wrap items-center gap-3");
  aiRow.append(copyBtn, el("span", "Paste it into your AI chat; it will ask what your bot should do.", "", HINT));
  preset.append(tplRow, aiRow);
  const look = createLookCard(deps.api, { onSaved: (n) => ctl.renamed(n), onUnauthorized: () => void ctl.expire() });
  const arena = createArenaCard(deps.api, { onUnauthorized: () => void ctl.expire() });
  editBox.append(title, arena.element, manual, preset, look.element);

  const ctl: EditorController = new EditorController({
    api: deps.api,
    session: deps.session,
    onJoined: (id) => store.setMyBot(id),
    onChange: (m) => render(m),
  });
  root.replaceChildren(message, joinBox, retryBox, editBox);
  joinBox.addEventListener("submit", (ev) => { ev.preventDefault(); void ctl.join(name.value); });

  let loadedLook: EditorModel["look"] = null;
  let wasReady = false;
  let wasSaved = false;

  function render(m: EditorModel): void {
    const ready = m.phase === "ready";
    joinBox.hidden = m.phase !== "need-name";
    retryBox.hidden = m.phase !== "offline" && m.phase !== "full";
    editBox.hidden = !ready;
    if (ready && !wasReady) void arena.controller.load();
    wasReady = ready;
    const saved = m.save.kind === "saved";
    if (saved && !wasSaved) void arena.controller.load(); // a save may have started a reboot: show it in the status card
    wasSaved = saved;
    message.textContent = m.message;
    message.hidden = m.message === "";
    title.textContent = `Bot: ${m.botName}`;
    if (m.look && m.look !== loadedLook) { loadedLook = m.look; look.controller.load(m.botName, m.look); }
    if (box.textarea.value !== m.source) {
      box.textarea.value = m.source;
      box.textarea.dispatchEvent(new Event("input"));
    }
    const dirty = isDirty(m);
    badge.textContent = dirty ? "Unsaved changes" : "Saved";
    badge.className = "rounded-full px-2.5 py-0.5 text-xs font-medium " + (dirty ? "bg-amber-900 text-amber-300" : "bg-emerald-900 text-emerald-300");
    saveBtn.disabled = !canSave(m);
    const errs = m.save.kind === "errors" ? m.save.errors : [];
    box.setErrors(errs);
    errors.replaceChildren(...errs.map((e) => el("li", formatError(e))));
    const info = saveText(m.save);
    saveInfo.textContent = info.text;
    saveInfo.className = TONES[info.tone];
    saveInfo.hidden = info.text === "";
  }

  let lastBeat = -1;
  const off = store.subscribe((vm) => {
    if (vm.beat !== lastBeat) { lastBeat = vm.beat; void ctl.poll(); if (wasReady) void arena.controller.poll(); }
  });
  void ctl.start();
  return () => { off(); look.dispose(); };
}
