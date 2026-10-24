/** AI answers come fenced: returns the content of the first ``` block (text outside it is dropped); plain code is returned as is. */
export function stripFence(text: string): string {
  const src = text.replace(/\r\n/g, "\n");
  const open = /^[ \t]*```[^\n]*\n/m.exec(src);
  if (!open) return text;
  const rest = src.slice(open.index + open[0].length);
  const close = /^[ \t]*```/m.exec(rest);
  return close ? rest.slice(0, close.index) : rest;
}
