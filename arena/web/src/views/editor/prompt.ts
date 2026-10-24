export interface Example {
  name: string;
  source: string;
}

export interface PromptDocs {
  /** LANGUAGE.md (sections 1-8 go into the prompt). */
  language: string;
  examples: Example[];
  /** The server address the AI should call, no trailing slash (see resolveBaseUrl). */
  baseUrl: string;
}

/** LANGUAGE.md sections that make the spec: rules, shape, execution, memory, readings, scan, built-ins, syntax. */
export const SPEC_SECTIONS = [1, 2, 3, 4, 5, 6, 7, 8];

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

const ROLE = `You are an expert author of BOTSCRIPT bots for BOT ARENA, a grid-arena programming game. The spec and example bots
follow. Use only what the spec allows.`;

const REQUEST = `Before you write any code, ask me what the bot should be optimised for: kills, survival, coins or a mix. Wait for my answer. Then write one program as a single code block, only the BOTSCRIPT.`;

/** The spec text: the intro of LANGUAGE.md (without its title) and sections 1-8. */
export function languageSpec(md: string): string {
  const intro = md.split(/^(?=## )/m)[0] ?? "";
  const lead = intro.replace(/^# .*\n/, "").replace(/^---$/m, "").trim();
  return [lead, extractSections(md, SPEC_SECTIONS)].filter(Boolean).join("\n\n");
}

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
retry; follow the spec by hand and write a program that passes every check you can do in your head.\n`
    : "";
  return `Test the program on the real server before you give it to me.

- POST ${base}/api/check - the program as the raw request body.
- GET ${base}/api/check?src=<URL-encoded program> - the same, for tools that can only fetch URLs.

At most 16 KB. The answer is JSON:

- \`{"ok": false, "errors": [{"line": 3, "col": 5, "message": "...", "hint": "..."}]}\` - the hint says what to write.
- \`{"ok": true, "trial": {...}}\` - valid; it also played a 100-beat trial against 3 house bots. \`trial\` has
  \`actions\` per kind, \`idle\` (beats missed: slow code), \`noAction\`, \`clicks\` (fired unloaded), \`hitsTaken\`, \`hits\`, \`kills\`, \`deaths\`, \`coins\` and \`warnings\`.

Fix all errors and check again until there are none. Act on the warnings (fix them or tell me why you keep one) before the final code.
${note}`;
}

function exampleBlock(e: Example): string {
  return `### ${e.name}\n\`\`\`js\n${e.source.trimEnd()}\n\`\`\``;
}

/** The "Copy rulebook for AI" text: role + LANGUAGE.md sections 1-8 + example bots + check API + the request. */
export function buildPrompt(docs: PromptDocs): string {
  const parts = [
    "# Your role\n\n" + ROLE,
    "# Language spec and game rules (BOTSCRIPT)\n\n" + languageSpec(docs.language),
    "# Example bots\n\n" + docs.examples.map(exampleBlock).join("\n\n"),
    "# Check your program\n\n" + checkSection(docs.baseUrl),
    "# Your request\n\n" + REQUEST,
  ];
  return parts.join("\n\n") + "\n";
}
