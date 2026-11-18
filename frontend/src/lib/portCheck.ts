import { Status, type Entry, type Holder, type PortCheck, type Request } from "@bindings/ports";
import type { PortRow } from "@/components/containers/createContainerForm";

export const isInUse = (c: PortCheck | undefined): c is PortCheck => c?.status === Status.StatusInUse;

/** Host port rows that name a fixed host port, as check requests (row index kept for display). */
export function requestsFromRows(rows: readonly PortRow[]): { index: number; request: Request }[] {
  const out: { index: number; request: Request }[] = [];
  rows.forEach((r, index) => {
    const port = Number(r.hostPort.trim());
    if (/^\d+$/.test(r.hostPort.trim()) && port >= 1 && port <= 65535) {
      out.push({ index, request: { hostIp: r.hostIp.trim(), hostPort: port, protocol: r.protocol || "tcp" } });
    }
  });
  return out;
}

/** "nginx.exe (PID 4321)" / "container shop-web-1 (project shop)" — who holds it, without the port. */
export function holderText(h: Holder): string {
  if (h.kind === "container") return `container ${h.containerName}${h.composeProject ? ` (project ${h.composeProject})` : ""}`;
  const name = h.processName || "a program that could not be identified";
  const where = h.location && h.location !== "This machine" ? ` on ${h.location}` : "";
  return `${h.kind === "system" ? "the system: " : ""}${name}${h.pid > 0 ? ` (PID ${h.pid})` : ""}${where}`;
}

/** One line for an inline state: what the check found, in the words of the design. */
export function checkText(c: PortCheck): string {
  const port = c.protocol === "tcp" ? String(c.hostPort) : `${c.hostPort}/${c.protocol}`;
  if (c.status === Status.StatusFree) return `${port} is free`;
  if (c.status === Status.StatusInUse && c.holder) return `${port} is used by ${holderText(c.holder)}`;
  return c.detail || "Could not check this port";
}

const PORT_ERROR_RE = /port is already allocated|address already in use|ports are not available|only one usage of each socket address/i;
const PORT_IN_ERROR_RE = /(?:\b\d{1,3}(?:\.\d{1,3}){3}|\[[0-9a-f:]*\]|\*|\b[a-z0-9]+):(\d{2,5})\b/gi;

/** The host ports named in a "port is already allocated" style Engine error, or [] when it is another error. */
export function conflictingPortsInError(message: string): number[] {
  if (!PORT_ERROR_RE.test(message)) return [];
  const ports = new Set<number>();
  for (const m of message.matchAll(PORT_IN_ERROR_RE)) {
    const n = Number(m[1]);
    if (n >= 1 && n <= 65535) ports.add(n);
  }
  return [...ports];
}

/** Everything the search box matches: port, container, program, PID, path, owner and place. */
function entryHaystack(e: Entry): string {
  const h = e.holder;
  return [e.hostPort, e.protocol, e.hostIp, h.containerName, h.composeProject, h.processName, h.pid || "", h.executablePath, h.user, h.location].join(" ").toLowerCase();
}

/** Entries matching every word of the query (case-insensitive). */
export function filterEntries(entries: Entry[], query: string): Entry[] {
  const terms = query.toLowerCase().split(/\s+/).filter(Boolean);
  return terms.length === 0 ? entries : entries.filter((e) => terms.every((t) => entryHaystack(e).includes(t)));
}
