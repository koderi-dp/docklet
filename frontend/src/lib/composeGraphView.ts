// Pure helpers for the Compose service graph: classification, state, labels, relations.
import { isCompleted, isFailed, isRunning } from "@/lib/composeGrouping";

export type GraphKind = "web" | "database" | "cache" | "queue" | "image";

const DATABASES = [
  "postgres", "postgresql", "mysql", "mariadb", "mongo", "mssql", "cockroach",
  "clickhouse", "timescale", "influxdb", "elasticsearch", "opensearch",
];
const CACHES = ["redis", "valkey", "memcached", "keydb", "dragonfly"];
const QUEUES = ["rabbitmq", "kafka", "nats", "redpanda", "mosquitto"];

function repositoryName(image: string): string {
  const noDigest = image.split("@")[0];
  const last = noDigest.split("/").pop() ?? "";
  return last.split(":")[0].toLowerCase();
}

function matchesAny(repo: string, names: string[]): boolean {
  return names.some((n) => repo === n || repo.startsWith(`${n}-`));
}

export function serviceKind(s: { image?: string; ports: unknown[] }): GraphKind {
  const repo = repositoryName(s.image ?? "");
  if (matchesAny(repo, DATABASES)) return "database";
  if (matchesAny(repo, CACHES)) return "cache";
  if (matchesAny(repo, QUEUES)) return "queue";
  return s.ports.length > 0 ? "web" : "image";
}

const KIND_LABELS: Record<GraphKind, string> = {
  web: "Web",
  database: "Database",
  cache: "Cache",
  queue: "Queue",
  image: "Image",
};
export function kindLabel(k: GraphKind): string {
  return KIND_LABELS[k];
}

export type GraphState =
  | "running" | "unhealthy" | "starting" | "stopped"
  | "failed" | "completed" | "partial" | "paused" | "notCreated";

export interface StateContainer {
  state: string;
  health?: string | { status?: string } | null;
  exitCode?: number | null;
  restartPolicy?: string;
}

function healthOf(c: StateContainer): string {
  const h = c.health;
  if (!h) return "";
  return typeof h === "string" ? h : (h.status ?? "");
}

export function serviceState(containers: StateContainer[]): GraphState {
  if (containers.length === 0) return "notCreated";
  const cs = containers as { state: string; exitCode?: number | null; restartPolicy?: string }[];
  const running = cs.filter((c) => isRunning(c)).length;
  const restarting = cs.some((c) => c.state === "restarting");
  if (running === 0) {
    if (cs.every((c) => isCompleted(c as Parameters<typeof isCompleted>[0]))) return "completed";
    if (restarting || cs.some((c) => isFailed(c as Parameters<typeof isFailed>[0]))) return "failed";
    if (cs.every((c) => c.state === "paused")) return "paused";
    return "stopped";
  }
  if (running < cs.length) return "partial";
  const healths = containers.map(healthOf);
  if (healths.includes("unhealthy")) return "unhealthy";
  if (healths.includes("starting")) return "starting";
  return "running";
}

const STATE_LABELS: Record<GraphState, string> = {
  running: "Up",
  unhealthy: "Unhealthy",
  starting: "Starting",
  stopped: "Stopped",
  failed: "Failed",
  completed: "Completed",
  partial: "Partial",
  paused: "Paused",
  notCreated: "Not created",
};
export function stateLabel(s: GraphState): string {
  return STATE_LABELS[s];
}

export function portLabel(p: { hostIp?: string; published: string; target: number; protocol?: string }): string {
  const suffix = p.protocol === "udp" ? "/udp" : "";
  return p.published ? `${p.published}→${p.target}${suffix}` : `${p.target}${suffix}`;
}

/** Last segment of a bind path; a named volume has no separator and stays as is. */
export function volumeLabel(source: string): string {
  if (!/[\\/]/.test(source)) return source;
  const parts = source.split(/[\\/]+/).filter(Boolean);
  return parts[parts.length - 1] ?? source;
}

/** The service, everything it needs and everything that needs it, transitively (no siblings). */
export function related(name: string, edges: { from: string; to: string }[]): Set<string> {
  const out = new Set<string>([name]);
  for (const [a, b] of [["from", "to"], ["to", "from"]] as const) {
    const next = new Map<string, string[]>();
    for (const e of edges) next.set(e[a], [...(next.get(e[a]) ?? []), e[b]]);
    const stack = [name];
    const seen = new Set<string>([name]);
    while (stack.length) {
      for (const m of next.get(stack.pop()!) ?? []) {
        if (seen.has(m)) continue;
        seen.add(m);
        out.add(m);
        stack.push(m);
      }
    }
  }
  return out;
}

export function matchesFind(s: { name: string; image?: string }, query: string): boolean {
  const q = query.trim().toLowerCase();
  if (!q) return true;
  return s.name.toLowerCase().includes(q) || (s.image ?? "").toLowerCase().includes(q);
}
