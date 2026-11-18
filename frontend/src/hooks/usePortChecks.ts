import { useEffect, useRef, useState } from "react";
import { PortService } from "@bindings/services";
import type { PortCheck } from "@bindings/ports";
import type { PortRow } from "@/components/containers/createContainerForm";
import { errorMessage } from "@/lib/errors";
import { requestsFromRows } from "@/lib/portCheck";

/** The state of one port row: still checking, answered, or the check itself failed (never shown as free). */
export type RowCheck = { state: "checking" } | { state: "done"; check: PortCheck } | { state: "failed"; reason: string };

const DEBOUNCE_MS = 400;

/** Checks every fixed host port of the form rows against the Engine and this machine, debounced. Returns checks by row index. */
export function usePortChecks(connectionId: string, rows: readonly PortRow[], ignoreContainerIds: readonly string[] = []): Record<number, RowCheck> {
  const [result, setResult] = useState<Record<number, RowCheck>>({});
  const seq = useRef(0);
  const reqs = requestsFromRows(rows);
  const key = JSON.stringify([connectionId, reqs.map((r) => r.request), ignoreContainerIds]);

  useEffect(() => {
    const mine = ++seq.current;
    const wanted = JSON.parse(key) as [string, unknown, string[]];
    if (reqs.length === 0) {
      setResult({});
      return;
    }
    setResult(Object.fromEntries(reqs.map((r) => [r.index, { state: "checking" as const }])));
    const timer = setTimeout(() => {
      PortService.Check(connectionId, reqs.map((r) => r.request), wanted[2])
        .then((checks) => {
          if (seq.current !== mine) return;
          setResult(Object.fromEntries(reqs.map((r, i) => [r.index, checks?.[i] ? { state: "done" as const, check: checks[i] } : { state: "failed" as const, reason: "No answer" }])));
        })
        .catch((e) => {
          if (seq.current !== mine) return;
          setResult(Object.fromEntries(reqs.map((r) => [r.index, { state: "failed" as const, reason: errorMessage(e) }])));
        });
    }, DEBOUNCE_MS);
    return () => clearTimeout(timer);
    // `key` captures every input; reqs is derived from it.
  }, [key]);

  return result;
}

/** A fresh check right now (used at submit, so a stale inline state never decides). */
export async function checkRowsNow(connectionId: string, rows: readonly PortRow[], ignoreContainerIds: readonly string[] = []): Promise<{ index: number; check: PortCheck }[]> {
  const reqs = requestsFromRows(rows);
  if (reqs.length === 0) return [];
  const checks = (await PortService.Check(connectionId, reqs.map((r) => r.request), [...ignoreContainerIds])) ?? [];
  return reqs.flatMap((r, i) => (checks[i] ? [{ index: r.index, check: checks[i] }] : []));
}
