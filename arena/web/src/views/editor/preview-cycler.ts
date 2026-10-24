/** Injectable interval timers (real ones by default). */
export interface Timers {
  set(fn: () => void, ms: number): unknown;
  clear(id: unknown): void;
}

const REAL: Timers = { set: (fn, ms) => setInterval(fn, ms), clear: (id) => clearInterval(id as ReturnType<typeof setInterval>) };

export interface CyclerDeps {
  steps: number;
  ms: number;
  onStep(step: number): void;
  timers?: Timers;
}

/** The preview's reaction cycle: hover runs through the steps, leave rests, tap advances one step. */
export class PreviewCycler {
  private step = 0;
  private id: unknown = null;
  private running = false;

  constructor(private readonly d: CyclerDeps) {}

  private advance = (): void => {
    this.step = (this.step + 1) % this.d.steps;
    this.d.onStep(this.step);
  };

  tap(): void { this.advance(); }

  hover(): void {
    if (this.running) return;
    this.running = true;
    this.advance();
    this.id = (this.d.timers ?? REAL).set(this.advance, this.d.ms);
  }

  leave(): void {
    this.halt();
    this.step = 0;
    this.d.onStep(0);
  }

  dispose(): void { this.halt(); }

  private halt(): void {
    if (!this.running) return;
    (this.d.timers ?? REAL).clear(this.id);
    this.running = false;
    this.id = null;
  }
}
