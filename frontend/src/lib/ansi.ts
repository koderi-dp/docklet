// Terminal escape codes in a log line: SGR (colour and text style) becomes styled ranges over the plain text,
// every other escape sequence is dropped. Pure: no React, no DOM.

/** A colour: 0-15 is one of the 16 named colours (a theme token), a string is a CSS colour drawn as given. */
export type AnsiColour = number | string;

export interface AnsiStyle {
  fg?: AnsiColour;
  bg?: AnsiColour;
  bold?: true;
  dim?: true;
  italic?: true;
  underline?: true;
  inverse?: true;
}

/** A styled range of the plain text: [start, end). Ranges are in order and never overlap. */
export interface AnsiSpan {
  start: number;
  end: number;
  style: AnsiStyle;
}

export interface AnsiLine {
  /** The text with every escape sequence removed. */
  text: string;
  /** Absent when no part of the line is styled. */
  spans?: AnsiSpan[];
}

const ESC = "\u001b";
const BEL = "\u0007";
const CUBE = [0, 95, 135, 175, 215, 255];

/** A colour of the 256-colour table: 0-15 named, 16-231 the 6x6x6 cube, 232-255 the grey ramp. */
function colour256(n: number): AnsiColour {
  if (n < 16) return n;
  if (n < 232) {
    const i = n - 16;
    return `rgb(${CUBE[Math.floor(i / 36)]},${CUBE[Math.floor(i / 6) % 6]},${CUBE[i % 6]})`;
  }
  const v = 8 + (n - 232) * 10;
  return `rgb(${v},${v},${v})`;
}

const isByte = (n: number | undefined): n is number => n !== undefined && Number.isInteger(n) && n >= 0 && n <= 255;

/**
 * Reads the colour after a 38 or 48 at `p[i]`. Returns the colour (undefined when the values are missing or out
 * of range) and how many parameters it took.
 */
function extendedColour(p: number[], i: number): { colour?: AnsiColour; used: number } {
  const mode = p[i + 1];
  if (mode === 5) return { colour: isByte(p[i + 2]) ? colour256(p[i + 2]) : undefined, used: 3 };
  if (mode === 2) {
    const [r, g, b] = [p[i + 2], p[i + 3], p[i + 4]];
    return { colour: isByte(r) && isByte(g) && isByte(b) ? `rgb(${r},${g},${b})` : undefined, used: 5 };
  }
  return { used: 1 };
}

/** `38:2::r:g:b`, `38:2:r:g:b` and `38:5:n` (the colon form) as the numbers of the semicolon form. */
function colonColour(part: string): number[] {
  const n = part.split(":").map((s) => (s === "" ? NaN : Number(s)));
  if (n[1] === 2 && n.length >= 6) return [n[0], 2, ...n.slice(-3)];
  return n.map((v) => (Number.isNaN(v) ? 0 : v));
}

/** The style after applying the parameters of one SGR sequence to `style` (which is not changed). */
function applySgr(style: AnsiStyle, params: string): AnsiStyle {
  const p: number[] = [];
  for (const part of params.split(";")) {
    if (part.includes(":")) {
      // Colon sub-parameters: an extended colour, or a variant of a plain code (4:3 = curly underline, 4:0 = off).
      const head = Number(part.slice(0, part.indexOf(":")));
      if (head === 38 || head === 48) p.push(...colonColour(part));
      else p.push(part === "4:0" ? 24 : head);
    } else p.push(part === "" ? 0 : Number(part));
  }
  let s: AnsiStyle = { ...style };
  for (let i = 0; i < p.length; i++) {
    const c = p[i];
    if (c === 0) s = {};
    else if (c === 1) s.bold = true;
    else if (c === 2) s.dim = true;
    else if (c === 3) s.italic = true;
    else if (c === 4) s.underline = true;
    else if (c === 7) s.inverse = true;
    else if (c === 22) {
      delete s.bold;
      delete s.dim;
    } else if (c === 23) delete s.italic;
    else if (c === 24) delete s.underline;
    else if (c === 27) delete s.inverse;
    else if (c >= 30 && c <= 37) s.fg = c - 30;
    else if (c >= 40 && c <= 47) s.bg = c - 40;
    else if (c >= 90 && c <= 97) s.fg = c - 90 + 8;
    else if (c >= 100 && c <= 107) s.bg = c - 100 + 8;
    else if (c === 39) delete s.fg;
    else if (c === 49) delete s.bg;
    else if (c === 38 || c === 48) {
      const { colour, used } = extendedColour(p, i);
      if (colour !== undefined) s[c === 38 ? "fg" : "bg"] = colour;
      i += used - 1;
    }
  }
  return s;
}

const hasStyle = (s: AnsiStyle) => Object.keys(s).length > 0;
const code = (s: string, i: number) => (i < s.length ? s.charCodeAt(i) : -1);

/** Index after a string sequence (OSC, DCS, ...) whose body starts at `i`: it ends at BEL or ESC \, or with the line. */
function endOfString(raw: string, i: number): number {
  for (; i < raw.length; i++) {
    if (raw[i] === BEL) return i + 1;
    if (raw[i] === ESC) return raw[i + 1] === "\\" ? i + 2 : i;
  }
  return i;
}

/**
 * Splits a line into its plain text and the ranges styled by SGR codes. Every line starts with no style.
 * A line with no escape character is returned as it is, with no range list.
 */
export function parseAnsi(raw: string): AnsiLine {
  let esc = raw.indexOf(ESC);
  if (esc < 0) return { text: raw };

  let text = "";
  const spans: AnsiSpan[] = [];
  let style: AnsiStyle = {};
  const emit = (chunk: string) => {
    if (chunk === "") return;
    if (hasStyle(style)) {
      const last = spans[spans.length - 1];
      if (last && last.style === style && last.end === text.length) last.end += chunk.length;
      else spans.push({ start: text.length, end: text.length + chunk.length, style });
    }
    text += chunk;
  };

  let at = 0;
  while (esc >= 0) {
    emit(raw.slice(at, esc));
    let i = esc + 1;
    const kind = raw[i];
    if (kind === "[") {
      // CSI: parameters 0-?, intermediates space-/, one final byte @-~. Only a plain "...m" is SGR.
      const from = ++i;
      while (code(raw, i) >= 0x30 && code(raw, i) <= 0x3f) i++;
      const params = raw.slice(from, i);
      const inter = i;
      while (code(raw, i) >= 0x20 && code(raw, i) <= 0x2f) i++;
      const final = code(raw, i);
      if (final >= 0x40 && final <= 0x7e) {
        if (raw[i] === "m" && inter === i && /^[0-9;:]*$/.test(params)) style = applySgr(style, params);
        i++;
      }
    } else if (kind === "]" || kind === "P" || kind === "X" || kind === "^" || kind === "_") {
      i = endOfString(raw, i + 1);
    } else if (kind !== undefined && kind !== ESC) {
      // Two-character and charset sequences (ESC c, ESC ( B): intermediates, then one final byte.
      while (code(raw, i) >= 0x20 && code(raw, i) <= 0x2f) i++;
      if (code(raw, i) >= 0x30 && code(raw, i) <= 0x7e) i++;
    }
    at = i;
    esc = raw.indexOf(ESC, at);
  }
  emit(raw.slice(at));
  return spans.length > 0 ? { text, spans } : { text };
}
