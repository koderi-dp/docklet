import type { ImageRow } from "@bindings/model";

export type ImageFilter = "all" | "inuse" | "unused" | "dangling";

export function matchesImage(r: ImageRow, filter: ImageFilter, query: string): boolean {
  if (filter === "inuse" && !r.inUse) return false;
  if (filter === "unused" && r.inUse) return false;
  if (filter === "dangling" && !r.dangling) return false;
  const q = query.trim().toLowerCase();
  if (!q) return true;
  return r.ref.toLowerCase().includes(q) || r.id.toLowerCase().replace("sha256:", "").includes(q.replace("sha256:", ""));
}

export interface ImageSummary {
  /** Distinct images (not tag rows). */
  images: number;
  tagRows: number;
  /** Sum of the reported size of distinct images; shared layers are counted per image. */
  totalSize: number;
  unused: number;
  dangling: number;
}

export function summarizeImages(rows: readonly ImageRow[]): ImageSummary {
  const seen = new Map<string, ImageRow>();
  for (const r of rows) if (!seen.has(r.id)) seen.set(r.id, r);
  let totalSize = 0;
  let unused = 0;
  for (const r of seen.values()) {
    totalSize += r.size;
    if (!r.inUse) unused++;
  }
  return {
    images: seen.size,
    tagRows: rows.length,
    totalSize,
    unused,
    dangling: rows.filter((r) => r.dangling).length,
  };
}

/** Client-side shape check mirroring the server; the server stays authoritative. */
export function quickRefProblem(ref: string): string | null {
  const v = ref.trim();
  if (!v) return "Enter an image reference, e.g. nginx:1.27";
  if (/\s/.test(v)) return "A reference cannot contain spaces";
  if (!/^[a-zA-Z0-9][\w.\-:/@]*$/.test(v)) return "Invalid characters in reference";
  if (v.endsWith(":") || v.endsWith("/")) return "Reference is incomplete";
  return null;
}
