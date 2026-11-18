export interface EnvRow {
  key: string;
  value: string;
}
export interface PortRow {
  hostIp: string;
  hostPort: string;
  containerPort: string;
  protocol: string;
}
export interface MountRow {
  type: "bind" | "volume";
  source: string;
  target: string;
  readOnly: boolean;
}

const NAME_RE = /^[a-zA-Z0-9][a-zA-Z0-9_.-]{0,127}$/;
const ENV_RE = /^[A-Za-z_][A-Za-z0-9_.-]*$/;
const VOL_RE = /^[a-zA-Z0-9][a-zA-Z0-9_.-]*$/;

function parsePortNum(s: string, min: number): number | null {
  if (!/^\d+$/.test(s.trim())) return null;
  const n = Number(s);
  return n >= min && n <= 65535 ? n : null;
}

/** Returns an error message or null; mirrors the server-side rules. */
export function validateForm(f: {
  image: string;
  name: string;
  env: EnvRow[];
  ports: PortRow[];
  mounts: MountRow[];
  memory: string;
  cpus: string;
  maxRetries: string;
}): string | null {
  if (!f.image.trim()) return "Image is required.";
  if (/\s/.test(f.image.trim())) return "Image reference must not contain spaces.";
  if (f.name.trim() && !NAME_RE.test(f.name.trim()))
    return "Name may only contain letters, digits, _ . - and must start with a letter or digit.";
  const keys = new Set<string>();
  for (const e of f.env) {
    if (!e.key.trim() && !e.value) continue;
    if (!ENV_RE.test(e.key.trim())) return `Invalid environment variable name "${e.key}".`;
    if (keys.has(e.key.trim())) return `Environment variable "${e.key}" is set twice.`;
    keys.add(e.key.trim());
  }
  for (const p of f.ports) {
    if (!p.containerPort.trim() && !p.hostPort.trim() && !p.hostIp.trim()) continue;
    if (parsePortNum(p.containerPort, 1) === null) return "Container ports must be between 1 and 65535.";
    if (p.hostPort.trim() && parsePortNum(p.hostPort, 1) === null) return "Host ports must be between 1 and 65535 (or empty).";
  }
  const targets = new Set<string>();
  for (const m of f.mounts) {
    if (!m.source.trim() && !m.target.trim()) continue;
    if (!m.target.trim().startsWith("/")) return `Mount target "${m.target}" must be an absolute container path (start with /).`;
    if (targets.has(m.target.trim())) return `Mount target "${m.target}" is used twice.`;
    targets.add(m.target.trim());
    if (m.type === "bind" && !m.source.trim()) return "Bind mounts need a host path.";
    if (m.type === "volume" && !VOL_RE.test(m.source.trim())) return `Invalid volume name "${m.source}".`;
  }
  if (f.memory.trim()) {
    const n = Number(f.memory);
    if (!Number.isFinite(n) || n < 6) return "Memory limit must be at least 6 MiB (or empty for unlimited).";
  }
  if (f.cpus.trim()) {
    const n = Number(f.cpus);
    if (!Number.isFinite(n) || n <= 0) return "CPU limit must be a positive number (or empty for unlimited).";
  }
  if (f.maxRetries.trim() && !/^\d+$/.test(f.maxRetries.trim())) return "Maximum retries must be a non-negative number.";
  return null;
}

export const emptyPort = (): PortRow => ({ hostIp: "", hostPort: "", containerPort: "", protocol: "tcp" });
