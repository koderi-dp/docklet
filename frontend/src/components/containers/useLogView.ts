import { useCallback, useDeferredValue, useEffect, useMemo, useState } from "react";
import { stripAnsi } from "@/lib/ansi";
import { detectLevel, LOG_LEVELS, type LogLevel } from "@/lib/logLevel";
import { inTimeRange, type TimeRange } from "@/lib/logRange";
import { compileSearch, matchRanges, type Range } from "@/lib/logSearch";
import type { ViewLine } from "./useLogStream";

export interface ShownLine {
  line: ViewLine;
  /** ANSI-stripped text. */
  text: string;
  level: LogLevel;
  ranges: Range[];
  /** Global index of this line's first search match (matches are numbered across all shown lines). */
  firstMatch: number;
}

interface Analysed {
  line: ViewLine;
  text: string;
  level: LogLevel;
}

// A line never changes after it arrives, so its text and level are computed once.
const analysedCache = new WeakMap<ViewLine, Analysed>();
function analyse(line: ViewLine): Analysed {
  let a = analysedCache.get(line);
  if (!a) {
    const text = stripAnsi(line.text);
    a = { line, text, level: detectLevel(text) };
    analysedCache.set(line, a);
  }
  return a;
}

const EMPTY_COUNTS: Record<LogLevel, number> = { error: 0, warn: 0, info: 0, debug: 0, other: 0 };

/** Filtering pipeline of the log viewer: time range, search (text or regex), level chips, match navigation. */
export function useLogView(lines: readonly ViewLine[]) {
  const [search, setSearch] = useState("");
  const [useRegex, setUseRegex] = useState(false);
  const [levels, setLevels] = useState<ReadonlySet<LogLevel>>(new Set());
  const [range, setRange] = useState<TimeRange>("all");
  const [current, setCurrent] = useState(0);
  const [tick, setTick] = useState(0);

  // A relative range slides with the clock even when no new lines arrive.
  useEffect(() => {
    if (range === "all") return;
    const t = setInterval(() => setTick((n) => n + 1), 30_000);
    return () => clearInterval(t);
  }, [range]);

  const deferred = useDeferredValue(search);
  const compiled = useMemo(() => compileSearch(deferred, useRegex), [deferred, useRegex]);

  const timed = useMemo(() => {
    const now = Date.now();
    return lines.filter((l) => inTimeRange(l.timestamp, range, now)).map(analyse);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [lines, range, tick]);

  const searched = useMemo(() => {
    const re = compiled.re;
    if (!re) return timed.map((a) => ({ ...a, ranges: [] as Range[] }));
    const out: Array<Analysed & { ranges: Range[] }> = [];
    for (const a of timed) {
      const ranges = matchRanges(a.text, re);
      if (ranges.length) out.push({ ...a, ranges });
    }
    return out;
  }, [timed, compiled]);

  const counts = useMemo(() => {
    const c = { ...EMPTY_COUNTS };
    for (const a of searched) c[a.level]++;
    return c;
  }, [searched]);

  const { shown, totalMatches } = useMemo(() => {
    let n = 0;
    const out: ShownLine[] = [];
    for (const a of searched) {
      if (levels.size > 0 && !levels.has(a.level)) continue;
      out.push({ ...a, firstMatch: n });
      n += a.ranges.length;
    }
    return { shown: out, totalMatches: n };
  }, [searched, levels]);

  // The active match always stays inside the available matches.
  const active = totalMatches === 0 ? 0 : Math.min(current, totalMatches - 1);

  const step = useCallback(
    (dir: 1 | -1) => {
      if (totalMatches === 0) return;
      setCurrent((c) => (Math.min(c, totalMatches - 1) + dir + totalMatches) % totalMatches);
    },
    [totalMatches],
  );

  const toggleLevel = useCallback((l: LogLevel) => {
    setLevels((prev) => {
      const next = new Set(prev);
      if (next.has(l)) next.delete(l);
      else next.add(l);
      return next;
    });
  }, []);

  const changeSearch = useCallback((v: string) => {
    setSearch(v);
    setCurrent(0);
  }, []);

  return {
    search, setSearch: changeSearch, useRegex, setUseRegex, regexError: compiled.error, hasQuery: compiled.re !== null,
    levels, toggleLevel, counts, range, setRange,
    shown, totalMatches, active, step, levelOrder: LOG_LEVELS,
  };
}
