// Search of the log view: finds lines, never hides them. Pure: no React, no DOM.
// The query uses the same syntax as filter rules (plain text, `*` / `?` wildcards, /regex/).
import { getPattern, patternRegex } from "./pattern";

export interface TextRange {
  start: number;
  end: number;
}

export interface Search {
  query: string;
  /** Does the line match (cheap: used for counting). */
  test: (text: string) => boolean;
  /** Where the matches are inside the line (used for the rows that are drawn). */
  ranges: (text: string) => TextRange[];
}

export interface CompiledSearch {
  /** Null for an empty or invalid query. */
  search: Search | null;
  /** Why the query cannot be used (invalid regex), or null. */
  error: string | null;
}

const MAX_RANGES_PER_LINE = 100;

/** All non-empty matches of a global regex in a text, left to right. */
export function findRanges(text: string, re: RegExp, limit = MAX_RANGES_PER_LINE): TextRange[] {
  const out: TextRange[] = [];
  re.lastIndex = 0;
  let m: RegExpExecArray | null;
  while (out.length < limit && (m = re.exec(text)) !== null) {
    if (m[0] === "") {
      re.lastIndex++;
      continue;
    }
    out.push({ start: m.index, end: m.index + m[0].length });
  }
  return out;
}

/** Compiles a query once; the result is reused for every line. */
export function compileSearch(query: string): CompiledSearch {
  if (query.trim() === "") return { search: null, error: null };
  const p = getPattern(query);
  if (p.error) return { search: null, error: p.error };
  const re = patternRegex(query);
  if (!re) return { search: null, error: "Invalid pattern" };
  return { search: { query, test: p.test, ranges: (text) => findRanges(text, re) }, error: null };
}

/** The line identity of a match is its `seq`; the list is in display order. */
interface Matchable {
  seq: number;
  text: string;
}

/**
 * Sequence numbers of the matching lines, in the order of `lines`. `member` (when given) limits the search to
 * the lines it holds (the lines some split pane actually shows).
 */
export function matchingSeqs(lines: readonly Matchable[], search: Search, member: ReadonlySet<number> | null = null): number[] {
  const out: number[] = [];
  for (const l of lines) {
    if (member && !member.has(l.seq)) continue;
    if (search.test(l.text)) out.push(l.seq);
  }
  return out;
}

/** 1-based position of the current match in the list; 0 when there is none (or it is not in the list any more). */
export function positionOf(matches: readonly number[], current: number | null): number {
  if (current === null) return 0;
  const i = matches.indexOf(current);
  return i < 0 ? 0 : i + 1;
}

/** The match after (dir 1) or before (dir -1) the current one, wrapping around; with no current match: the first, or the last. */
export function stepMatch(matches: readonly number[], current: number | null, dir: 1 | -1): number | null {
  const n = matches.length;
  if (n === 0) return null;
  const at = current === null ? -1 : matches.indexOf(current);
  if (at < 0) return dir > 0 ? matches[0] : matches[n - 1];
  return matches[(at + dir + n) % n];
}
