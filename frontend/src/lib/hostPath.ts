/** Connection types whose engine can share files with this computer (the backend still checks the path). */
export function engineMayBeLocal(type: string | undefined): boolean {
  return type === "npipe" || type === "unix" || type === "wsl";
}

/** The file name of an engine path (either separator). */
export function baseName(p: string): string {
  return p.split(/[\\/]/).filter(Boolean).pop() ?? p;
}
