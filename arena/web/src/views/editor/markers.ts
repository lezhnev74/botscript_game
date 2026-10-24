export interface SrcError {
  line: number;
  col: number;
  msg: string;
  hint: string;
}

export interface LineMark {
  line: number;
  errors: SrcError[];
}

export function countLines(source: string): number {
  return source.split("\n").length;
}

/** Groups errors per source line (clamped into 1..lineCount), ordered by line, then column. */
export function markLines(errors: SrcError[], lineCount: number): LineMark[] {
  const byLine = new Map<number, SrcError[]>();
  for (const e of [...errors].sort((a, b) => a.line - b.line || a.col - b.col)) {
    const line = Math.min(Math.max(e.line, 1), lineCount);
    byLine.set(line, [...(byLine.get(line) ?? []), e]);
  }
  return [...byLine].sort(([a], [b]) => a - b).map(([line, errs]) => ({ line, errors: errs }));
}

/** "L2:5 message - hint". */
export function formatError(e: SrcError): string {
  return `L${e.line}:${e.col} ${e.msg}` + (e.hint === "" ? "" : ` - ${e.hint}`);
}
