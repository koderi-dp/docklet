// Pure row model of the Compose Services tab: one row per container (or one for a service
// that has none yet), plus the Running / Stopped filter over those rows.
import type { Container, ImageRow } from "@bindings/model";

export type ServiceFilter = "all" | "running" | "stopped";

export interface ServiceRow {
  key: string;
  /** Compose service name; "" for containers that carry no service label. */
  service: string;
  /** Undefined for a service defined in the Compose files that has no container yet. */
  container?: Container;
  /** 1-based position of this container among the containers of its service. */
  replica: number;
  /** Containers of the service (at least 1: a service without container still counts one row). */
  replicas: number;
  running: boolean;
}

function byName(a: Container, b: Container): number {
  return a.name.localeCompare(b.name, undefined, { numeric: true });
}

/**
 * Rows grouped by service: `serviceNames` first in their order, then services only known from
 * containers (in first-seen order), then containers without a service label under "".
 */
export function buildRows(serviceNames: readonly string[], containers: readonly Container[]): ServiceRow[] {
  const by = new Map<string, Container[]>();
  for (const c of containers) {
    const k = c.composeService || "";
    const list = by.get(k);
    if (list) list.push(c);
    else by.set(k, [c]);
  }
  const names = [...new Set([...serviceNames.filter((n) => n !== ""), ...by.keys()])];
  const ordered = [...names.filter((n) => n !== ""), ...names.filter((n) => n === "")];
  const rows: ServiceRow[] = [];
  for (const service of ordered) {
    const list = [...(by.get(service) ?? [])].sort(byName);
    if (list.length === 0) {
      rows.push({ key: `service:${service}`, service, container: undefined, replica: 1, replicas: 1, running: false });
      continue;
    }
    list.forEach((container, i) => {
      rows.push({ key: container.id, service, container, replica: i + 1, replicas: list.length, running: container.state === "running" });
    });
  }
  return rows;
}

/**
 * Size in bytes of the image a container runs. The container list carries only the image
 * reference the Engine reports: an image ID (when the tag moved or never existed) or a name.
 */
export function imageSizeFor(container: Pick<Container, "image"> & { imageId?: string }, images: readonly Pick<ImageRow, "id" | "repository" | "tag" | "size">[]): number | undefined {
  const strip = (s: string) => s.replace(/^sha256:/, "");
  if (container.imageId) {
    const id = strip(container.imageId);
    return images.find((i) => strip(i.id) === id)?.size;
  }
  const ref = container.image;
  if (!ref) return undefined;
  const bare = strip(ref);
  const byId = /^[0-9a-f]{12,64}$/.test(bare) ? images.find((i) => strip(i.id).startsWith(bare)) : undefined;
  const byTag = images.find((i) => i.repository && (`${i.repository}:${i.tag}` === ref || (i.tag === "latest" && i.repository === ref)));
  return (byId ?? byTag)?.size;
}

export interface RowCounts {
  all: number;
  running: number;
  stopped: number;
}

/** Stopped is every row that is not running: exited, completed, created and not created. */
export function countRows(rows: readonly ServiceRow[]): RowCounts {
  const running = rows.filter((r) => r.running).length;
  return { all: rows.length, running, stopped: rows.length - running };
}

export function filterRows(rows: readonly ServiceRow[], filter: ServiceFilter): ServiceRow[] {
  if (filter === "all") return [...rows];
  return rows.filter((r) => r.running === (filter === "running"));
}
