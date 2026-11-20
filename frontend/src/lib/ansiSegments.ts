// Cuts a log line into the pieces a row draws: each piece has one ANSI style (or none) and lies inside one
// search match (or none). Pure: no React, no DOM.
import type { AnsiSpan, AnsiStyle } from "./ansi";
import type { TextRange } from "./logFilter/search";

export interface Segment {
  start: number;
  end: number;
  style?: AnsiStyle;
  /** Index of the search match this piece belongs to; -1 outside every match. */
  mark: number;
}

/** `spans` and `marks` are each in order and free of overlaps; the result covers [0, length) without gaps. */
export function segmentLine(length: number, spans: readonly AnsiSpan[], marks: readonly TextRange[]): Segment[] {
  const cuts = new Set<number>([0, length]);
  for (const r of [...spans, ...marks]) {
    if (r.start < length) cuts.add(r.start);
    if (r.end < length) cuts.add(r.end);
  }
  const points = [...cuts].sort((a, b) => a - b);
  const out: Segment[] = [];
  let s = 0;
  let m = 0;
  for (let i = 0; i + 1 < points.length; i++) {
    const start = points[i];
    const end = points[i + 1];
    while (s < spans.length && spans[s].end <= start) s++;
    while (m < marks.length && marks[m].end <= start) m++;
    const seg: Segment = { start, end, mark: m < marks.length && marks[m].start <= start ? m : -1 };
    if (s < spans.length && spans[s].start <= start) seg.style = spans[s].style;
    out.push(seg);
  }
  return out;
}
