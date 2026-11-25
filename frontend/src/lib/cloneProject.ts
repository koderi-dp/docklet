import type { ComposeClonePreview, ComposeProject } from "@bindings/model";
import { nameProblem } from "./newComposeFile";

/** Why a project cannot be renamed at all, or undefined when it can. */
export function renameUnavailable(project: Pick<ComposeProject, "managed" | "total">): string | undefined {
  if (!project.managed) return "Only a saved project can be renamed";
  if (project.total > 0)
    return "This project has containers. Compose finds them by the project name, so a renamed project would lose them. Run Down first, or clone the project.";
  return undefined;
}

function nameTaken(projects: readonly Pick<ComposeProject, "name">[], name: string): boolean {
  return projects.some((p) => p.name === name);
}

/** The reason Rename is disabled in the dialog, or undefined when the name can be used. */
export function renameBlocked(a: { current: string; next: string; projects: readonly Pick<ComposeProject, "name">[] }): string | undefined {
  const next = a.next.trim();
  if (next === "") return "Enter a project name";
  if (next === a.current) return "That is the current name";
  const problem = nameProblem(next);
  if (problem) return problem;
  if (nameTaken(a.projects, next)) return "A project with this name already exists";
  return undefined;
}

/** The reason Clone is disabled, or undefined when the copy can start. */
export function cloneBlocked(a: {
  preview: ComposeClonePreview | undefined;
  name: string;
  dir: string;
  projects: readonly Pick<ComposeProject, "name">[];
}): string | undefined {
  if (!a.preview) return "Checking the project…";
  if (a.preview.blocked) return a.preview.blocked;
  const name = a.name.trim();
  if (name === "") return "Enter a project name";
  const problem = nameProblem(name);
  if (problem) return problem;
  if (nameTaken(a.projects, name)) return "A project with this name already exists";
  if (a.dir.trim() === "") return "Choose a folder for the copy";
  return undefined;
}

const UNITS = ["B", "KB", "MB", "GB", "TB"];

/** "3 files, 1.2 MB" for the folder that will be copied. */
export function cloneSizeText(p: Pick<ComposeClonePreview, "sizeKnown" | "sizeBytes" | "files">): string {
  if (!p.sizeKnown) return "The size of the folder could not be measured in time; it may be large.";
  let n = p.sizeBytes;
  let u = 0;
  while (n >= 1024 && u < UNITS.length - 1) {
    n /= 1024;
    u++;
  }
  const size = u === 0 ? `${n} B` : `${n.toFixed(1)} ${UNITS[u]}`;
  return `${p.files.toLocaleString("en-US")} file${p.files === 1 ? "" : "s"}, ${size}`;
}

/** Above this size the dialog points out that the copy takes a while. */
export const LARGE_CLONE_BYTES = 500 * 1024 * 1024;
