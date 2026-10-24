import { readFileSync } from "node:fs";
import { join } from "node:path";
import type { Message } from "../protocol/protocol";

/** Loads web/fixtures/<name>.json (test helper). */
export function fixture(name: string): Message[] {
  return JSON.parse(readFileSync(join(__dirname, "../../fixtures", `${name}.json`), "utf8")) as Message[];
}
