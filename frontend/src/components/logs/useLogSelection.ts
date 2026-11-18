import { useCallback, useEffect, useMemo, useState } from "react";
import type { Container } from "@bindings/model";
import { defaultSelection, MAX_STREAMS } from "@/lib/logSources";

/**
 * Which containers are ticked. On first load the running containers are ticked (up to a cap, reported in
 * `capped`); from then on only the user changes the selection. Ticked containers that no longer exist are dropped.
 */
export function useLogSelection(containers: readonly Container[] | undefined, onLimit: () => void) {
  const [picked, setPicked] = useState<ReadonlySet<string> | null>(null);
  const [capped, setCapped] = useState<{ shown: number; total: number } | null>(null);

  useEffect(() => {
    if (picked !== null || !containers) return;
    const d = defaultSelection(containers);
    setPicked(new Set(d.ids));
    setCapped(d.total > d.ids.length ? { shown: d.ids.length, total: d.total } : null);
  }, [containers, picked]);

  const ticked = useMemo(() => {
    const live = new Set((containers ?? []).map((c) => c.id));
    return new Set([...(picked ?? [])].filter((id) => live.has(id)));
  }, [picked, containers]);

  const setMany = useCallback(
    (ids: readonly string[], on: boolean) => {
      const next = new Set(ticked);
      let hitLimit = false;
      for (const id of ids) {
        if (!on) next.delete(id);
        else if (next.size >= MAX_STREAMS) hitLimit = true;
        else next.add(id);
      }
      setPicked(next);
      setCapped(null);
      if (hitLimit) onLimit();
    },
    [ticked, onLimit],
  );

  const clear = useCallback(() => {
    setPicked(new Set());
    setCapped(null);
  }, []);

  return { ticked, setMany, clear, capped, /** The first selection has been made (containers loaded). */ ready: picked !== null };
}
