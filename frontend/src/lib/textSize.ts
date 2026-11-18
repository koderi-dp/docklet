/** Whole-window zoom steps, in percent. */
export const TEXT_SIZES = [90, 100, 110, 125] as const;
export type TextSize = (typeof TEXT_SIZES)[number];
export const DEFAULT_TEXT_SIZE: TextSize = 100;

/** Any stored value becomes one of the allowed steps: the nearest one, or 100 for non-numbers. */
export function normalizeTextSize(value: unknown): TextSize {
  if (typeof value !== "number" || !Number.isFinite(value)) return DEFAULT_TEXT_SIZE;
  return TEXT_SIZES.reduce((best, s) => (Math.abs(s - value) < Math.abs(best - value) ? s : best));
}

/** The next larger (+1) or smaller (-1) step; stays at the end of the range. */
export function stepTextSize(current: number, direction: 1 | -1): TextSize {
  const i = TEXT_SIZES.indexOf(normalizeTextSize(current));
  return TEXT_SIZES[Math.min(TEXT_SIZES.length - 1, Math.max(0, i + direction))];
}

export type TextSizeShortcut = "larger" | "smaller" | "reset";

/** Ctrl/Cmd with "=" or "+" (larger), "-" (smaller), "0" (reset). Anything else is not ours. */
export function textSizeShortcut(e: Pick<KeyboardEvent, "key" | "ctrlKey" | "metaKey" | "altKey">): TextSizeShortcut | null {
  if (e.altKey || !(e.ctrlKey || e.metaKey)) return null;
  if (e.key === "=" || e.key === "+") return "larger";
  if (e.key === "-" || e.key === "_") return "smaller";
  if (e.key === "0") return "reset";
  return null;
}
