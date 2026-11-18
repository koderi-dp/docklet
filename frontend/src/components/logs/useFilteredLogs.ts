import { useDeferredValue, useEffect, useMemo, useState } from "react";
import { TIME_RANGES } from "@/lib/logRange";
import type { LogEntry } from "@/lib/logEntry";
import { evaluate, type EvalResult } from "@/lib/logFilter/rules";
import type { LogFilterState } from "./useLogFilter";

/**
 * Runs the filter over the loaded lines. Rules are deferred so the view stays responsive when the buffer is large;
 * patterns are compiled once (memoised in the pattern module), never per line.
 */
export function useFilteredLogs(entries: readonly LogEntry[], f: LogFilterState): EvalResult<LogEntry> {
  const rules = useDeferredValue(f.rules);
  const levels = useDeferredValue(f.levels);
  const [tick, setTick] = useState(0);

  // A relative time range slides with the clock even when no new lines arrive.
  useEffect(() => {
    if (f.range === "all") return;
    const t = setInterval(() => setTick((n) => n + 1), 30_000);
    return () => clearInterval(t);
  }, [f.range]);

  return useMemo(() => {
    const span = TIME_RANGES.find((r) => r.id === f.range)?.ms;
    return evaluate(entries, rules, { includeMode: f.includeMode, levels, since: span ? Date.now() - span : undefined });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [entries, rules, levels, f.includeMode, f.range, tick]);
}
