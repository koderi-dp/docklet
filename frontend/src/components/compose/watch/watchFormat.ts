import type { ComposeWatchRule } from "@bindings/model";

/** Elapsed wall-clock time as "45s", "2m 13s" or "1h 02m". */
export function formatElapsed(ms: number): string {
  const s = Math.max(0, Math.floor(ms / 1000));
  if (s < 60) return `${s}s`;
  const m = Math.floor(s / 60);
  if (m < 60) return `${m}m ${String(s % 60).padStart(2, "0")}s`;
  return `${Math.floor(m / 60)}h ${String(m % 60).padStart(2, "0")}m`;
}

/** 24h clock time with seconds ("14:03:09"); "—" when unknown. */
export function formatClock(ms: number): string {
  return ms > 0 ? new Date(ms).toLocaleTimeString(undefined, { hour12: false }) : "—";
}

/**
 * The service arguments of `compose watch`: an empty list (no service args) when every service with rules
 * is ticked, otherwise exactly the ticked ones in rule order.
 */
export function serviceArgs(all: readonly string[], ticked: ReadonlySet<string>): string[] {
  const picked = all.filter((s) => ticked.has(s));
  return picked.length === all.length ? [] : picked;
}

/** "sync → /usr/share/nginx/html" style target text of a rule. */
export function ruleTarget(r: Pick<ComposeWatchRule, "action" | "target" | "exec">): string {
  if (r.action === "sync+exec") return r.exec ? `${r.target || "—"} · runs ${r.exec}` : r.target || "—";
  return r.target || "—";
}

/** Rule actions that copy files into the container (they have a target path). */
export const SYNC_ACTIONS = new Set(["sync", "sync+restart", "sync+exec"]);
