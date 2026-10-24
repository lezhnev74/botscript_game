import type { Message } from "../protocol/protocol";
import { initialState, reduce, setMyBot, type ViewModel } from "./reducer";

/** Holds the view model; views subscribe, the net layer dispatches. */
export class Store {
  state: ViewModel = initialState();
  private readonly listeners = new Set<(s: ViewModel) => void>();

  dispatch(msg: Message): void {
    this.set(reduce(this.state, msg));
  }

  setMyBot(id: string | null): void {
    this.set(setMyBot(this.state, id));
  }

  subscribe(fn: (s: ViewModel) => void): () => void {
    this.listeners.add(fn);
    return () => { this.listeners.delete(fn); };
  }

  private set(s: ViewModel): void {
    this.state = s;
    this.listeners.forEach((fn) => fn(s));
  }
}
