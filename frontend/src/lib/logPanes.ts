// Split view of the Logs screen: which containers each pane shows. Pure: no React, no DOM.
// Panes are views over the one merged stream; containers are always identified by NAME so an assignment
// survives the container being recreated.
import type { LogEntry } from "./logEntry";
import type { HighlightColour } from "./logFilter/rules";

export type LayoutId = "single" | "cols" | "rows" | "grid";

export const LAYOUT_IDS: readonly LayoutId[] = ["single", "cols", "rows", "grid"];
export const PANE_COUNT: Record<LayoutId, number> = { single: 1, cols: 2, rows: 2, grid: 4 };

/** What one pane shows: every ticked container, or an explicit list of names. */
export interface PaneSource {
  all: boolean;
  names: string[];
}

/** A ticked container as the split view sees it. */
export interface TickedItem {
  name: string;
  /** Compose project, null for standalone containers. */
  project: string | null;
}

export const emptyPane = (): PaneSource => ({ all: false, names: [] });
export const allPane = (): PaneSource => ({ all: true, names: [] });
const isEmpty = (p: PaneSource) => !p.all && p.names.length === 0;

/** The ticked names a pane actually shows, in ticked order. */
export function resolveNames(pane: PaneSource, ticked: readonly string[]): string[] {
  if (pane.all) return [...ticked];
  const own = new Set(pane.names);
  return ticked.filter((n) => own.has(n));
}

const byProjectThenName = (a: TickedItem, b: TickedItem) =>
  a.project === b.project ? a.name.localeCompare(b.name) : a.project === null ? 1 : b.project === null ? -1 : a.project.localeCompare(b.project);

export interface ItemGroup {
  project: string | null;
  items: TickedItem[];
}

/** Groups by Compose project (alphabetical, standalone last). */
export function groupItems(items: readonly TickedItem[]): ItemGroup[] {
  const map = new Map<string | null, TickedItem[]>();
  for (const it of [...items].sort(byProjectThenName)) {
    const list = map.get(it.project);
    if (list) list.push(it);
    else map.set(it.project, [it]);
  }
  return [...map.entries()].map(([project, list]) => ({ project, items: list }));
}

/**
 * First assignment when the user splits. Several ticked projects: one project per pane, then the standalone
 * containers one each (only when there are enough of these groups to fill the panes); otherwise one container
 * per pane. Overflow goes into the last pane; panes left over stay empty.
 */
export function defaultDistribution(items: readonly TickedItem[], count: number): PaneSource[] {
  if (count <= 1) return [allPane()];
  const sorted = [...items].sort(byProjectThenName);
  const projects = new Set(sorted.filter((i) => i.project !== null).map((i) => i.project));
  let units: string[][] = sorted.map((i) => [i.name]);
  if (projects.size >= 2) {
    const grouped: string[][] = [];
    for (const g of groupItems(sorted)) {
      if (g.project === null) for (const i of g.items) grouped.push([i.name]);
      else grouped.push(g.items.map((i) => i.name));
    }
    if (grouped.length >= count) units = grouped;
  }
  const panes: PaneSource[] = Array.from({ length: count }, emptyPane);
  units.forEach((u, i) => panes[Math.min(i, count - 1)].names.push(...u));
  return panes;
}

const assigned = (panes: readonly PaneSource[], name: string) => panes.some((p) => p.all || p.names.includes(name));

/**
 * Places newly ticked containers: nothing to do when a pane already shows "all ticked"; otherwise each Compose
 * project goes (together) to the pane with the fewest containers, standalone containers one by one.
 */
export function addTicked(panes: readonly PaneSource[], added: readonly TickedItem[]): { panes: PaneSource[]; placed: Array<{ name: string; pane: number }> } {
  const next = panes.map((p) => ({ all: p.all, names: [...p.names] }));
  const placed: Array<{ name: string; pane: number }> = [];
  if (next.length === 0 || next.some((p) => p.all)) return { panes: next, placed };
  const fresh = added.filter((a) => !assigned(next, a.name));
  const units: string[][] = [];
  for (const g of groupItems(fresh)) {
    if (g.project === null) for (const i of g.items) units.push([i.name]);
    else units.push(g.items.map((i) => i.name));
  }
  for (const unit of units) {
    let best = 0;
    next.forEach((p, i) => {
      if (p.names.length < next[best].names.length) best = i;
    });
    next[best].names.push(...unit);
    for (const name of unit) placed.push({ name, pane: best });
  }
  return { panes: next, placed };
}

const samePanes = (a: readonly PaneSource[], b: readonly PaneSource[]) =>
  a.length === b.length && a.every((p, i) => p.all === b[i].all && p.names.length === b[i].names.length && p.names.every((n, j) => n === b[i].names[j]));

/**
 * Brings the panes in line with the ticked containers: unticked names are dropped, newly ticked ones are placed
 * (`previous` = what was ticked before; null on first load, where everything not yet assigned counts as new and
 * a split that ended up completely empty is distributed afresh). Returns the same array when nothing changed.
 */
export function reconcilePanes(
  panes: PaneSource[],
  ticked: readonly TickedItem[],
  previous: ReadonlySet<string> | null,
): { panes: PaneSource[]; placed: Array<{ name: string; pane: number }> } {
  if (panes.length === 0) return { panes, placed: [] };
  const live = new Set(ticked.map((t) => t.name));
  const pruned = panes.map((p) => ({ all: p.all, names: p.names.filter((n) => live.has(n)) }));
  if (previous === null && ticked.length > 0 && pruned.every(isEmpty)) {
    const fresh = defaultDistribution(ticked, pruned.length);
    return { panes: samePanes(panes, fresh) ? panes : fresh, placed: [] };
  }
  const { panes: next, placed } = addTicked(pruned, ticked.filter((t) => !previous?.has(t.name)));
  return { panes: samePanes(panes, next) ? panes : next, placed };
}

/**
 * Pane list for a layout with `count` panes. Fewer panes than before: the removed panes' containers move into
 * the last remaining one. More panes (or none yet): a fresh default distribution.
 */
export function adaptPanes(panes: PaneSource[], count: number, items: readonly TickedItem[]): PaneSource[] {
  if (panes.length === count) return panes;
  if (panes.length === 0 || panes.length < count) return defaultDistribution(items, count);
  const kept = panes.slice(0, count).map((p) => ({ all: p.all, names: [...p.names] }));
  const last = kept[count - 1];
  for (const p of panes.slice(count)) {
    last.all ||= p.all;
    for (const n of p.names) if (!last.names.includes(n)) last.names.push(n);
  }
  return kept;
}

/** "All ticked containers" switched on or off. Off = nothing selected yet. */
export const setAll = (_p: PaneSource, on: boolean): PaneSource => (on ? allPane() : emptyPane());

/** Ticks or unticks containers in a pane. From "all ticked" the pane turns into the explicit list first. */
export function setNames(pane: PaneSource, ticked: readonly string[], names: readonly string[], on: boolean): PaneSource {
  const current = new Set(pane.all ? ticked : pane.names);
  for (const n of names) {
    if (on) current.add(n);
    else current.delete(n);
  }
  return { all: false, names: ticked.filter((n) => current.has(n)) };
}

export interface PaneSummary {
  names: string[];
  label: string;
  /** The one container shown, when exactly one. */
  single: string | null;
}

/** Short description of what a pane shows, for its header. */
export function describePane(pane: PaneSource, items: readonly TickedItem[]): PaneSummary {
  const names = resolveNames(pane, items.map((i) => i.name));
  if (names.length === 0) return { names, label: "Nothing selected", single: null };
  if (names.length === 1) return { names, label: names[0], single: names[0] };
  if (pane.all) return { names, label: "All ticked containers", single: null };
  const shown = new Set(names);
  for (const g of groupItems(items)) {
    if (g.project !== null && g.items.length === shown.size && g.items.every((i) => shown.has(i.name))) return { names, label: g.project, single: null };
  }
  return { names, label: `${names.length} containers`, single: null };
}

export interface PaneLines {
  entries: LogEntry[];
  highlights: Array<HighlightColour | undefined>;
}

/**
 * The lines of one pane out of the globally filtered stream (`highlights` runs parallel to `visible`).
 * `names` null = every container. With neither a name set nor a text test the inputs are returned as they are.
 */
export function selectPaneLines(
  visible: LogEntry[],
  highlights: Array<HighlightColour | undefined>,
  names: ReadonlySet<string> | null,
  test: ((text: string) => boolean) | null,
): PaneLines {
  if (names === null && test === null) return { entries: visible, highlights };
  const entries: LogEntry[] = [];
  const marks: Array<HighlightColour | undefined> = [];
  for (let i = 0; i < visible.length; i++) {
    const e = visible[i];
    if (names !== null && !names.has(e.container)) continue;
    if (test !== null && !test(e.text)) continue;
    entries.push(e);
    marks.push(highlights[i]);
  }
  return { entries, highlights: marks };
}

/** True when two selections hold the very same lines with the same highlights (so a pane need not re-render). */
export function sameLines(a: PaneLines, b: PaneLines): boolean {
  if (a === b) return true;
  if (a.entries.length !== b.entries.length) return false;
  for (let i = 0; i < a.entries.length; i++) if (a.entries[i] !== b.entries[i] || a.highlights[i] !== b.highlights[i]) return false;
  return true;
}
