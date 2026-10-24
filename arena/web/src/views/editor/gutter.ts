import { countLines, markLines, type SrcError } from "./markers";

export const LINE_HEIGHT = 18;

/** A source box: line-number gutter (with error markers) next to a plain textarea. */
export interface SourceBox {
  element: HTMLElement;
  textarea: HTMLTextAreaElement;
  setErrors(errors: SrcError[]): void;
}

export function createSourceBox(): SourceBox {
  const element = document.createElement("div");
  element.style.cssText = "display:flex;border:1px solid #334155;border-radius:8px;overflow:hidden;background:#0b1120";
  const gutter = document.createElement("div");
  gutter.id = "gutter";
  gutter.style.cssText = `width:3.5em;overflow:hidden;text-align:right;color:#64748b;padding:6px 6px 6px 0;font:13px/${LINE_HEIGHT}px monospace;height:320px;box-sizing:border-box`;
  const textarea = document.createElement("textarea");
  textarea.id = "source";
  textarea.spellcheck = false;
  textarea.wrap = "off";
  textarea.placeholder = "Paste your bot here (function loop() { ... })";
  textarea.style.cssText = `flex:1;height:320px;box-sizing:border-box;padding:6px;border:0;outline:0;resize:vertical;background:#0b1120;color:#e2e8f0;font:13px/${LINE_HEIGHT}px monospace`;
  element.append(gutter, textarea);
  let errors: SrcError[] = [];
  const draw = (): void => {
    const bad = new Set(markLines(errors, countLines(textarea.value)).map((m) => m.line));
    gutter.replaceChildren(
      ...Array.from({ length: countLines(textarea.value) }, (_, i) => {
        const row = document.createElement("div");
        row.textContent = String(i + 1);
        if (bad.has(i + 1)) {
          row.className = "marker";
          row.style.cssText = "color:#fff;background:#b3202f";
        }
        return row;
      }),
    );
    gutter.scrollTop = textarea.scrollTop;
  };
  textarea.addEventListener("input", draw);
  textarea.addEventListener("scroll", () => { gutter.scrollTop = textarea.scrollTop; });
  draw();
  return { element, textarea, setErrors: (e) => { errors = e; draw(); } };
}
