import type { CSSProperties, ReactNode } from "react";
import type { AnsiColour, AnsiSpan, AnsiStyle } from "@/lib/ansi";
import { segmentLine, type Segment } from "@/lib/ansiSegments";
import type { TextRange } from "@/lib/logFilter/search";
import { markClass } from "./MarkedText";

/** A named colour is a theme token (--ansi-0 ... --ansi-15 in globals.css); anything else is drawn as given. */
const css = (c: AnsiColour) => (typeof c === "number" ? `var(--ansi-${c})` : c);

// The named colours are tuned to be read on the log background, so as backgrounds they are light in the dark
// theme and dark in the light one. Black, white or uncoloured text on such a background takes the one colour
// that reads on it (--ansi-on-bg), so "black on green" and "white on red" badges stay readable in both themes.
// On a 256-colour or truecolor background, which is drawn as given, black and white are plain dark and light.
const PLAIN: Record<number, string> = { 0: "#0f1623", 7: "#f1f5fb", 15: "#ffffff" };
const isNeutral = (c: AnsiColour | undefined) => c === undefined || c === 0 || c === 7 || c === 15;

/** Text colour on a filled background; undefined keeps the line's own colour. */
function onBackground(front: AnsiColour | undefined, themed: boolean): string | undefined {
  if (themed && isNeutral(front)) return "var(--ansi-on-bg)";
  if (front === undefined) return undefined;
  return typeof front === "number" && PLAIN[front] ? PLAIN[front] : css(front);
}

/** Bold, italic and underline: the part of a style that is kept inside a search mark. */
function decoration(s: AnsiStyle): CSSProperties {
  const out: CSSProperties = {};
  if (s.bold) out.fontWeight = 700;
  if (s.italic) out.fontStyle = "italic";
  if (s.underline) out.textDecoration = "underline";
  return out;
}

const dimmed = (colour: string) => `color-mix(in oklab, ${colour} 60%, transparent)`;

function Styled({ style, children }: { style: AnsiStyle; children: string }) {
  const outer = decoration(style);
  const back = style.inverse ? style.fg : style.bg;
  const front = style.inverse ? style.bg : style.fg;
  // Inverse with no foreground colour: the background is the line's own tint.
  const tintBack = !!style.inverse && style.fg === undefined;
  const filled = tintBack || back !== undefined;
  let colour = filled ? onBackground(front, tintBack || typeof back === "number") : front !== undefined ? css(front) : undefined;
  if (style.dim) colour = dimmed(colour ?? "currentColor");
  if (tintBack) {
    // currentColor must be read before the text colour changes, hence the inner element.
    return (
      <span style={{ ...outer, backgroundColor: "currentColor" }}>
        <span style={{ color: colour }}>{children}</span>
      </span>
    );
  }
  if (back !== undefined) outer.backgroundColor = css(back);
  if (colour) outer.color = colour;
  return <span style={outer}>{children}</span>;
}

function piece(text: string, seg: Segment, inMark: boolean): ReactNode {
  const chunk = text.slice(seg.start, seg.end);
  if (!seg.style) return chunk;
  if (!inMark) {
    return (
      <Styled key={seg.start} style={seg.style}>
        {chunk}
      </Styled>
    );
  }
  const deco = decoration(seg.style);
  if (Object.keys(deco).length === 0) return chunk;
  return (
    <span key={seg.start} style={deco}>
      {chunk}
    </span>
  );
}

/** A message drawn with its ANSI styles; search matches are marked on top, and the mark's colours win inside a match. */
export function AnsiText({ text, spans, ranges, current }: { text: string; spans: readonly AnsiSpan[]; ranges: readonly TextRange[]; current: boolean }) {
  const segs = segmentLine(text.length, spans, ranges);
  const parts: ReactNode[] = [];
  for (let i = 0; i < segs.length; ) {
    const mark = segs[i].mark;
    if (mark < 0) {
      parts.push(piece(text, segs[i++], false));
      continue;
    }
    const inner: ReactNode[] = [];
    for (; i < segs.length && segs[i].mark === mark; i++) inner.push(piece(text, segs[i], true));
    parts.push(
      <mark key={`m${mark}`} className={markClass(current)}>
        {inner}
      </mark>,
    );
  }
  return <>{parts}</>;
}
