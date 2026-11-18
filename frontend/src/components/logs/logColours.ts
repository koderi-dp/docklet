import type { HighlightColour } from "@/lib/logFilter/rules";
import type { LogLevel } from "@/lib/logLevel";

// Container tag colours: the --log-tag-* tokens from globals.css (eight hues spaced apart, tuned per theme).
// Class strings are written out in full so Tailwind can see them.
const TONES = [
  { text: "text-log-tag-1", dot: "bg-log-tag-1" },
  { text: "text-log-tag-2", dot: "bg-log-tag-2" },
  { text: "text-log-tag-3", dot: "bg-log-tag-3" },
  { text: "text-log-tag-4", dot: "bg-log-tag-4" },
  { text: "text-log-tag-5", dot: "bg-log-tag-5" },
  { text: "text-log-tag-6", dot: "bg-log-tag-6" },
  { text: "text-log-tag-7", dot: "bg-log-tag-7" },
  { text: "text-log-tag-8", dot: "bg-log-tag-8" },
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
