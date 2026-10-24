export interface Example {
  name: string;
  source: string;
}

export interface PromptDocs {
  /** LANGUAGE.md. */
  language: string;
  /** VISION.md (the rule sections are cut out of it). */
  vision: string;
  examples: Example[];
  /** The server address the AI should call, no trailing slash (see resolveBaseUrl). */
  baseUrl: string;
}

/** VISION sections that describe the rules: world, weapons, scan, randomness, programming, scoring. */
export const RULE_SECTIONS = [3, 4, 5, 6, 7, 9];

/** Cuts the "## N. title" sections with the given numbers out of a markdown text, in document order. */
export function extractSections(md: string, numbers: number[]): string {
  const parts = md.split(/^(?=## )/m);
  const wanted = new Set(numbers);
  return parts
    .filter((p) => {
      const m = /^## (\d+)\./.exec(p);
      return m !== null && wanted.has(Number(m[1]));
    })
    .map((p) => p.trim())
    .join("\n\n");
}

const ROLE = `You are an expert BOTSCRIPT bot author helping a player build a bot for BOT ARENA, a programming game.
Bots fight in a grid arena; each bot is a small program in BOTSCRIPT (a tiny JavaScript). The language spec, the game
rules and some example bots follow. Use only what the spec allows.`;

const REQUEST = `Before you write any code, ask me what the bot should be optimised for: kills, survival, coins, or a mix
(or a particular style of play). Wait for my answer. Then write one program as a single code block containing only the
BOTSCRIPT program (no let/var, strings, arrays, closures or recursion; at most 120 lines).`;

/** The configured base URL (ARENA_BASE_URL via /api/config) or, when there is none, the page origin. */
export function resolveBaseUrl(configured: string, origin: string): string {
  return (configured || origin).replace(/\/+$/, "");
}

const PRIVATE_HOST = new RegExp(
  "^(localhost|127\\.\\d+\\.\\d+\\.\\d+|10\\.\\d+\\.\\d+\\.\\d+|192\\.168\\.\\d+\\.\\d+|172\\.(1[6-9]|2\\d|3[01])\\.\\d+\\.\\d+" +
    "|169\\.254\\.\\d+\\.\\d+|100\\.(6[4-9]|[7-9]\\d|1[01]\\d|12[0-7])\\.\\d+\\.\\d+|0\\.0\\.0\\.0" +
    "|\\[(::1|f[cd][0-9a-f:]*)\\]|.+\\.local|[^.:\\[\\]]+)$",
);

/** True for addresses an outside AI service cannot reach: localhost, loopback, 10.x, 192.168.x, 172.16-31.x, 169.254.x, 100.64.0.0/10, IPv6 loopback and fc/fd, *.local and single-label names. */
export function isPrivateUrl(url: string): boolean {
  try {
    const host = new URL(url).hostname;
    const mapped = /^\[::ffff:([0-9a-f]{1,4}):([0-9a-f]{1,4})\]$/.exec(host);
    if (mapped) {
      const hi = parseInt(mapped[1] ?? "0", 16), lo = parseInt(mapped[2] ?? "0", 16);
      return PRIVATE_HOST.test(`${hi >> 8}.${hi & 255}.${lo >> 8}.${lo & 255}`);
    }
    return PRIVATE_HOST.test(host);
  } catch {
    return false;
  }
}

function checkSection(base: string): string {
  const note = isPrivateUrl(base)
    ? `\nNote: ${base} is a private address, so you probably cannot reach it from your chat. If a call fails, do not
retry; follow the rules above by hand and still write a program that passes every check you can do in your head.\n`
    : "";
  return `You can test a program on the real server before you give it to me.

- POST ${base}/api/check - send the program as the raw request body (plain text, not JSON).
- GET ${base}/api/check?src=<URL-encoded program> - the same, for tools that can only fetch URLs.

The program is at most 16 KB. The answer is JSON:

- \`{"ok": false, "errors": [{"line": 3, "col": 5, "message": "...", "hint": "..."}]}\` - the program is not valid.
  The hint says what to write instead.
- \`{"ok": true, "trial": {...}}\` - the program is valid. It also played a 100-beat trial against 3 house bots, and
  the trial says what the bot did: \`actions\` (scan, move, fire, wait), \`idle\` (beats missed because the code is
  too slow), \`noAction\` (beats where loop() ended without an action), \`clicks\` (fired with the gun not loaded),
  \`hitsTaken\`, \`hits\`, \`kills\`, \`deaths\`, \`coins\`, and \`warnings\` in plain words.

Check every program you write. Fix all errors and check again until there are none. Act on the warnings (fix them, or
tell me why you leave one) before you give me the final code.
${note}`;
}

function exampleBlock(e: Example): string {
  return `### ${e.name}\n\`\`\`js\n${e.source.trimEnd()}\n\`\`\``;
}

/** The "Copy rulebook for AI" text: role + LANGUAGE.md + game rules + example bots + the request. */
export function buildPrompt(docs: PromptDocs): string {
  const parts = [
    "# Your role\n\n" + ROLE,
    "# Language spec (BOTSCRIPT)\n\n" + docs.language.trim(),
    "# Game rules\n\n" + extractSections(docs.vision, RULE_SECTIONS),
    "# Example bots\n\n" + docs.examples.map(exampleBlock).join("\n\n"),
    "# Check your program\n\n" + checkSection(docs.baseUrl),
    "# Your request\n\n" + REQUEST,
  ];
  return parts.join("\n\n") + "\n";
}
