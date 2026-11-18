/** Compose file paths out of the "files-dropped" event payload ({ files: string[] }, possibly array-wrapped). */
export function composeFilesFromDrop(data: unknown): string[] {
  const payload = Array.isArray(data) ? data[0] : data;
  const files = (payload as { files?: unknown } | null | undefined)?.files;
  if (!Array.isArray(files)) return [];
  return files.filter((f): f is string => typeof f === "string" && /\.ya?ml$/i.test(f));
}
