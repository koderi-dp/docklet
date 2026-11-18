/** Formatting and small pure helpers shared by the build dialog, progress view and history. */

/** "0.4 s", "41 s", "2 m 05 s", "1 h 03 m". */
export function formatDuration(ms: number): string {
  if (!Number.isFinite(ms) || ms < 0) return "—";
  if (ms < 1000) return `${(ms / 1000).toFixed(1)} s`;
  const s = Math.round(ms / 1000);
  if (s < 60) return `${s} s`;
  const m = Math.floor(s / 60);
  if (m < 60) return `${m} m ${String(s % 60).padStart(2, "0")} s`;
  return `${Math.floor(m / 60)} h ${String(m % 60).padStart(2, "0")} m`;
}

/** "5 of 9 steps · 2 cached · 41 s" (the cached and time parts are omitted when unknown or zero). */
export function summaryLine(done: number, total: number, cached: number, elapsedMs?: number): string {
  const parts = [`${done} of ${total} step${total === 1 ? "" : "s"}`];
  if (cached > 0) parts.push(`${cached} cached`);
  if (elapsedMs !== undefined) parts.push(formatDuration(elapsedMs));
  return parts.join(" · ");
}

/** Mirrors the Go side (buildkit.IsSecretName): names that look like credentials are masked and never stored. */
const SECRET_RE = /(secret|token|passw|pwd|passphrase|credential|auth|private|api[_-]?key|access[_-]?key|(^|[_.-])key($|[_.-]))/i;
export function isSecretName(name: string): boolean {
  return SECRET_RE.test(name);
}

/** Path of file relative to dir (slash-separated), or null when it is outside dir. */
export function relativeInside(dir: string, file: string): string | null {
  const norm = (p: string) => p.replace(/\\/g, "/").replace(/\/+$/, "");
  const d = norm(dir);
  const f = norm(file);
  const windowsLike = /^[a-zA-Z]:/.test(d);
  const dd = windowsLike ? d.toLowerCase() : d;
  const ff = windowsLike ? f.toLowerCase() : f;
  if (!ff.startsWith(dd + "/")) return null;
  return f.slice(d.length + 1);
}
