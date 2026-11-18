import type { ContainerCreateSpec } from "@bindings/model";

export interface EnvRow {
  key: string;
  value: string;
  /** Edit mode: the value is a secret that stays on the server and is kept as is. */
  keep?: boolean;
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

/** All editable fields of the container form, as the strings the inputs hold. */
export interface FormState {
  image: string;
  name: string;
  command: string;
  env: EnvRow[];
  ports: PortRow[];
  mounts: MountRow[];
  restart: string;
  maxRetries: string;
  memory: string;
  cpus: string;
  autoStart: boolean;
}

export const emptyForm = (image = ""): FormState => ({
  image,
  name: "",
  command: "",
  env: [],
  ports: [],
  mounts: [],
  restart: "no",
  maxRetries: "",
  memory: "",
  cpus: "",
  autoStart: true,
});

/** Form fields for an existing spec (edit mode). */
export function formFromSpec(s: ContainerCreateSpec): FormState {
  return {
    image: s.image,
    name: s.name,
    command: s.command,
    env: (s.env ?? []).map((e) => ({ key: e.key, value: e.keep ? "" : e.value, keep: e.keep })),
    ports: (s.ports ?? []).map((p) => ({
      hostIp: p.hostIp,
      hostPort: p.hostPort ? String(p.hostPort) : "",
      containerPort: String(p.containerPort),
      protocol: p.protocol || "tcp",
    })),
    mounts: (s.mounts ?? []).map((m) => ({ type: m.type === "bind" ? "bind" : "volume", source: m.source, target: m.target, readOnly: m.readOnly })),
    restart: s.restartPolicy || "no",
    maxRetries: s.maxRetries ? String(s.maxRetries) : "",
    memory: s.memoryMiB ? String(s.memoryMiB) : "",
    cpus: s.cpus ? String(s.cpus) : "",
    autoStart: s.autoStart,
  };
}

/**
 * The spec for the backend. `base` carries the settings the form does not edit
 * (entrypoint, labels, networks, user...) through unchanged.
 */
export function specFromForm(f: FormState, base?: ContainerCreateSpec): ContainerCreateSpec {
  return {
    entrypoint: "",
    workDir: "",
    user: "",
    hostname: "",
    labels: null,
    stopSignal: "",
    stopTimeout: null,
    networkMode: "",
    networks: null,
    ...base,
    name: f.name.trim(),
    image: f.image.trim(),
    command: f.command.trim(),
    env: f.env
      .filter((e) => e.key.trim())
      .map((e) => ({ key: e.key.trim(), value: e.keep ? "" : e.value, keep: !!e.keep })),
    ports: f.ports
      .filter((p) => p.containerPort.trim())
      .map((p) => ({
        hostIp: p.hostIp.trim(),
        hostPort: p.hostPort.trim() ? Number(p.hostPort) : 0,
        containerPort: Number(p.containerPort),
        protocol: p.protocol,
      })),
    mounts: f.mounts
      .filter((m) => m.target.trim())
      .map((m) => ({ type: m.type, source: m.source.trim(), target: m.target.trim(), readOnly: m.readOnly })),
    restartPolicy: f.restart,
    maxRetries: f.restart === "on-failure" && f.maxRetries.trim() ? Number(f.maxRetries) : 0,
    memoryMiB: f.memory.trim() ? Math.round(Number(f.memory)) : 0,
    cpus: f.cpus.trim() ? Number(f.cpus) : 0,
    autoStart: f.autoStart,
  };
}
