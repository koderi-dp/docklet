export interface PlanItem {
  service: string;
  action: string;
  detail: string;
  /** A completed one-shot service: it runs again on Apply, but that is not a change to the project. */
  oneShot?: boolean;
}

export interface ApplyOptions {
  removeOrphans: boolean;
  profiles: string[];
}

export interface ViewItem extends PlanItem {
  /** A remove that will not happen because Remove orphans is off. */
  skipped: boolean;
}

const LABELS: Record<string, string> = {
  start: "Start",
  recreate: "Recreate",
  recreateImage: "Recreate (new image)",
  scale: "Scale",
  new: "New",
  remove: "Remove",
  unchanged: "Unchanged",
};

export const actionLabel = (action: string): string => LABELS[action] ?? action;

/** The row label: a one-shot that only runs again says so, and a remove that is off is an orphan. */
export const itemLabel = (i: PlanItem & { skipped?: boolean }): string => (i.oneShot ? "Runs again" : i.skipped ? "Orphan" : actionLabel(i.action));

/** True when the row is not a pending change: unchanged, a finished one-shot, or a skipped remove. */
const quiet = (i: ViewItem): boolean => i.action === "unchanged" || i.skipped || i.oneShot === true;

export const SKIPPED_DETAIL = "Not in the file; stays until Remove orphans is on";

export function visibleItems(items: readonly PlanItem[], removeOrphans: boolean): ViewItem[] {
  return items.map((i) => {
    const skipped = i.action === "remove" && !removeOrphans;
    return { ...i, detail: skipped ? SKIPPED_DETAIL : i.detail, skipped };
  });
}

export const pendingChanges = (items: readonly PlanItem[], removeOrphans: boolean): boolean =>
  visibleItems(items, removeOrphans).some((i) => !quiet(i));

const ORDER = ["start", "recreate", "scale", "new", "remove", "unchanged"];

export function summary(items: readonly PlanItem[], removeOrphans: boolean): string {
  if (!pendingChanges(items, removeOrphans)) return "Nothing to do";
  const counts = new Map<string, number>();
  for (const i of visibleItems(items, removeOrphans)) {
    if (i.skipped) continue;
    const key = i.oneShot ? "unchanged" : i.action === "recreateImage" ? "recreate" : i.action;
    counts.set(key, (counts.get(key) ?? 0) + 1);
  }
  return ORDER.filter((k) => counts.has(k))
    .map((k) => `${counts.get(k)} ${k}`)
    .join(" · ");
}

export function applyDisabledReason(s: {
  dirty: boolean;
  planError: boolean;
  planLoading: boolean;
  canRun: boolean;
  capReason: string;
  busy: boolean;
  pending: boolean;
}): string | undefined {
  if (s.dirty) return "Save the file first";
  if (!s.canRun) return s.capReason || "Compose CLI is not available for this project";
  if (s.planLoading) return "Checking what would change";
  if (s.planError) return "Fix the configuration errors first";
  if (s.busy) return "A Compose command is being submitted";
  if (!s.pending) return "Nothing to apply";
  return undefined;
}
