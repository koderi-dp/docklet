// Pattern compiler of the log filter. One input syntax for every rule:
//   plain text      -> case-insensitive substring
//   * and ?         -> wildcards (any run of characters / exactly one); `\*`, `\?`, `\/` and `\\` are literal
//   /regex/flags    -> regular expression; case-insensitive unless flags are given
// Compiling never throws: an invalid pattern reports `error` and matches nothing.

export type PatternKind = "text" | "wildcard" | "regex";

export interface CompiledPattern {
  kind: PatternKind;
  /** Set when the pattern cannot be compiled (invalid regex, empty pattern). */
  error: string | null;
  test: (value: string) => boolean;
}

export interface CompileOptions {
  /** Plain text and wildcards must match the whole value (container names, levels) instead of a part of it. */
  whole?: boolean;
}

const REGEX_FORM = /^\/(.+)\/([a-z]*)$/s;
const ESCAPE = /[.*+?^${}()|[\]\\]/g;

const NEVER = (): boolean => false;

function escapeRegex(s: string): string {
  return s.replace(ESCAPE, "\\$&");
}

/** Translates wildcard text into a regex source. */
function wildcardSource(text: string): { source: string; hasWildcard: boolean } {
  let out = "";
  let hasWildcard = false;
  for (let i = 0; i < text.length; i++) {
    const ch = text[i];
    const next = text[i + 1];
    if (ch === "\\" && (next === "*" || next === "?" || next === "/" || next === "\\")) {
      out += escapeRegex(next);
      i++;
    } else if (ch === "*") {
      out += ".*";
      hasWildcard = true;
    } else if (ch === "?") {
      out += ".";
      hasWildcard = true;
    } else {
      out += escapeRegex(ch);
    }
  }
  return { source: out, hasWildcard };
}

function build(kind: PatternKind, source: string, flags: string): CompiledPattern {
  try {
    const re = new RegExp(source, flags);
    return { kind, error: null, test: (v) => re.test(v) };
  } catch (e) {
    const msg = e instanceof Error ? e.message.replace(/^Invalid regular expression: /, "").replace(/^\/.*\/[a-z]*: /, "") : "Invalid regular expression";
    return { kind, error: msg, test: NEVER };
  }
}

/** True when the text is written in the /regex/flags form. */
export function isRegexPattern(src: string): boolean {
  return REGEX_FORM.test(src);
}

interface Parts {
  kind: PatternKind;
  source: string;
  flags: string;
}

/** The regex source and flags a pattern stands for; null for an empty pattern. */
function partsOf(src: string, opts: CompileOptions): Parts | null {
  if (src === "") return null;
  const rx = REGEX_FORM.exec(src);
  if (rx) {
    // g and y make `test` stateful; they have no meaning for a yes/no match.
    const given = rx[2].replace(/[gy]/g, "");
    return { kind: "regex", source: rx[1], flags: given === "" ? "i" : given };
  }
  const { source, hasWildcard } = wildcardSource(src);
  return { kind: hasWildcard ? "wildcard" : "text", source: opts.whole ? `^(?:${source})$` : source, flags: "is" };
}

export function compilePattern(src: string, opts: CompileOptions = {}): CompiledPattern {
  const p = partsOf(src, opts);
  if (!p) return { kind: "text", error: "Empty pattern", test: NEVER };
  return build(p.kind, p.source, p.flags);
}

/**
 * A global regex of the pattern, for finding every match inside a line (the search marks). Null when the
 * pattern is empty or invalid. The regex is stateful: reset `lastIndex` before use.
 */
export function patternRegex(src: string, opts: CompileOptions = {}): RegExp | null {
  const p = partsOf(src, opts);
  if (!p) return null;
  try {
    return new RegExp(p.source, p.flags.includes("g") ? p.flags : p.flags + "g");
  } catch {
    return null;
  }
}

const CACHE_LIMIT = 300;
const cache = new Map<string, CompiledPattern>();

/** Memoised compilePattern: a pattern is compiled once, never per line. */
export function getPattern(src: string, opts: CompileOptions = {}): CompiledPattern {
  const key = `${opts.whole ? "w" : "p"}${src}`;
  let p = cache.get(key);
  if (!p) {
    if (cache.size >= CACHE_LIMIT) cache.clear();
    p = compilePattern(src, opts);
    cache.set(key, p);
  }
  return p;
}

/** Escapes text so that it matches itself literally when used as a pattern (for rules built from a log line). */
export function literalPattern(text: string): string {
  // Every backslash is doubled, not only one in front of a special character: the caller may turn what
  // follows into a wildcard afterwards (digits -> *), and a single backslash would then escape it.
  const t = text.replace(/\\/g, "\\\\").replace(/[*?]/g, "\\$&");
  // Text shaped like /regex/ would be read as one; escaping the first slash keeps it literal.
  return REGEX_FORM.test(t) ? `\\${t}` : t;
}
