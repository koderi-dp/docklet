/** Operating system of the webview, for platform-specific UI (shortcut labels, Windows-only connection types). */
const ua = typeof navigator === "undefined" ? "" : navigator.userAgent;

export const isWindows = ua.includes("Windows");
export const isMac = ua.includes("Macintosh") || ua.includes("Mac OS X");

/** Label of the primary shortcut modifier. */
export const modKeyLabel = isMac ? "⌘" : "Ctrl";

/** Shortcut text like "Ctrl+K" or "⌘K". */
export function shortcutLabel(key: string): string {
  return isMac ? `⌘${key}` : `Ctrl+${key}`;
}
