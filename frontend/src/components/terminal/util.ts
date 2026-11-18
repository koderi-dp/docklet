import type { ITheme } from "@xterm/xterm";

const encoder = new TextEncoder();

/** UTF-8 string -> base64 (what ExecService.Write expects). */
export function toBase64(s: string): string {
  const bytes = encoder.encode(s);
  let bin = "";
  for (let i = 0; i < bytes.length; i += 0x8000) {
    bin += String.fromCharCode(...bytes.subarray(i, i + 0x8000));
  }
  return btoa(bin);
}

/** base64 -> raw bytes (xterm decodes UTF-8 itself, so chunk splits are safe). */
export function fromBase64(b64: string): Uint8Array {
  const bin = atob(b64);
  const out = new Uint8Array(bin.length);
  for (let i = 0; i < bin.length; i++) out[i] = bin.charCodeAt(i);
  return out;
}

function cssVar(name: string, fallback = ""): string {
  const v = getComputedStyle(document.documentElement).getPropertyValue(name).trim();
  return v || fallback;
}

/** xterm theme from the app's CSS variables, so light and dark both work. */
export function readTerminalTheme(): ITheme {
  const bg = cssVar("--card", cssVar("--background"));
  const fg = cssVar("--foreground");
  const primary = cssVar("--primary");
  const hex7 = /^#[0-9a-fA-F]{6}$/.test(primary);
  const success = cssVar("--status-success") || undefined;
  const warning = cssVar("--status-warning") || undefined;
  const danger = cssVar("--destructive") || undefined;
  return {
    background: bg,
    foreground: fg,
    cursor: primary || fg,
    cursorAccent: bg,
    selectionBackground: hex7 ? `${primary}66` : undefined,
    green: success,
    brightGreen: success,
    yellow: warning,
    brightYellow: warning,
    red: danger,
    brightRed: danger,
    blue: primary || undefined,
    brightBlue: primary || undefined,
    brightBlack: cssVar("--muted-foreground") || undefined,
  };
}

export function readMonoFont(): string {
  return cssVar("--font-mono", "ui-monospace, Consolas, monospace");
}
