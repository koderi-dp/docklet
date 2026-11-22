import type { ComposeFileRef, ComposeSaveResult } from "@bindings/model";

export interface YamlError {
  line: number;
  message: string;
}

/** What the tab knows about the open file: the text on disk (as loaded or saved) and the text in the editor. */
export interface EditState {
  loadedText: string;
  text: string;
  version: string;
  conflict: boolean;
  error: YamlError | null;
}

export const isDirty = (loadedText: string, currentText: string): boolean => loadedText !== currentText;

/**
 * Folds a save result into the state. `sentText` is the text that was handed to Save; the editor may
 * have moved on since, and that newer text stays dirty.
 */
export function afterSave<S extends EditState>(result: ComposeSaveResult, state: S, sentText: string = state.text): S {
  if (result.saved) return { ...state, loadedText: sentText, version: result.version, conflict: false, error: null };
  if (result.conflict) return { ...state, conflict: true, error: null };
  if (result.error) return { ...state, conflict: false, error: { line: result.error.line, message: result.error.message } };
  return state;
}

/** The wanted path while it is still listed, else the first config file ("" when there is none). */
export function pickFile(files: readonly Pick<ComposeFileRef, "path" | "kind">[], wanted: string): string {
  if (wanted && files.some((f) => f.path === wanted)) return wanted;
  return files.find((f) => f.kind === "config")?.path ?? "";
}

const parts = (path: string) => path.split(/[\\/]/).filter(Boolean);

/** The base name; with `all`, files sharing a base name get their parent folder in front. */
export function fileLabel(path: string, all: readonly string[] = []): string {
  const p = parts(path);
  const base = p[p.length - 1] ?? path;
  const clash = all.some((o) => o !== path && parts(o).slice(-1)[0] === base);
  return clash && p.length > 1 ? `${p[p.length - 2]}/${base}` : base;
}
