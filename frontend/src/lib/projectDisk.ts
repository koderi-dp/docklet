// Pure helpers for the per-project disk figures (Engine disk accounting).
import type { ComposeProjectDisk } from "@bindings/model";

export interface ProjectData {
  bytes: number;
  /** Set when a part is missing or only partly reported, so the sum is a lower bound. */
  note?: string;
}

/**
 * "Project data" = container writable layers + volumes. Images are never added: they are usually shared.
 * Null when the Engine reported no size for either part (never 0 for unknown).
 */
export function projectData(d: ComposeProjectDisk | undefined): ProjectData | null {
  if (!d || (d.containersBytes == null && d.volumesBytes == null)) return null;
  const parts: string[] = [];
  if (d.containersBytes == null) parts.push("no container size reported");
  else if (d.containersKnown < d.containerCount) parts.push(`${d.containersKnown} of ${d.containerCount} containers report a size`);
  if (d.volumesBytes == null) parts.push(d.volumeCount > 0 ? "no volume size reported" : "");
  else if (d.volumesKnown < d.volumeCount) parts.push(`${d.volumesKnown} of ${d.volumeCount} volumes report a size`);
  const note = parts.filter(Boolean).join("; ");
  return { bytes: (d.containersBytes ?? 0) + (d.volumesBytes ?? 0), note: note || undefined };
}
