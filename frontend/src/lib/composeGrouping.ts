// Pure Compose grouping / aggregation / filtering logic for the Containers screen.
// Grouping is done ONLY by the exact `com.docker.compose.project` label (Container.composeProject).
// Never by name, image or path heuristics.
import type { Container } from "@bindings/model";

export const STANDALONE_PROJECT = "__standalone__"; // not a valid Compose project name, so it cannot collide

export type GroupState = "running" | "stopped" | "partial" | "transitioning";
export type GroupHealth = "healthy" | "degraded" | "starting" | "none";
export type StatusTab = "all" | "running" | "stopped";
export type HealthFilter = "all" | "healthy" | "unhealthy" | "starting" | "none";

/** Engine states that count as "stopped" in summary cards / tabs (documented mapping). */
const STOPPED_STATES = new Set(["exited", "created", "dead"]);
const TRANSITIONING_STATES = new Set(["restarting", "removing"]);

export function isRunning(c: Pick<Container, "state">): boolean {
  return c.state === "running";
}
export function isStopped(c: Pick<Container, "state">): boolean {
  return STOPPED_STATES.has(c.state);
}
export function isTransitioningState(state: string): boolean {
  return TRANSITIONING_STATES.has(state);
}
/**
 * A finished one-shot (e.g. a `migrate` service): exited with code 0 and restart policy "no"
 * (the Engine list marks Compose one-offs as "no"). Unknown code or policy never counts.
 */
export function isCompleted(c: Pick<Container, "state" | "exitCode" | "restartPolicy">): boolean {
  return c.state === "exited" && c.exitCode === 0 && c.restartPolicy === "no";
}
/** Exited with a non-zero code. */
export function isFailed(c: Pick<Container, "state" | "exitCode">): boolean {
  return c.state === "exited" && c.exitCode != null && c.exitCode !== 0;
}
export interface OneShotCounts {
  completed: number;
  failed: number;
}
export function countOneShots(containers: readonly Container[]): OneShotCounts {
  let completed = 0;
  let failed = 0;
  for (const c of containers) {
    if (isCompleted(c)) completed++;
    else if (isFailed(c)) failed++;
  }
  return { completed, failed };
}
export function hasHealthcheck(c: Pick<Container, "health">): boolean {
  return c.health === "healthy" || c.health === "unhealthy" || c.health === "starting";
}

/** Stable, engine-scoped identifiers. */
export function containerKey(connectionId: string, containerId: string): string {
  return `${connectionId}:${containerId}`;
}
export function groupId(connectionId: string, project: string | null | undefined): string {
  return `${connectionId}:${project ? project : STANDALONE_PROJECT}`;
}
/** Key used in the UI store's collapsedGroups map (`${connectionId}::${project}`). */
export function groupCollapseKey(connectionId: string, project: string | null | undefined): string {
  return `${connectionId}::${project ? project : STANDALONE_PROJECT}`;
}

export interface ContainerFilters {
  tab: StatusTab;
  search: string;
  /** Exact engine state, or "all". */
  status: string;
  health: HealthFilter;
}

export const EMPTY_FILTERS: ContainerFilters = { tab: "all", search: "", status: "all", health: "all" };

export function hasActiveFilters(f: ContainerFilters): boolean {
  return f.tab !== "all" || f.search.trim() !== "" || f.status !== "all" || f.health !== "all";
}

export interface SummaryCounts {
  total: number;
  running: number;
  stopped: number;
  /** Stopped containers that are finished one-shots / that exited non-zero (both are part of `stopped`). */
  completed: number;
  failed: number;
  /** Containers with health === healthy (never inferred). */
  healthy: number;
  /** Containers that actually have a healthcheck (healthy|unhealthy|starting). */
  withHealthcheck: number;
}

export function summarize(containers: readonly Container[]): SummaryCounts {
  let running = 0;
  let stopped = 0;
  let completed = 0;
  let failed = 0;
  let healthy = 0;
  let withHealthcheck = 0;
  for (const c of containers) {
    if (isRunning(c)) running++;
    else if (isStopped(c)) {
      stopped++;
      if (isCompleted(c)) completed++;
      else if (isFailed(c)) failed++;
    }
    if (c.health === "healthy") healthy++;
    if (hasHealthcheck(c)) withHealthcheck++;
  }
  return { total: containers.length, running, stopped, completed, failed, healthy, withHealthcheck };
}

/**
 * Aggregate state of a set of containers. Finished one-shots (see isCompleted) are ignored:
 * a project whose remaining containers all run is "running". `inFlightIds` are containers with a UI-issued action not yet reflected by the Engine.
 */
export function aggregateState(
  children: readonly Container[],
  inFlightIds?: ReadonlySet<string>,
): GroupState {
  if (children.length === 0) return "stopped";
  if (children.some((c) => isTransitioningState(c.state) || inFlightIds?.has(c.id))) return "transitioning";
  const relevant = children.filter((c) => !isCompleted(c));
  if (relevant.length === 0) return "stopped"; // everything finished: nothing is running
  const running = relevant.filter(isRunning).length;
  if (running === relevant.length) return "running";
  if (running === 0) return "stopped";
  return "partial";
}

export interface HealthSummary {
  kind: GroupHealth;
  healthy: number;
  unhealthy: number;
  starting: number;
  /** Children without a healthcheck (health none / unknown). */
  none: number;
  withHealthcheck: number;
}

export function aggregateHealth(children: readonly Container[]): HealthSummary {
  let healthy = 0;
  let unhealthy = 0;
  let starting = 0;
  let none = 0;
  for (const c of children) {
    if (c.health === "healthy") healthy++;
    else if (c.health === "unhealthy") unhealthy++;
    else if (c.health === "starting") starting++;
    else none++;
  }
  const withHealthcheck = healthy + unhealthy + starting;
  let kind: GroupHealth = "none";
  if (withHealthcheck > 0) {
    kind = unhealthy > 0 ? "degraded" : starting > 0 ? "starting" : "healthy";
  }
  return { kind, healthy, unhealthy, starting, none, withHealthcheck };
}

export interface ComposeGroup {
  /** `connectionId:project` (standalone uses a reserved suffix). */
  id: string;
  collapseKey: string;
  connectionId: string;
  kind: "compose" | "standalone";
  /** Compose project name; null for the Standalone group. */
  project: string | null;
  /** ALL engine containers in the group (unfiltered) — totals always come from here. */
  all: Container[];
  /** Containers matching the current filters (sorted by name). */
  visible: Container[];
  state: GroupState;
  /** Finished one-shots / non-zero exits among ALL children (unfiltered). */
  completed: number;
  failed: number;
  health: HealthSummary;
  /** True when filters hide at least one child. */
  filtered: boolean;
}

export type FlatRow =
  | { kind: "group"; id: string; group: ComposeGroup }
  | { kind: "container"; id: string; container: Container; group: ComposeGroup | null };

export function matchesSearch(c: Container, query: string): boolean {
  const q = query.trim().toLowerCase();
  if (!q) return true;
  return [c.name, c.id, c.image, c.composeService, c.composeProject].some(
    (v) => !!v && v.toLowerCase().includes(q),
  );
}

export function matchesFilters(c: Container, f: ContainerFilters): boolean {
  if (f.tab === "running" && !isRunning(c)) return false;
  if (f.tab === "stopped" && !isStopped(c)) return false;
  if (f.status !== "all" && c.state !== f.status) return false;
  if (f.health !== "all") {
    if (f.health === "none") {
      if (hasHealthcheck(c)) return false;
    } else if (c.health !== f.health) return false;
  }
  return matchesSearch(c, f.search);
}

function byName(a: Container, b: Container): number {
  return a.name.localeCompare(b.name, undefined, { numeric: true, sensitivity: "base" });
}

/** Keep only containers that belong to this connection (never mix engines). Empty connectionId is tolerated. */
export function scopeToConnection(containers: readonly Container[], connectionId: string): Container[] {
  return containers.filter((c) => !c.connectionId || c.connectionId === connectionId);
}

/**
 * Group containers by exact Compose project label. Groups with no visible child are omitted.
 * A project name match (via search) exposes all children because matchesSearch includes composeProject.
 */
export function buildGroups(
  connectionId: string,
  containers: readonly Container[],
  filters: ContainerFilters = EMPTY_FILTERS,
  inFlightIds?: ReadonlySet<string>,
): ComposeGroup[] {
  const scoped = scopeToConnection(containers, connectionId);
  const byProject = new Map<string, Container[]>();
  for (const c of scoped) {
    const key = c.composeProject ? c.composeProject : "";
    const list = byProject.get(key);
    if (list) list.push(c);
    else byProject.set(key, [c]);
  }
  const groups: ComposeGroup[] = [];
  for (const [project, all] of byProject) {
    const visible = all.filter((c) => matchesFilters(c, filters)).sort(byName);
    if (visible.length === 0) continue;
    groups.push({
      id: groupId(connectionId, project),
      collapseKey: groupCollapseKey(connectionId, project),
      connectionId,
      kind: project ? "compose" : "standalone",
      project: project || null,
      all: [...all].sort(byName),
      visible,
      state: aggregateState(all, inFlightIds),
      ...countOneShots(all),
      health: aggregateHealth(all),
      filtered: visible.length !== all.length,
    });
  }
  groups.sort((a, b) => {
    if (a.kind !== b.kind) return a.kind === "standalone" ? 1 : -1; // Standalone last
    return (a.project ?? "").localeCompare(b.project ?? "", undefined, { sensitivity: "base" });
  });
  return groups;
}

/** Visible rows for the grouped view; collapsed groups hide children (header stays). */
export function flattenGrouped(
  groups: readonly ComposeGroup[],
  isCollapsed: (g: ComposeGroup) => boolean,
): FlatRow[] {
  const rows: FlatRow[] = [];
  for (const g of groups) {
    rows.push({ kind: "group", id: `group:${g.id}`, group: g });
    if (isCollapsed(g)) continue;
    for (const c of g.visible) {
      rows.push({ kind: "container", id: containerKey(g.connectionId, c.id), container: c, group: g });
    }
  }
  return rows;
}

/** Flat view rows: filtered containers sorted by name, group kept only for the Project column. */
export function flattenFlat(connectionId: string, containers: readonly Container[], filters: ContainerFilters): FlatRow[] {
  return scopeToConnection(containers, connectionId)
    .filter((c) => matchesFilters(c, filters))
    .sort(byName)
    .map((c) => ({
      kind: "container" as const,
      id: containerKey(connectionId, c.id),
      container: c,
      group: null,
    }));
}

/* ------------------------- selection helpers (explicit ids only) ------------------------- */

export type CheckState = "checked" | "indeterminate" | "unchecked";

export function checkState(ids: readonly string[], selected: ReadonlySet<string>): CheckState {
  if (ids.length === 0) return "unchecked";
  let n = 0;
  for (const id of ids) if (selected.has(id)) n++;
  return n === 0 ? "unchecked" : n === ids.length ? "checked" : "indeterminate";
}

/** Group checkbox: selects only the currently visible children; if all are selected, deselects them. */
export function toggleIds(selected: ReadonlySet<string>, ids: readonly string[]): Set<string> {
  const next = new Set(selected);
  const all = ids.length > 0 && ids.every((id) => next.has(id));
  for (const id of ids) {
    if (all) next.delete(id);
    else next.add(id);
  }
  return next;
}

/** Drop selected ids that no longer exist on the engine (e.g. removed). Selected-but-filtered ids are kept. */
export function pruneSelection(selected: ReadonlySet<string>, existingIds: ReadonlySet<string>): Set<string> {
  const next = new Set<string>();
  for (const id of selected) if (existingIds.has(id)) next.add(id);
  return next.size === selected.size ? (selected as Set<string>) : next;
}

/**
 * Shift+click range selection: every id between `anchorId` and `targetId` (inclusive, in
 * display order) is set to `check`. Falls back to toggling only the target when the anchor
 * is unknown (e.g. not displayed any more).
 */
export function applyRange(
  selected: ReadonlySet<string>,
  ordered: readonly string[],
  anchorId: string | null,
  targetId: string,
  check: boolean,
): Set<string> {
  const next = new Set(selected);
  const a = anchorId == null ? -1 : ordered.indexOf(anchorId);
  const b = ordered.indexOf(targetId);
  const ids = a < 0 || b < 0 ? [targetId] : ordered.slice(Math.min(a, b), Math.max(a, b) + 1);
  for (const id of ids) {
    if (check) next.add(id);
    else next.delete(id);
  }
  return next;
}

/** "1 completed" / "1 failed" notes shown next to a project state; empty when there are none. */
export function oneShotNotes(n: OneShotCounts): { text: string; tone: "muted" | "warning" }[] {
  const out: { text: string; tone: "muted" | "warning" }[] = [];
  if (n.completed > 0) out.push({ text: `${n.completed} completed`, tone: "muted" });
  if (n.failed > 0) out.push({ text: `${n.failed} failed`, tone: "warning" });
  return out;
}
