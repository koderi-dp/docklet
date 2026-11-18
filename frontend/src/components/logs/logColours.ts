import type { HighlightColour } from "@/lib/logFilter/rules";
import type { LogLevel } from "@/lib/logLevel";

// Container tag colours: theme tokens only (status colours, primary and mixes of them), so both themes and
// both palettes stay readable. Class strings are written out in full so Tailwind can see them.
const TONES = [
  { text: "text-primary", dot: "bg-primary" },
  { text: "text-info", dot: "bg-info" },
  { text: "text-success", dot: "bg-success" },
  { text: "text-warning", dot: "bg-warning" },
  { text: "text-[color:color-mix(in_oklab,var(--primary),var(--destructive))]", dot: "bg-[color-mix(in_oklab,var(--primary),var(--destructive))]" },
  { text: "text-[color:color-mix(in_oklab,var(--status-info),var(--status-success))]", dot: "bg-[color-mix(in_oklab,var(--status-info),var(--status-success))]" },
  { text: "text-[color:color-mix(in_oklab,var(--status-warning),var(--destructive))]", dot: "bg-[color-mix(in_oklab,var(--status-warning),var(--destructive))]" },
  { text: "text-[color:color-mix(in_oklab,var(--primary),var(--status-info))]", dot: "bg-[color-mix(in_oklab,var(--primary),var(--status-info))]" },
] as const;

const assigned = new Map<string, number>();

/** Colour of a container: assigned in the order containers are first seen, so tones only repeat after all are used. */
export function containerTone(name: string): (typeof TONES)[number] {
  let i = assigned.get(name);
  if (i === undefined) {
    i = assigned.size % TONES.length;
    assigned.set(name, i);
  }
  return TONES[i];
}

/** Line background of a highlight rule. */
export const HIGHLIGHT_BG: Record<HighlightColour, string> = {
  warning: "bg-warning/20",
  success: "bg-success/20",
  info: "bg-info/20",
  primary: "bg-primary/20",
  destructive: "bg-destructive/20",
};

/** Solid swatch of a highlight colour. */
export const HIGHLIGHT_SWATCH: Record<HighlightColour, string> = {
  warning: "bg-warning",
  success: "bg-success",
  info: "bg-info",
  primary: "bg-primary",
  destructive: "bg-destructive",
};

export const HIGHLIGHT_LABEL: Record<HighlightColour, string> = {
  warning: "Amber",
  success: "Green",
  info: "Blue",
  primary: "Violet",
  destructive: "Red",
};

export const LEVEL_TEXT: Record<LogLevel, string> = {
  error: "text-destructive",
  warn: "text-warning",
  info: "text-foreground",
  debug: "text-muted-foreground",
  other: "text-foreground",
};

export const LEVEL_LABEL: Record<LogLevel, string> = { error: "Error", warn: "Warn", info: "Info", debug: "Debug", other: "Other" };

export const LEVEL_DOT: Record<LogLevel, string> = {
  error: "bg-destructive",
  warn: "bg-warning",
  info: "bg-info",
  debug: "bg-muted-foreground/60",
  other: "bg-border",
};
