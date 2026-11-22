import { useEffect, useRef, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { ContainerService } from "@bindings/services";
import type { ContainerStatDetail } from "@bindings/model";
import { usePageVisible } from "@/hooks/queryUtils";

const POLL_MS = 2000;
const HISTORY = 90; // ~3 minutes at 2s

export interface StatPoint {
  cpu: number | null;
  mem: number | null;
  /** Bytes per second since the previous sample; null for the first sample. */
  rxRate: number | null;
  txRate: number | null;
}

/** Rate between two cumulative counters; null when unknown or the counter reset. */
export function ratePerSecond(prev: number | null | undefined, cur: number | null | undefined, seconds: number): number | null {
  if (prev == null || cur == null || seconds <= 0 || cur < prev) return null;
  return (cur - prev) / seconds;
}

/** Live stats polled every 2s only while mounted, running and the window is visible; keeps a short history. */
export function useLiveStats(connectionId: string, containerId: string, running: boolean) {
  const visible = usePageVisible();
  const q = useQuery({
    queryKey: ["container-stat-detail", connectionId, containerId],
    queryFn: () => ContainerService.StatsDetail(connectionId, containerId),
    enabled: running && visible,
    refetchInterval: POLL_MS,
    refetchIntervalInBackground: false,
    retry: false,
  });
  const [history, setHistory] = useState<StatPoint[]>([]);
  const last = useRef<{ at: number; d: ContainerStatDetail } | null>(null);
  useEffect(() => {
    if (!q.data || q.dataUpdatedAt === last.current?.at) return;
    const prev = last.current;
    const d = q.data;
    const secs = prev ? (q.dataUpdatedAt - prev.at) / 1000 : 0;
    last.current = { at: q.dataUpdatedAt, d };
    const point: StatPoint = {
      cpu: d.cpuPercent ?? null,
      mem: d.memoryBytes ?? null,
      rxRate: ratePerSecond(prev?.d.netRxBytes, d.netRxBytes, secs),
      txRate: ratePerSecond(prev?.d.netTxBytes, d.netTxBytes, secs),
    };
    setHistory((h) => [...h, point].slice(-HISTORY));
  }, [q.data, q.dataUpdatedAt]);
  return { data: q.data as ContainerStatDetail | undefined, error: q.error, isLoading: q.isLoading, history };
}
