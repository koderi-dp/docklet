import type { ContainerCreateSpec, CreateEnvVar, MountSpec, PortMapping } from "@bindings/model";

/** One human-readable difference between the current container and the edited spec. */
export interface SpecChange {
  label: string;
  from: string;
  to: string;
}

const NONE = "—";
const SECRET_KEY = /(PASSWORD|PASS|SECRET|TOKEN|KEY|CREDENTIAL|AUTH|PRIVATE)/i;

const portText = (p: PortMapping) => `${p.hostIp ? `${p.hostIp}:` : ""}${p.hostPort || "auto"} → ${p.containerPort}/${p.protocol || "tcp"}`;
const mountText = (m: MountSpec) => `${m.source || "?"} → ${m.target}${m.readOnly ? " (read-only)" : ""}`;

function setDiff(label: string, before: string[], after: string[]): SpecChange[] {
  const out: SpecChange[] = [];
  for (const b of before) if (!after.includes(b)) out.push({ label: `${label} removed`, from: b, to: NONE });
  for (const a of after) if (!before.includes(a)) out.push({ label: `${label} added`, from: NONE, to: a });
  return out;
}

function envChanges(before: CreateEnvVar[], after: CreateEnvVar[]): SpecChange[] {
  const out: SpecChange[] = [];
  const prev = new Map(before.map((e) => [e.key, e]));
  const next = new Map(after.map((e) => [e.key, e]));
  const show = (e: CreateEnvVar) => (SECRET_KEY.test(e.key) ? "(hidden)" : e.value);
  for (const [k, b] of prev) {
    const a = next.get(k);
    if (!a) out.push({ label: `Env ${k}`, from: b.keep ? "(hidden)" : b.value, to: "removed" });
    else if (a.keep) continue;
    else if (b.keep) out.push({ label: `Env ${k}`, from: "(hidden)", to: "replaced (hidden)" });
    else if (a.value !== b.value) out.push({ label: `Env ${k}`, from: b.value, to: show(a) });
  }
  for (const [k, a] of next) if (!prev.has(k)) out.push({ label: `Env ${k}`, from: NONE, to: show(a) });
  return out;
}

const scalar = (label: string, from: string | number | boolean, to: string | number | boolean): SpecChange[] =>
  from === to ? [] : [{ label, from: String(from) || NONE, to: String(to) || NONE }];

/** Fields the user changed between the original spec and the edited one. */
export function diffSpecs(before: ContainerCreateSpec, after: ContainerCreateSpec): SpecChange[] {
  return [
    ...scalar("Image", before.image, after.image.trim()),
    ...scalar("Name", before.name, after.name.trim()),
    ...scalar("Command", before.command, after.command.trim()),
    ...envChanges(before.env ?? [], after.env ?? []),
    ...setDiff("Port", (before.ports ?? []).map(portText), (after.ports ?? []).map(portText)),
    ...setDiff("Mount", (before.mounts ?? []).map(mountText), (after.mounts ?? []).map(mountText)),
    ...scalar("Restart policy", before.restartPolicy || "no", after.restartPolicy || "no"),
    ...scalar("Memory limit (MiB)", before.memoryMiB, after.memoryMiB),
    ...scalar("CPU limit", before.cpus, after.cpus),
    ...scalar("Start after recreate", before.autoStart ? "yes" : "no", after.autoStart ? "yes" : "no"),
  ];
}

/** True when only the tag/digest of the same repository differs ("nginx:1.26" → "nginx:1.27"). */
export function isTagChange(before: string, after: string): boolean {
  const repo = (r: string) => r.replace(/@.*$/, "").replace(/:[^:/]*$/, "");
  return before !== after && repo(before) === repo(after);
}
