import { useCallback, useEffect, useMemo, useState, useSyncExternalStore } from "react";
import type { LogEntry } from "@/lib/logEntry";
import { compileSearch, matchingSeqs, positionOf, stepMatch } from "@/lib/logFilter/search";
import { SearchRegistry } from "./searchRegistry";
import type { SearchContextValue } from "./searchContext";

/** The count pass waits this long after the last keystroke. */
const COUNT_DELAY_MS = 120;
const NONE: readonly number[] = [];

/** `value`, delayed; null follows at once so clearing the box never lags. */
function useDebounced<T>(value: T | null, ms: number): T | null {
  const [d, setD] = useState<T | null>(value);
  useEffect(() => {
    if (value === null) return setD(null);
    const t = setTimeout(() => setD(value), ms);
    return () => clearTimeout(t);
  }, [value, ms]);
  return value === null ? null : d;
}

export interface LogSearchState {
  query: string;
  setQuery: (q: string) => void;
  clear: () => void;
  /** Why the query cannot be used (invalid regex), or null. */
  error: string | null;
  /** The query is non-empty and valid. */
  valid: boolean;
  /** The count has not caught up with the text yet. */
  pending: boolean;
  /** Matching lines (not occurrences), in display order. */
  total: number;
  /** 1-based position of the current match; 0 when none is current. */
  position: number;
  next: () => void;
  prev: () => void;
  context: SearchContextValue;
}

/**
 * Search of a log view. Only lines are found, none is hidden. `entries` are the lines the list shows (after
 * the filters); in the split view `split` limits the search to the lines the panes show. The text is not persisted.
 */
export function useLogSearch(entries: readonly LogEntry[], split: boolean): LogSearchState {
  const [query, setQueryRaw] = useState("");
  const [currentSeq, setCurrentSeq] = useState<number | null>(null);
  const [registry] = useState(() => new SearchRegistry());
  const { search, error } = useMemo(() => compileSearch(query), [query]);
  const counted = useDebounced(search, COUNT_DELAY_MS);
  const version = useSyncExternalStore(registry.subscribe, registry.getVersion);

  useEffect(() => {
    registry.watching = counted !== null;
  }, [registry, counted]);

  const matches = useMemo<readonly number[]>(
    () => (counted ? matchingSeqs(entries, counted, split ? registry.memberSeqs() : null) : NONE),
    // `version` changes when the panes show other lines.
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [entries, counted, split, registry, split ? version : 0],
  );
  const position = positionOf(matches, currentSeq);
  const current = position > 0 ? currentSeq : null;

  const setQuery = useCallback((q: string) => {
    setQueryRaw(q);
    setCurrentSeq(null);
  }, []);
  const clear = useCallback(() => setQuery(""), [setQuery]);

  const step = useCallback(
    (dir: 1 | -1) => {
      const seq = stepMatch(matches, current, dir);
      if (seq === null) return;
      setCurrentSeq(seq);
      registry.reveal(seq);
    },
    [matches, current, registry],
  );
  const next = useCallback(() => step(1), [step]);
  const prev = useCallback(() => step(-1), [step]);

  const context = useMemo<SearchContextValue>(() => ({ search, current, registry }), [search, current, registry]);

  return {
    query, setQuery, clear, error, valid: search !== null, pending: search !== counted,
    total: matches.length, position, next, prev, context,
  };
}
