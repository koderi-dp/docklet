// Which containers the Logs screen offers and which are ticked when it opens. Pure.
// Grouping is by the exact com.docker.compose.project label only (Container.composeProject).
import type { Container } from "@bindings/model";

/** Containers ticked when the screen opens. */
export const DEFAULT_SELECTION_CAP = 12;
/** Most containers streamed at once. */
export const MAX_STREAMS = 40;

export interface SourceGroup {
  /** Compose project name, null for containers that are not part of a project. */
  project: string | null;
  containers: Container[];
}

const byName = (a: Container, b: Container) => a.name.localeCompare(b.name);

function matches(c: Container, q: string): boolean {
  return [c.name, c.composeProject, c.composeService, c.image].some((v) => !!v && v.toLowerCase().includes(q));
}

/**
 * Groups containers by Compose project (alphabetical, standalone last).
 * Running-only keeps every running container plus any container in `keep` (ticked ones stay visible).
 */
export function groupSources(
  containers: readonly Container[],
  opts: { search: string; runningOnly: boolean; keep?: ReadonlySet<string> },
): SourceGroup[] {
  const q = opts.search.trim().toLowerCase();
  const groups = new Map<string | null, Container[]>();
  for (const c of containers) {
    if (opts.runningOnly && c.state !== "running" && !opts.keep?.has(c.id)) continue;
    if (q && !matches(c, q)) continue;
    const key = c.composeProject ? c.composeProject : null;
    const list = groups.get(key);
    if (list) list.push(c);
    else groups.set(key, [c]);
  }
  return [...groups.entries()]
    .map(([project, list]) => ({ project, containers: list.sort(byName) }))
    .sort((a, b) => (a.project === null ? 1 : b.project === null ? -1 : a.project.localeCompare(b.project)));
}

/** The running containers in display order, limited to `cap`; `total` is how many are running. */
export function defaultSelection(containers: readonly Container[], cap = DEFAULT_SELECTION_CAP): { ids: string[]; total: number } {
  const running = groupSources(containers, { search: "", runningOnly: true }).flatMap((g) => g.containers);
  return { ids: running.slice(0, cap).map((c) => c.id), total: running.length };
}
