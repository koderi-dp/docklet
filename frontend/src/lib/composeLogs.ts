// A Compose project's log snapshot, prepared for display: escape codes parsed once per snapshot,
// the service filter applied afterwards without parsing again. Pure: no React, no DOM.
import { parseAnsi, type AnsiSpan } from "./ansi";

/** One record as the backend returns it (model.ComposeLogLine). */
export interface RawComposeLogLine {
  service: string;
  container: string;
  timestamp: string;
  stream: string;
  text: string;
}

export interface ComposeLogRow {
  /** Position in the fetched snapshot: a key that does not change when the filter does. */
  seq: number;
  service: string;
  container: string;
  timestamp: string;
  stream: string;
  /** The text with escape sequences removed. */
  text: string;
  spans?: AnsiSpan[];
}

/** Parses every line of a snapshot once. */
export function parseComposeLogs(lines: readonly RawComposeLogLine[] | null | undefined): ComposeLogRow[] {
  return (lines ?? []).map((l, seq) => {
    const { text, spans } = parseAnsi(l.text);
    return { seq, service: l.service, container: l.container, timestamp: l.timestamp, stream: l.stream, text, spans };
  });
}

/** The rows of one service, or all rows for "all". Returns the same array when nothing is filtered out. */
export function filterComposeLogs(rows: readonly ComposeLogRow[], service: string): readonly ComposeLogRow[] {
  return service === "all" ? rows : rows.filter((r) => r.service === service);
}
