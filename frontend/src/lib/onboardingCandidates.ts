import { ConnectionType, WSLInstallState, type ConnectionProfile, type EnvironmentReport, type WSLDistroReport } from "@bindings/model";
import { endpointSummary, typeLabel } from "@/components/connections/connectionMeta";

/** What the setup wizard knows about one engine it could connect to. */
export type CandidateState =
  | "running" //        a daemon answers right now
  | "docker-stopped" // Docker Engine is installed in the distribution but not running
  | "distro-stopped" // the distribution is not running, so Docker was not looked for
  | "no-docker" //      the distribution has no Docker and Docklet can install it
  | "unsupported" //    no Docker and no automatic install here
  | "desktop-down" //   Docker Desktop's integration is there but Docker Desktop is not answering
  | "unchecked"; //     a saved connection the wizard has not tested yet

/** What, if anything, has to happen before the engine can be tested. */
export type SetupNeed = "none" | "start-distro" | "start-docker" | "install-docker" | "install-guide" | "docker-desktop";

export interface Candidate {
  id: string;
  kind: "local" | "wsl" | "saved";
  name: string;
  /** The profile to test and save. Discovered and WSL candidates have no id until they are saved. */
  profile: ConnectionProfile;
  /** Set when a saved connection already points at this engine. */
  savedId?: string;
  state: CandidateState;
  /** Reachable through Docker Desktop's WSL integration instead of an engine inside the distribution. */
  viaDesktop: boolean;
  /** The user's default WSL distribution. */
  isDefault: boolean;
  /** One line of facts for the card, for example "Ubuntu-24.04 · WSL 2 · Docker Engine 29.9 running". */
  facts: string;
  reason?: string;
  docsUrl?: string;
  distro?: string;
}

const RANK: Record<CandidateState, number> = {
  running: 0,
  "docker-stopped": 1,
  "distro-stopped": 2,
  "no-docker": 3,
  unsupported: 4,
  "desktop-down": 4,
  unchecked: 5,
};

/** Lower is better: a running engine, then Docker installed but stopped, then a distribution without Docker. */
export function rankCandidate(c: Candidate): number {
  return RANK[c.state];
}

const KIND_ORDER = { local: 0, wsl: 1, saved: 2 } as const;

/** Best first. Ties go to a real engine over Docker Desktop's proxy, local before WSL, then the default distribution. */
export function sortCandidates(list: Candidate[]): Candidate[] {
  return [...list].sort(
    (a, b) =>
      rankCandidate(a) - rankCandidate(b) ||
      Number(a.viaDesktop) - Number(b.viaDesktop) ||
      KIND_ORDER[a.kind] - KIND_ORDER[b.kind] ||
      Number(b.isDefault) - Number(a.isDefault),
  );
}

const dot = (parts: (string | undefined | false)[]) => parts.filter(Boolean).join(" · ");

function wslFacts(d: WSLDistroReport, state: CandidateState, viaDesktop: boolean): string {
  const head = `${d.name} · WSL ${d.version}`;
  const v = d.docker.version;
  switch (state) {
    case "running":
      return dot([head, viaDesktop ? `Docker ${v ?? ""} through Docker Desktop's integration` : `Docker Engine ${v ?? ""} running`]);
    case "docker-stopped":
      return dot([head, "Docker Engine installed, not running"]);
    case "distro-stopped":
      return dot([head, "Stopped, Docker not checked yet"]);
    case "no-docker":
      return dot([head, "No Docker Engine, Docklet can install it"]);
    case "desktop-down":
      return dot([head, "Docker Desktop integration, Docker Desktop is not running"]);
    default:
      return dot([head, "No Docker Engine, no automatic install for this distribution"]);
  }
}

function wslState(d: WSLDistroReport): CandidateState {
  const k = d.docker;
  if (k.state === WSLInstallState.WSLStateStopped) return "distro-stopped";
  if (k.reachable) return "running";
  switch (k.state) {
    case WSLInstallState.WSLStateDaemonStopped:
      return "docker-stopped";
    case WSLInstallState.WSLStateDockerDesktop:
      return "desktop-down";
    case WSLInstallState.WSLStateNotInstalled:
      return k.supported ? "no-docker" : "unsupported";
    default:
      return "unsupported";
  }
}

function wslCandidate(d: WSLDistroReport, saved: ConnectionProfile[]): Candidate {
  const state = wslState(d);
  const viaDesktop = d.docker.state === WSLInstallState.WSLStateDockerDesktop;
  const match = saved.find((s) => s.type === ConnectionType.ConnWSL && s.wslDistro === d.name);
  return {
    id: `wsl:${d.name}`,
    kind: "wsl",
    name: d.name,
    profile: { id: "", name: `WSL (${d.name})`, type: ConnectionType.ConnWSL, endpoint: "", wslDistro: d.name } as ConnectionProfile,
    savedId: match?.id,
    state,
    viaDesktop,
    isDefault: d.default,
    facts: wslFacts(d, state, viaDesktop),
    reason: d.docker.reason || undefined,
    docsUrl: d.docker.docsUrl || undefined,
    distro: d.name,
  };
}

/** Everything the wizard can offer, best first. Unreachable local endpoints are not offered: the checklist reports them. */
export function buildCandidates(report: EnvironmentReport): Candidate[] {
  const saved = report.saved ?? [];
  const out: Candidate[] = [];
  const claimed = new Set<string>();
  for (const l of report.local ?? []) {
    if (!l.reachable) continue;
    const match = saved.find((s) => s.endpoint.toLowerCase() === l.profile.endpoint.toLowerCase());
    if (match) claimed.add(match.id);
    out.push({
      id: `local:${l.profile.endpoint}`,
      kind: "local",
      name: l.profile.name,
      profile: { ...l.profile, id: "", discovered: false },
      savedId: match?.id,
      state: "running",
      viaDesktop: false,
      isDefault: false,
      facts: dot([typeLabel(l.profile.type), `Docker Engine ${l.serverVersion ?? ""} running`]),
    });
  }
  for (const d of report.wsl?.distros ?? []) {
    if (d.internal) continue;
    const c = wslCandidate(d, saved);
    if (c.savedId) claimed.add(c.savedId);
    out.push(c);
  }
  for (const s of saved) {
    if (claimed.has(s.id)) continue;
    out.push({
      id: `saved:${s.id}`, kind: "saved", name: s.name, profile: s, savedId: s.id, state: "unchecked",
      viaDesktop: false, isDefault: false, facts: dot([typeLabel(s.type), endpointSummary(s)]),
    });
  }
  return sortCandidates(out);
}

/** What has to happen before this engine can be tested. */
export function setupNeed(c: Candidate | undefined): SetupNeed {
  switch (c?.state) {
    case "distro-stopped":
      return "start-distro";
    case "docker-stopped":
      return "start-docker";
    case "no-docker":
      return "install-docker";
    case "unsupported":
      return "install-guide";
    case "desktop-down":
      return "docker-desktop";
    default:
      return "none";
  }
}
