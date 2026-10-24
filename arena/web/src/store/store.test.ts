import { describe, expect, it } from "vitest";
import { fixture } from "../testing/fixtures";
import { Store } from "./store";

describe("Store", () => {
  it("folds messages and notifies subscribers with the new state", () => {
    const store = new Store();
    const seen: number[] = [];
    store.subscribe((s) => seen.push(s.beat));
    for (const m of fixture("move_free_cell")) store.dispatch(m);
    expect(seen).toEqual([0, 1, 2]);
    expect(store.state.beat).toBe(2);
  });
  it("unsubscribe stops notifications", () => {
    const store = new Store();
    let n = 0;
    const off = store.subscribe(() => n++);
    off();
    store.dispatch(fixture("move_free_cell")[0]!);
    expect(n).toBe(0);
  });
  it("remembers my bot", () => {
    const store = new Store();
    store.setMyBot("A");
    store.dispatch(fixture("move_free_cell")[0]!);
    expect(store.state.myBotId).toBe("A");
  });
});
