/**
 * Where the cursor is in a Compose file, worked out from indentation line by line. A file that is
 * being typed is usually not valid YAML at the cursor, and indentation still says which map it is in.
 */
export interface CursorContext {
  /** Keys from the document root to the map or list the cursor is in. A list level is the string "[]". */
  path: string[];
  /** "key": typing a key in a map. "value": after `key: ` on the same line, or a plain list item. */
  kind: "key" | "value";
  /** For kind "value" on a `key: ` line: that key. Undefined for a list item. */
  key?: string;
  /** What is already typed, and where it starts (document offset): the range a suggestion replaces. */
  prefix: string;
  from: number;
  /** Keys already present in the same map (kind "key" only), without the one being typed. */
  siblings: string[];
}

interface Line {
  indent: number;
  body: string;
  /** Empty or a comment: says nothing about structure. */
  blank: boolean;
  /** Indented with a tab: indentation cannot be trusted. */
  tab: boolean;
}

function parseLine(raw: string): Line {
  const text = raw.endsWith("\r") ? raw.slice(0, -1) : raw;
  const ws = /^[ \t]*/.exec(text)![0];
  const body = text.slice(ws.length);
  return { indent: ws.length, body, blank: body === "" || body.startsWith("#"), tab: ws.includes("\t") };
}

/** A `- ` list item: what follows the dash and the column it starts in. */
function dashOf(line: Line): { content: string; contentCol: number } | null {
  const m = /^-(\s+|$)/.exec(line.body);
  return m ? { content: line.body.slice(m[0].length), contentCol: line.indent + m[0].length } : null;
}

const KEY_LINE = /^([A-Za-z0-9_.$<][^:#]*?)\s*:(?:\s+(.*))?$/;

/** A `key:` line. `parent` is true when nothing but a comment follows the colon, so a map or list can hang below it. */
function keyOf(body: string): { key: string; parent: boolean } | null {
  const m = KEY_LINE.exec(body);
  if (!m) return null;
  const rest = (m[2] ?? "").replace(/(^|\s+)#.*$/, "").trim();
  return { key: m[1], parent: rest === "" };
}

/** True when the text ends inside a comment, a quoted string or a flow collection. */
function insideCommentQuoteOrFlow(s: string): boolean {
  let quote = "";
  let depth = 0;
  for (let i = 0; i < s.length; i++) {
    const ch = s[i];
    const prev = i === 0 ? " " : s[i - 1];
    if (quote) {
      if (ch === quote && !(quote === '"' && prev === "\\")) quote = "";
      continue;
    }
    if ((ch === '"' || ch === "'") && /[\s[{,:]/.test(prev)) quote = ch;
    else if (ch === "#" && /\s/.test(prev)) return true;
    else if (ch === "[" || ch === "{") depth++;
    else if ((ch === "]" || ch === "}") && depth > 0) depth--;
  }
  return quote !== "" || depth > 0;
}

/**
 * Null where nothing should be suggested. `readLine` (1-based line number to line text) exists so
 * a caller with its own line store, or a test that counts reads, can supply the lines; by default
 * they are read from `text`.
 */
export function cursorContext(text: string, offset: number, readLine?: (lineNumber: number) => string | undefined): CursorContext | null {
  offset = Math.min(Math.max(offset, 0), text.length);
  const lineStart = text.lastIndexOf("\n", offset - 1) + 1;
  let lineNo = 1;
  for (let i = text.indexOf("\n"); i !== -1 && i < lineStart; i = text.indexOf("\n", i + 1)) lineNo++;
  let split: string[] | undefined;
  const read = readLine ?? ((n: number) => (split ??= text.split("\n"))[n - 1]);

  // Only what is left of the cursor decides where it is.
  const here = parseLine(text.slice(lineStart, offset));
  if (here.tab || insideCommentQuoteOrFlow(here.body)) return null;
  const dash = dashOf(here);
  if (!dash && here.body.startsWith("-")) return null; // "-" with nothing after it yet
  const content = dash ? dash.content : here.body;

  let kind: "key" | "value";
  let key: string | undefined;
  let prefix: string;
  const valueLine = /^([A-Za-z0-9_.$<][^:#]*?)\s*:\s+(.*)$/.exec(content);
  if (valueLine) {
    kind = "value";
    key = valueLine[1];
    prefix = valueLine[2];
  } else if (dash) {
    kind = "value";
    prefix = content;
  } else {
    if (!/^[\w.$-]*$/.test(content)) return null;
    kind = "key";
    prefix = content;
  }
  const from = offset - prefix.length;

  // Walk up. "map": cur is the indentation of the keys of the map we are in.
  // "list": cur is the indentation of the dashes of the list we are in, and its owning key is wanted.
  const path: string[] = [];
  const siblings: string[] = [];
  let mode: "map" | "list" = dash ? "list" : "map";
  let cur = here.indent;
  let collecting = kind === "key";
  if (dash) path.unshift("[]");

  for (let n = lineNo - 1; n >= 1; n--) {
    const L = parseLine(read(n) ?? "");
    if (L.blank) continue;
    if (L.tab) return null;
    if (L.indent > cur) continue; // content of something that closed before our level
    const d = dashOf(L);

    if (mode === "map") {
      if (L.indent === cur) {
        const k = d ? null : keyOf(L.body);
        if (k && collecting) siblings.unshift(k.key);
        continue;
      }
      collecting = false;
      if (d) {
        const k = keyOf(d.content);
        if (!k) return null;
        if (d.contentCol === cur) {
          // The first key of the list item whose map we are in.
          if (kind === "key" && path.length === 0) siblings.unshift(k.key);
        } else {
          if (!k.parent) return null;
          path.unshift(k.key);
        }
        path.unshift("[]");
        mode = "list";
        cur = L.indent;
        continue;
      }
      const k = keyOf(L.body);
      if (!k || !k.parent) return null;
      path.unshift(k.key);
      cur = L.indent;
      if (cur === 0) break;
    } else {
      if (d && L.indent === cur) continue; // an earlier item of the same list
      const k = keyOf(d ? d.content : L.body);
      if (!k || !k.parent) return null;
      path.unshift(k.key);
      cur = L.indent;
      if (d) {
        path.unshift("[]"); // the owning key is itself inside a list item
      } else {
        mode = "map";
        if (cur === 0) break;
      }
    }
  }
  if (mode === "list") return null; // a list with no key above it

  if (kind === "key") {
    for (let n = lineNo + 1; ; n++) {
      const raw = read(n);
      if (raw === undefined) break;
      const L = parseLine(raw);
      if (L.blank) continue;
      if (L.tab || L.indent < here.indent) break;
      if (L.indent > here.indent) continue;
      if (dashOf(L)) continue;
      const k = keyOf(L.body);
      if (k) siblings.push(k.key);
    }
  }

  return { path, kind, key, prefix, from, siblings };
}
