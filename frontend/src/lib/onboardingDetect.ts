import { WSLInstallState, type EnvironmentReport, type LocalEngineReport, type WSLDistroReport } from "@bindings/model";
import { typeLabel } from "@/components/connections/connectionMeta";
import { engineVersionLabel, isPodmanKind } from "@/lib/engineKind";
import { rankCandidate, type Candidate } from "./onboardingCandidates";

/** How a checklist row ended: found and working, found but needs attention, or not there. */
export type RowTone = "ok" | "warn" | "none";

export interface DetectRow {
  id: string;
  title: string;
  tone: RowTone;
  detail: string;
  /** One line per item under the row (the WSL distributions). */
  items?: DetectItem[];
}

export interface DetectItem {
  label: string;
  detail: string;
  tone: RowTone;
}

const plural = (n: number, one: string, many = `${one}s`) => `${n} ${n === 1 ? one : many}`;

function distroItem(d: WSLDistroReport): DetectItem {
  const k = d.docker;
  const label = `${d.name} · WSL ${d.version}${d.default ? " · default" : ""}`;
  if (d.internal) return { label, detail: "Internal distribution of Docker Desktop or a Podman machine", tone: "none" };
  switch (k.state) {
    case WSLInstallState.WSLStateStopped:
      return { label, detail: "Stopped, not started to look for Docker", tone: "none" };
    case WSLInstallState.WSLStateRunning:
      return { label, detail: `Docker Engine ${k.version ?? ""} running`.replace("  ", " "), tone: "ok" };
    case WSLInstallState.WSLStateDockerDesktop:
      return k.reachable
        ? { label, detail: `Docker ${k.version ?? ""} through Docker Desktop's integration`.replace("  ", " "), tone: "ok" }
        : { label, detail: "Docker Desktop integration, Docker Desktop is not running", tone: "warn" };
    case WSLInstallState.WSLStateDaemonStopped:
      return { label, detail: "Docker Engine installed, not running", tone: "warn" };
    case WSLInstallState.WSLStateNotInstalled:
      return { label, detail: k.supported ? "No Docker Engine, Docklet can install it" : "No Docker Engine", tone: "none" };
    default:
      return { label, detail: k.detail || "Docker state could not be determined", tone: "warn" };
  }
}

function wslRow(r: EnvironmentReport): DetectRow {
  const w = r.wsl;
  if (!w.available) return { id: "wsl", title: "Windows Subsystem for Linux", tone: "none", detail: w.detail || "No distributions found" };
  const distros = w.distros ?? [];
  const real = distros.filter((d) => !d.internal);
  return {
    id: "wsl",
    title: "Windows Subsystem for Linux",
    tone: "ok",
    detail: `${plural(real.length, "distribution")} found`,
    items: distros.map(distroItem),
  };
}

function localRow(l: LocalEngineReport): DetectRow {
  const where = typeLabel(l.profile.type);
  const id = `local:${l.profile.endpoint}`;
  if (l.stopped) return { id, title: l.profile.name, tone: "warn", detail: `${where} · Stopped. ${l.error ?? ""}`.trim() };
  if (l.reachable) {
    const engine = isPodmanKind(l.kind) ? engineVersionLabel(l.kind, l.serverVersion) : `Docker Engine ${l.serverVersion ?? ""}`.trim();
    return { id, title: l.profile.name, tone: "ok", detail: `${where} · ${engine} answers` };
  }
  return { id: `local:${l.profile.endpoint}`, title: l.profile.name, tone: "none", detail: `${where} · ${l.error || "does not answer"}` };
}

/** The checklist shown while detecting, one row per kind of thing looked at. Facts only. */
export function detectRows(r: EnvironmentReport): DetectRow[] {
  const rows: DetectRow[] = [];
  if (r.os === "windows") rows.push(wslRow(r));
  rows.push(...(r.local ?? []).map(localRow));
  const ctx = r.contexts ?? [];
  rows.push({
    id: "contexts",
    title: "Docker CLI contexts",
    tone: ctx.length ? "ok" : "none",
    detail: ctx.length ? `${plural(ctx.length, "context")}: ${ctx.map((c) => c.name + (c.current ? " (current)" : "")).join(", ")}` : "None found",
  });
  const saved = r.saved ?? [];
  if (saved.length) rows.push({ id: "saved", title: "Saved connections", tone: "ok", detail: `${plural(saved.length, "connection")}: ${saved.map((s) => s.name).join(", ")}` });
  return rows;
}

/** The pending rows shown before the answer arrives (the same titles, no results). */
export function pendingTitles(windows: boolean, hasSaved = false): { id: string; title: string }[] {
  return [
    ...(windows ? [{ id: "wsl", title: "Windows Subsystem for Linux" }] : []),
    { id: "local", title: "Local Docker Engine" },
    { id: "contexts", title: "Docker CLI contexts" },
    ...(hasSaved ? [{ id: "saved", title: "Saved connections" }] : []),
  ];
}

/** The short result under the Detect step in the rail. */
export function detectSummary(candidates: Candidate[]): { text: string; attention: boolean } {
  if (!candidates.length) return { text: "Nothing found", attention: true };
  const running = candidates.filter((c) => rankCandidate(c) === 0).length;
  const rest = candidates.length - running;
  const parts = [running && `${plural(running, "engine")} running`, rest && `${rest} ${running ? "more" : plural(rest, "option", "options")}`].filter(Boolean);
  return { text: parts.join(", "), attention: false };
}
