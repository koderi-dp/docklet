import { useMemo, useRef } from "react";
import type { LogEntry } from "@/lib/logEntry";
import { getPattern } from "@/lib/logFilter/pattern";
import type { HighlightColour } from "@/lib/logFilter/rules";
import { sameLines, selectPaneLines, type PaneLines } from "@/lib/logPanes";

/**
 * The lines of one pane, taken from the already filtered stream (no rule is evaluated again). The result keeps
 * its identity while the pane's own lines did not change, so a pane whose containers received nothing in a
 * batch does not re-render.
 */
export function usePaneLines(
  visible: LogEntry[],
  highlights: Array<HighlightColour | undefined>,
  names: readonly string[],
  allTicked: boolean,
  text: string,
): { lines: PaneLines; textError: string | null } {
  const key = names.join("\n");
  const pattern = useMemo(() => (text === "" ? null : getPattern(text)), [text]);
  const last = useRef<PaneLines | null>(null);
  const lines = useMemo(() => {
    const set = allTicked ? null : new Set(key === "" ? [] : key.split("\n"));
    // An invalid pattern filters nothing, like an invalid rule chip.
    const test = pattern && !pattern.error ? pattern.test : null;
    const next = selectPaneLines(visible, highlights, set, test);
    if (last.current && sameLines(last.current, next)) return last.current;
    last.current = next;
    return next;
  }, [visible, highlights, key, allTicked, pattern]);
  return { lines, textError: pattern?.error ?? null };
}
