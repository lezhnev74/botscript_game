// Playwright smoke test: open /bot -> join -> paste a bad bot (see the error markers) -> paste a good bot -> see it on the TV.
// Runs offline against the installed chromium (PLAYWRIGHT_CHROMIUM overrides the lookup). Usage: make e2e
import { spawn, execFileSync } from "node:child_process";
import { existsSync, mkdtempSync, readdirSync, rmSync } from "node:fs";
import { createServer } from "node:net";
import { homedir, tmpdir } from "node:os";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import assert from "node:assert/strict";
import { chromium } from "playwright-core";

const here = dirname(fileURLToPath(import.meta.url));
const engineDir = join(here, "..", "..", "engine");
const distDir = join(here, "..", "dist");
const SHOTS = process.env.E2E_SHOTS ?? "/tmp";

function chromePath() {
  if (process.env.PLAYWRIGHT_CHROMIUM) return process.env.PLAYWRIGHT_CHROMIUM;
  const root = process.env.PLAYWRIGHT_BROWSERS_PATH ?? join(homedir(), ".cache", "ms-playwright");
  const dirs = readdirSync(root).filter((d) => /^chromium-\d+$/.test(d)).sort().reverse();
  const found = dirs.map((d) => join(root, d, "chrome-linux64", "chrome")).find(existsSync);
  assert.ok(found, `no chromium under ${root}; set PLAYWRIGHT_CHROMIUM`);
  return found;
}

function freePort() {
  return new Promise((resolve, reject) => {
    const s = createServer();
    s.listen(0, "127.0.0.1", () => {
      const { port } = s.address();
      s.close(() => resolve(port));
    });
    s.on("error", reject);
  });
}

async function waitUp(url) {
  for (let i = 0; i < 100; i++) {
    try {
      if ((await fetch(url)).ok) return;
    } catch { /* not up yet */ }
    await new Promise((r) => setTimeout(r, 100));
  }
  throw new Error(`server did not come up at ${url}`);
}

async function startServer(tmp) {
  const bin = join(tmp, "arena");
  execFileSync("go", ["build", "-o", bin, "./cmd/arena"], { cwd: engineDir, stdio: "inherit" });
  const port = await freePort();
  const child = spawn(bin, ["-addr", `127.0.0.1:${port}`, "-beat", "100", "-min-bots", "2", "-reboot", "1500ms", "-web", distDir, "-data", join(tmp, "data")], {
    stdio: "inherit",
    env: { ...process.env, ARENA_BASE_URL: `http://127.0.0.1:${port}/` },
  });
  const base = `http://127.0.0.1:${port}`;
  await waitUp(`${base}/api/leaderboard`);
  return { child, base };
}

async function checkApi(base) {
  const config = await (await fetch(`${base}/api/config`)).json();
  assert.equal(config.baseUrl, base, "ARENA_BASE_URL is reported without the trailing slash");

  const bad = await (await fetch(`${base}/api/check`, { method: "POST", body: "function loop() {\n  let x = 1;\n  wait();\n}\n" })).json();
  assert.equal(bad.ok, false);
  assert.equal(bad.errors[0].line, 2);
  assert.ok(bad.errors[0].message && bad.errors[0].hint, "errors carry a message and a hint");

  const src = "function loop() {\n  scan(E);\n  move(pick(N, E, S, W));\n}\n";
  const good = await (await fetch(`${base}/api/check`, { method: "POST", body: src })).json();
  assert.equal(good.ok, true);
  assert.equal(good.trial.beats, 100);
  assert.ok(good.trial.warnings.some((w) => w.includes("never fired")), "a bot that never fires is warned");
  const viaGet = await (await fetch(`${base}/api/check?src=${encodeURIComponent(src)}`)).json();
  assert.deepEqual(viaGet, good, "GET and POST answer the same");
}

// Step 50: leave the arena on /bot (the TV roster drops the bot), then return (it is back, same name).
async function leaveAndReturn(page, base) {
  page.once("dialog", (d) => void d.accept());
  await page.goto(`${base}/bot`);
  await page.locator("#leave-arena").waitFor({ state: "visible" });
  assert.equal(await page.locator("#arena-title").innerText(), "Your bot is in the arena");
  assert.equal(await page.locator("#arena-stay").isHidden(), false, "the default 24 h stay shows its time left");
  assert.match(await page.locator("#arena-stay").innerText(), /23 h/);
  await page.click("#leave-arena");
  await page.locator("#return-arena").waitFor({ state: "visible" });
  assert.equal(await page.locator("#arena-title").innerText(), "Your bot is not in the arena");
  await page.screenshot({ path: join(SHOTS, "step50_bot_detached.png") });

  await page.goto(`${base}/`);
  await page.locator("table tbody tr", { hasText: "Zed" }).waitFor({ state: "detached" });

  await page.goto(`${base}/bot`);
  await page.locator("#return-arena").waitFor({ state: "visible" });
  await page.click("#return-arena"); // too soon after its own leave: a program saved meanwhile must not skip the reboot
  await page.waitForFunction(() => /try again/.test(document.getElementById("arena-info")?.textContent ?? ""));
  await page.waitForTimeout(1800); // the reboot time (1500 ms) after the leave landed
  await page.click("#return-arena");
  await page.locator("#leave-arena").waitFor({ state: "visible" });
  await page.goto(`${base}/`);
  await page.locator("table tbody tr", { hasText: "* Zed" }).waitFor();
  await page.screenshot({ path: join(SHOTS, "step50_tv_back.png") });
}

// Step 51: a saved program takes the bot off the map: the TV roster marks it, then it comes back.
async function rebootOnTv(page, base) {
  await page.goto(`${base}/`);
  const row = page.locator("table tbody tr", { hasText: "* Ann" });
  await row.waitFor();
  await page.locator("table tbody tr", { hasText: "* Ann (rebooting)" }).waitFor({ timeout: 3000 });
  await page.screenshot({ path: join(SHOTS, "step51_tv_rebooting.png") });
  await page.locator("table tbody tr", { hasText: "(rebooting)" }).waitFor({ state: "detached", timeout: 5000 });
  assert.equal(await row.count(), 1, "the bot is back on the roster");
  await page.locator("#add-my-bot").waitFor({ state: "hidden" });
  await page.screenshot({ path: join(SHOTS, "step51_tv_back.png") });
  await page.goto(`${base}/bot`);
  await page.locator("#arena-title").waitFor();
  await page.waitForFunction(() => document.getElementById("arena-title")?.textContent === "Your bot is in the arena");
}

async function run(page, base) {
  await checkApi(base);
  await page.goto(`${base}/`);
  await page.locator("#add-my-bot").waitFor({ state: "visible" });

  await page.goto(`${base}/bot`);
  await page.fill("#name", "Ann");
  await page.click("#join-btn");
  await page.locator("#edit").waitFor({ state: "visible" });
  assert.match(await page.inputValue("#source"), /function loop/, "a new bot starts with a template");

  assert.equal(await page.locator("#save").isDisabled(), true, "Save is disabled when the box is clean");
  await page.fill("#source", "function loop() {\n  let x = 1;\n  wait();\n}\n");
  await page.click("#save");
  await page.locator("#errors li").first().waitFor();
  assert.ok((await page.locator("#gutter .marker").count()) > 0, "error line is marked in the gutter");
  assert.match(await page.locator("#errors li").first().innerText(), /^L2:/);
  await page.screenshot({ path: join(SHOTS, "step18_bot_error.png") });

  await page.fill("#source", "function loop() {\n  move(pick(N, E, S, W));\n}\n");
  await page.click("#save");
  await page.waitForFunction(() => document.getElementById("save-info")?.textContent?.startsWith("Saved"));
  assert.equal(await page.locator("#save-info").innerText(), "Saved - your bot is rebooting and returns in 2 s");
  assert.equal(await page.locator("#save").isDisabled(), true, "Save is disabled after saving");
  await page.locator("#reboot-bar").waitFor({ state: "visible" });
  assert.equal(await page.locator("#arena-title").innerText(), "Rebooting...", "the status card shows the reboot");
  assert.match(await page.locator("#reboot-text").innerText(), /returns in \d s/);
  await page.screenshot({ path: join(SHOTS, "step51_bot_rebooting.png") });

  await rebootOnTv(page, base);

  await page.goto(`${base}/bot`);
  await page.locator("#edit").waitFor({ state: "visible" });
  assert.equal(await page.locator("#save-look").isDisabled(), true, "Save look is disabled when nothing changed");
  await page.fill("#look-name", "Zed");
  await page.fill("#look-color", "#12ab34");
  await page.click("#tile-body-ghost");
  await page.click("#tile-eyes-hearts");
  await page.click("#tile-mouth-tongue");
  await page.click("#tile-hat-crown");
  await page.hover("#look-preview");
  await page.screenshot({ path: join(SHOTS, "step49_bot_edited.png") });
  await page.click("#randomize");
  assert.equal(await page.locator("#save-look").isDisabled(), false, "Randomize leaves something to save");
  await page.click("#tile-body-ghost");
  await page.fill("#look-color", "#12ab34");
  await page.click("#tile-eyes-hearts");
  await page.click("#tile-mouth-tongue");
  await page.click("#tile-hat-crown");
  await page.click("#save-look");
  await page.waitForFunction(() => document.getElementById("look-info")?.textContent?.startsWith("Saved"));
  assert.equal(await page.locator("#save-look").isDisabled(), true, "Save look is disabled after saving");
  assert.equal(await page.locator("#title").innerText(), "Bot: Zed", "the title follows the new name");
  await page.screenshot({ path: join(SHOTS, "step49_bot_saved.png") });

  await page.goto(`${base}/`);
  await page.locator("table tbody tr", { hasText: "* Zed" }).waitFor();
  await page.screenshot({ path: join(SHOTS, "step49_tv.png") });

  await leaveAndReturn(page, base);

  await page.goto(`${base}/leaderboard`);
  await page.locator("#leaderboard tbody tr").first().waitFor();
  await page.locator("#add-my-bot").waitFor({ state: "hidden" });
  await page.screenshot({ path: join(SHOTS, "step18_leaderboard.png") });
}

const tmp = mkdtempSync(join(tmpdir(), "arena-e2e-"));
let server;
let browser;
try {
  server = await startServer(tmp);
  browser = await chromium.launch({ executablePath: chromePath(), args: ["--no-sandbox"] });
  const page = await browser.newPage({ viewport: { width: 1100, height: 780 } });
  const pageErrors = [];
  page.on("pageerror", (e) => pageErrors.push(String(e)));
  await run(page, server.base);
  assert.deepEqual(pageErrors, [], "no uncaught page errors");
  console.log("e2e: ok");
} finally {
  await browser?.close();
  server?.child.kill("SIGTERM");
  rmSync(tmp, { recursive: true, force: true });
}
