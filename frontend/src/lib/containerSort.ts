// Sorting of the Containers table (flat rows and rows inside Compose groups).
import type { Container, ContainerStat } from "@bindings/model";
import { uptimeLabel } from "@/components/containers/format";
import type { ComposeGroup, FlatRow } from "./composeGrouping";
import { sortItems, type Accessors, type SortState } from "./sorting";
import { uptimeSeconds } from "./uptime";

/** Column accessors; CPU / memory are unknown (last) unless the container runs and a sample exists. */
export function containerAccessors(stats: ReadonlyMap<string, ContainerStat>): Accessors<Container> {
  const running = (c: Container) => (c.state === "running" ? stats.get(c.id) : undefined);
  return {
    name: (c) => c.name,
    image: (c) => c.image,
    project: (c) => c.composeProject,
    status: (c) => c.state,
    health: (c) => (c.health === "none" ? undefined : c.health),
    cpu: (c) => running(c)?.cpuPercent,
    memory: (c) => running(c)?.memoryBytes,
    uptime: (c) => uptimeSeconds(uptimeLabel(c)),
    created: (c) => c.created || undefined,
  };
}

/** Sort containers inside each group; the groups themselves stay ordered by name (Standalone last). */
export function sortGroups(groups: readonly ComposeGroup[], sort: SortState | undefined, stats: ReadonlyMap<string, ContainerStat>): ComposeGroup[] {
  const acc = containerAccessors(stats);
  const withSortedChildren = groups.map((g) => ({ ...g, visible: sortItems(g.visible, sort, acc) }));
  const nameDir = sort?.id === "name" && sort.desc ? -1 : 1;
  return withSortedChildren.sort((a, b) => {
    if (a.kind !== b.kind) return a.kind === "standalone" ? 1 : -1;
    return nameDir * (a.project ?? "").localeCompare(b.project ?? "", undefined, { sensitivity: "base", numeric: true });
  });
}

/** Sort the flat view's rows (all of kind "container"). */
export function sortFlatRows(rows: readonly FlatRow[], sort: SortState | undefined, stats: ReadonlyMap<string, ContainerStat>): FlatRow[] {
  const acc = containerAccessors(stats);
  const byContainer: Accessors<FlatRow> = {};
  for (const [id, get] of Object.entries(acc)) byContainer[id] = (r) => (r.kind === "container" ? get(r.container) : undefined);
  return sortItems(rows, sort, byContainer);
}
