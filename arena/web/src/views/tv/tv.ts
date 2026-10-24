import type { Renderer } from "../../render/scene";
import type { ViewModel } from "../../store/reducer";
import { buildScene } from "./scene";
import { mountOverlay } from "./overlay-dom";
import { overlayOf } from "./overlay";
import { sidebarRows } from "./sidebar";
import { renderSidebar } from "./sidebar-dom";

export interface TvStore {
  state: ViewModel;
  subscribe(fn: (s: ViewModel) => void): () => void;
}

export type RendererFactory = (canvas: HTMLCanvasElement) => Renderer;

/** Mounts the TV view (arena canvas + sidebar) into root; returns an unmount function. */
export function mountTv(root: HTMLElement, store: TvStore, makeRenderer: RendererFactory): () => void {
  root.style.cssText = "display:flex;gap:16px;align-items:flex-start;height:100vh;padding:8px;box-sizing:border-box";
  const stage = document.createElement("div");
  stage.style.cssText = "flex:1;height:100%;min-width:0;position:relative;overflow:hidden";
  const canvas = document.createElement("canvas");
  canvas.style.cssText = "position:absolute;left:0;top:0";
  const table = document.createElement("table");
  // Fixed width and layout: changing cell text (e.g. "reloading") must never resize the arena.
  table.style.cssText = "font:14px monospace;color:#ddd;border-collapse:collapse;flex:none;table-layout:fixed";
  stage.append(canvas);
  root.replaceChildren(stage, table);
  const renderer = makeRenderer(canvas);
  const overlay = mountOverlay(stage);
  const draw = (vm: ViewModel): void => {
    renderer.render(buildScene(vm));
    renderSidebar(table, sidebarRows(vm));
    overlay.render(overlayOf(vm));
  };
  const fit = (): void => renderer.resize(stage.clientWidth, stage.clientHeight);
  const observer = new ResizeObserver(fit);
  observer.observe(stage);
  fit();
  const off = store.subscribe(draw);
  draw(store.state);
  return () => {
    off();
    observer.disconnect();
    renderer.destroy();
  };
}
