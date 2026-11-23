// Pure row model for the Compose projects table: the status bar, health line, filters and the config subline.
import type { ComposeProject, Container } from "@bindings/model";
import { aggregateHealth, isCompleted, isFailed, isRunning } from "./composeGrouping";

export type BarKind = "running" | "failed" | "completed" | "stopped" | "notCreated";
export const BAR_ORDER: readonly BarKind[] = ["running", "failed", "completed", "stopped", "notCreated"];

const KIND_TEXT: Record<BarKind, string> = {
  running: "running",
  failed: "failed",
  completed: "completed",
  stopped: "stopped",
  notCreated: "not created",
};

export interface ProjectBar {
  counts: Record<BarKind, number>;
  segments: BarKind[];
  running: number;
  total: number;
  summary: string;
  label: string;
}

export function projectBar(
  project: Pick<ComposeProject, "services" | "notCreated">,
  containers: readonly Container[],
): ProjectBar {
  const counts: Record<BarKind, number> = { running: 0, failed: 0, completed: 0, stopped: 0, notCreated: 0 };
  for (const c of containers) {
    if (isRunning(c)) counts.running++;
    else if (isFailed(c)) counts.failed++;
    else if (isCompleted(c)) counts.completed++;
    else counts.stopped++;
  }
  counts.notCreated = (project.services ?? []).filter((s) => s.total === 0).length;

  const segments = BAR_ORDER.flatMap((k) => Array<BarKind>(counts[k]).fill(k));
  const total = containers.length + counts.notCreated;
  if (total === 0) {
    return { counts, segments, running: 0, total, summary: "No containers", label: "No containers" };
  }
  const summary =
    `${counts.running} of ${total} running` + (counts.completed > 0 ? ` · ${counts.completed} completed` : "");
  const label = BAR_ORDER.filter((k) => counts[k] > 0)
    .map((k) => `${counts[k]} ${KIND_TEXT[k]}`)
    .join(", ");
  return { counts, segments, running: counts.running, total, summary, label };
}

export function projectHealthLine(
  containers: readonly Container[],
): { text: string; tone: "muted" | "danger" } | undefined {
  const health = aggregateHealth(containers);
  const failed = containers.filter(isFailed).length;
  const parts: string[] = [];
  if (health.withHealthcheck > 0) parts.push(`${health.healthy} of ${health.withHealthcheck} healthy`);
  if (failed > 0) parts.push(`${failed} failed`);
  if (parts.length === 0) return undefined;
  return { text: parts.join(" · "), tone: failed > 0 || health.kind === "degraded" ? "danger" : "muted" };
}

export type ProjectFilter = "all" | "running" | "stopped";

export function isProjectRunning(
  project: Pick<ComposeProject, "running">,
  containers: readonly Container[] | undefined,
): boolean {
  return containers ? containers.some(isRunning) : project.running > 0;
}

export function countProjectFilters<T>(items: readonly T[], running: (t: T) => boolean): Record<ProjectFilter, number> {
  const k = items.filter(running).length;
  return { all: items.length, running: k, stopped: items.length - k };
}

export function filterProjects<T>(items: readonly T[], filter: ProjectFilter, running: (t: T) => boolean): T[] {
  if (filter === "all") return [...items];
  return items.filter((t) => running(t) === (filter === "running"));
}

export function configSubline(project: Pick<ComposeProject, "configFiles" | "capability" | "source">): string {
  const location = (project.capability?.executionLocation ?? "").replace(/^WSL: /, "");
  const files = project.configFiles ?? [];
  let head = project.source;
  if (files.length > 0) {
    head = files[0].split(/[/\\]/).pop() || files[0];
    if (files.length > 1) head += ` +${files.length - 1}`;
  }
  return location ? `${head} · ${location}` : head;
}
