import type { Container, PortBinding } from "@bindings/model";

export function formatBytes(n: number): string {
  if (n < 1024) return `${n} B`;
  const units = ["KB", "MB", "GB", "TB"];
  let v = n / 1024;
  let i = 0;
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024;
    i++;
  }
  return `${v >= 100 ? v.toFixed(0) : v.toFixed(1)} ${units[i]}`.replace(".0 ", " ");
}

/** "127.0.0.1:8080 → 80/tcp" for published ports, "80/tcp" for exposed-only. */
export function formatPortLong(p: PortBinding): string {
  const target = `${p.containerPort}/${p.protocol || "tcp"}`;
  if (!p.hostPort) return target;
  const ip = p.hostIp?.includes(":") && !p.hostIp.startsWith("[") ? `[${p.hostIp}]` : p.hostIp;
  return `${ip ? `${ip}:` : ""}${p.hostPort} → ${target}`;
}

function portLabel(p: PortBinding): string {
  return `${p.hostPort}:${p.containerPort}${p.protocol && p.protocol !== "tcp" ? `/${p.protocol}` : ""}`;
}

/** Published bindings with their compact label (e.g. "8080:80"). Duplicate v4/v6 bindings collapse. */
export function publishedBindings(ports: readonly PortBinding[]): { label: string; binding: PortBinding }[] {
  const seen = new Set<string>();
  const out: { label: string; binding: PortBinding }[] = [];
  for (const p of ports) {
    if (!p.hostPort) continue;
    const label = portLabel(p);
    if (!seen.has(label)) {
      seen.add(label);
      out.push({ label, binding: p });
    }
  }
  return out;
}

/** Compact published mapping labels for the table. */
export function publishedPorts(ports: readonly PortBinding[]): string[] {
  return publishedBindings(ports).map((x) => x.label);
}

/** Dedupe long-form ports (IPv4 + IPv6 mappings of the same host port). */
export function uniquePortsLong(ports: readonly PortBinding[]): string[] {
  return uniquePortBindings(ports).map((x) => x.label);
}

/** Same as uniquePortsLong but keeps the binding so a row can be made clickable. */
export function uniquePortBindings(ports: readonly PortBinding[]): { label: string; binding: PortBinding }[] {
  const seen = new Set<string>();
  const out: { label: string; binding: PortBinding }[] = [];
  for (const p of ports) {
    const label = formatPortLong(p);
    if (!seen.has(label)) {
      seen.add(label);
      out.push({ label, binding: p });
    }
  }
  return out;
}

/** Uptime text from the Engine status ("Up 2 hours (healthy)" -> "2 hours"); undefined when not running. */
export function uptimeLabel(c: Pick<Container, "state" | "status">): string | undefined {
  if (c.state !== "running" && c.state !== "paused") return undefined;
  const m = /^Up\s+(.*?)(\s*\((?:Paused|healthy|unhealthy|health:.*)\))*$/i.exec(c.status.trim());
  return m?.[1] || undefined;
}

/** Local "Jan 5, 2026, 10:42" style text shared by every created/started column. */
export function formatDateTime(d: Date): string {
  return d.toLocaleString(undefined, { year: "numeric", month: "short", day: "numeric", hour: "2-digit", minute: "2-digit" });
}

export function formatCreated(unixSeconds: number): string {
  return unixSeconds ? formatDateTime(new Date(unixSeconds * 1000)) : "—";
}

export function shortId(id: string): string {
  return id.slice(0, 12);
}

export const STATE_FILTER_OPTIONS = ["running", "exited", "created", "paused", "restarting", "removing", "dead"] as const;
