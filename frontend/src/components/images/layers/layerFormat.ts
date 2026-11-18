import type { ImageLayersReport, ImageLayerStep } from "@bindings/model";
import { formatBytes } from "@/components/containers/format";

/** Semantic-token classes per instruction kind (badge background + text). */
export const KIND_CLASS: Record<string, string> = {
  RUN: "bg-info/15 text-info",
  COPY: "bg-success/15 text-success",
  ADD: "bg-warning/15 text-warning",
  ENV: "bg-primary/15 text-primary",
  CMD: "bg-muted text-muted-foreground",
  OTHER: "bg-muted text-muted-foreground",
};

export function kindLabel(kind: string): string {
  return kind === "OTHER" ? "other" : kind;
}

/** Steps reporting no size are shown as an em dash: the Engine added nothing or did not say. */
export function stepSize(step: ImageLayerStep): string {
  return step.size > 0 ? formatBytes(step.size) : "—";
}

export function formatShare(share: number): string {
  if (share <= 0) return "—";
  const pct = share * 100;
  return pct < 1 ? "<1%" : `${Math.round(pct)}%`;
}

const UNITS: [number, string][] = [
  [365 * 86400, "y"],
  [30 * 86400, "mo"],
  [7 * 86400, "w"],
  [86400, "d"],
  [3600, "h"],
  [60, "m"],
];

/** Compact age such as "3mo ago"; an em dash when the Engine did not report a time. */
export function formatAge(unixSeconds: number, nowMs: number = Date.now()): string {
  if (!unixSeconds) return "—";
  const diff = Math.max(0, Math.floor(nowMs / 1000) - unixSeconds);
  for (const [secs, label] of UNITS) {
    if (diff >= secs) return `${Math.floor(diff / secs)}${label} ago`;
  }
  return "just now";
}

/** The largest step as display parts, or null when no step reports a size. */
export function largestLabel(report: ImageLayersReport): { text: string; size: string; share: string } | null {
  const st = (report.steps ?? []).find((s) => s.index === report.largestStep);
  if (!st) return null;
  return { text: st.instruction || `Step ${st.index}`, size: formatBytes(st.size), share: formatShare(st.share) };
}

/** True when inspect total and summed history differ enough to deserve a note. */
export function totalsDiffer(report: ImageLayersReport): boolean {
  const { totalSize, historySize } = report;
  if (totalSize <= 0 || historySize <= 0) return false;
  return Math.abs(totalSize - historySize) / Math.max(totalSize, historySize) > 0.05;
}
