// Log search: case-insensitive text or regex. An invalid regex reports an error and matches nothing
// as a filter (the caller shows every line).

export interface Search {
  /** null when the query is empty or invalid (no filtering). */
  re: RegExp | null;
  error: string | null;
}

export function compileSearch(query: string, useRegex: boolean): Search {
  if (query === "") return { re: null, error: null };
  try {
    const source = useRegex ? query : query.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
    return { re: new RegExp(source, "gi"), error: null };
  } catch (e) {
    return { re: null, error: e instanceof Error ? e.message.replace(/^Invalid regular expression: /, "") : "Invalid regular expression" };
  }
}

export type Range = [start: number, end: number];

/** Non-empty match ranges of the search in one line. */
export function matchRanges(text: string, re: RegExp): Range[] {
  const out: Range[] = [];
  re.lastIndex = 0;
  let m: RegExpExecArray | null;
  while ((m = re.exec(text)) !== null) {
    if (m[0] === "") {
      re.lastIndex++; // empty match (e.g. a*): skip, never loop forever
      continue;
    }
    out.push([m.index, m.index + m[0].length]);
    if (out.length >= 200) break; // a pathological pattern must not freeze the view
  }
  return out;
}

/** Split a line into plain and matched segments for highlighting. */
export function segments(text: string, ranges: readonly Range[]): Array<{ text: string; match: boolean; index?: number }> {
  const out: Array<{ text: string; match: boolean; index?: number }> = [];
  let pos = 0;
  ranges.forEach(([s, e], i) => {
    if (s > pos) out.push({ text: text.slice(pos, s), match: false });
    out.push({ text: text.slice(s, e), match: true, index: i });
    pos = e;
  });
  if (pos < text.length) out.push({ text: text.slice(pos), match: false });
  return out;
}
