import type { ComposeLocation, ComposeProject } from "@bindings/model";

/** The last segment of a folder path, whichever slash it uses. */
export function folderName(dir: string): string {
  const parts = dir.trim().split(/[\\/]+/).filter(Boolean);
  return parts.length ? parts[parts.length - 1] : "";
}

/** The project name Compose derives from a folder name (same rule as compose.NormalizeProjectName in Go). */
export function defaultProjectName(dir: string): string {
  return folderName(dir)
    .toLowerCase()
    .replace(/[^a-z0-9_-]/g, "")
    .replace(/^[_-]+/, "");
}

const NAME_RULE = "Use lower-case letters, digits, - and _; start with a letter or digit";

/** Why a project name is not acceptable, or undefined when it is. */
export function nameProblem(name: string): string | undefined {
  return name.length <= 255 && /^[a-z0-9][a-z0-9_-]*$/.test(name) ? undefined : NAME_RULE;
}

export type NameState = { kind: "free" } | { kind: "saved" } | { kind: "takeover"; containers: number; containerIds: string[] };

/**
 * What a project name means on this connection: free, already saved with its files, or a
 * project that only exists as containers and would get this file ("takeover").
 */
export function nameState(projects: readonly ComposeProject[], name: string): NameState {
  const p = projects.find((x) => x.name === name);
  if (!p) return { kind: "free" };
  if (p.managed) return { kind: "saved" };
  const containerIds = [...(p.containerIds ?? [])];
  return { kind: "takeover", containers: containerIds.length, containerIds };
}

export type NewSourceKind = "minimal" | "template" | "containers";

/** The reason Create is disabled, or undefined when the file can be created. */
export function createBlocked(a: {
  location: ComposeLocation | undefined;
  dir: string;
  name: string;
  nameState: NameState;
  source: NewSourceKind;
  ticked: number;
  previewReady: boolean;
}): string | undefined {
  const l = a.location;
  if (!l) return "Checking where Compose runs…";
  if (!l.available) return l.reason || "Compose is not available for this connection";
  if (!l.managedAvailable) return "Saving projects is not available: the application database is not open";
  if (a.dir.trim() === "") return "Choose a folder";
  if (a.name === "") return "Enter a project name";
  const problem = nameProblem(a.name);
  if (problem) return problem;
  if (a.nameState.kind === "saved") return "A project with this name is already saved";
  if (a.source === "containers" && a.ticked === 0) return "Tick at least one container";
  if (!a.previewReady) return "Preparing the preview…";
  return undefined;
}

/** Whether a preview should be requested: never for a bad name or an empty container choice. */
export function previewWanted(a: { name: string; source: NewSourceKind; ticked: number }): boolean {
  if (a.name === "" || nameProblem(a.name)) return false;
  return !(a.source === "containers" && a.ticked === 0);
}

/** Running containers first, each group by name. Returns a new list. */
export function orderContainers<T extends { name: string; state: string }>(containers: readonly T[]): T[] {
  const running = (c: T) => (c.state === "running" ? 0 : 1);
  return [...containers].sort((a, b) => running(a) - running(b) || a.name.localeCompare(b.name));
}

/**
 * The ticked containers that still exist. A container removed while the dialog is open drops out
 * of the selection instead of staying ticked where nobody can untick it. While the list is still
 * loading (undefined) the selection is kept as it is.
 */
export function presentIds(ticked: ReadonlySet<string>, containers: readonly { id: string }[] | undefined): ReadonlySet<string> {
  if (!containers) return ticked;
  const ids = new Set(containers.map((c) => c.id));
  return new Set([...ticked].filter((id) => ids.has(id)));
}
