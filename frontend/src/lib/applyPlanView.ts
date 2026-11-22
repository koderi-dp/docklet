export interface PlanItem {
  service: string;
  action: string;
  detail: string;
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
  new: "New",
  remove: "Remove",
  unchanged: "Unchanged",
};

export const actionLabel = (action: string): string => LABELS[action] ?? action;

export const SKIPPED_DETAIL = "Left running: not in the file";

export function visibleItems(items: readonly PlanItem[], removeOrphans: boolean): ViewItem[] {
  return items.map((i) => {
    const skipped = i.action === "remove" && !removeOrphans;
    return { ...i, detail: skipped ? SKIPPED_DETAIL : i.detail, skipped };
  });
}

const changes = (items: readonly PlanItem[], removeOrphans: boolean) =>
  visibleItems(items, removeOrphans).filter((i) => i.action !== "unchanged" && !i.skipped);

export const pendingChanges = (items: readonly PlanItem[], removeOrphans: boolean): boolean => changes(items, removeOrphans).length > 0;

const ORDER = ["start", "recreate", "new", "remove", "unchanged"];

export function summary(items: readonly PlanItem[], removeOrphans: boolean): string {
  if (!pendingChanges(items, removeOrphans)) return "Nothing to do";
  const counts = new Map<string, number>();
  for (const i of visibleItems(items, removeOrphans)) {
    if (i.skipped) continue;
    const key = i.action === "recreateImage" ? "recreate" : i.action;
    counts.set(key, (counts.get(key) ?? 0) + 1);
  }
  return ORDER.filter((k) => counts.has(k))
    .map((k) => `${counts.get(k)} ${k}`)
    .join(" · ");
}

export function applyDisabledReason(s: { dirty: boolean; planError: boolean; canRun: boolean; capReason: string; busy: boolean; pending: boolean }): string | undefined {
  if (s.dirty) return "Save the file first";
  if (!s.canRun) return s.capReason || "Compose CLI is not available for this project";
  if (s.planError) return "Fix the configuration errors first";
  if (s.busy) return "A Compose command is being submitted";
  if (!s.pending) return "Nothing to apply";
  return undefined;
}
