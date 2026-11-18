// Which lifecycle actions are meaningful for a container in a given Engine state,
// and how a mixed selection is split into eligible ids and skipped containers.
import type { Container } from "@bindings/model";

export type ContainerAction = "start" | "stop" | "restart" | "remove" | "pause" | "unpause" | "kill";

const ELIGIBLE_STATES: Record<ContainerAction, ReadonlySet<string>> = {
  start: new Set(["created", "exited"]),
  stop: new Set(["running", "restarting", "paused"]),
  restart: new Set(["running", "restarting", "paused", "exited", "created"]),
  remove: new Set(["created", "running", "paused", "restarting", "exited", "dead"]),
  pause: new Set(["running"]),
  unpause: new Set(["paused"]),
  kill: new Set(["running", "restarting"]),
};

export const ACTION_LABEL: Record<ContainerAction, string> = {
  start: "Start",
  stop: "Stop",
  restart: "Restart",
  remove: "Delete",
  pause: "Pause",
  unpause: "Unpause",
  kill: "Kill",
};

export function isEligible(action: ContainerAction, state: string): boolean {
  return ELIGIBLE_STATES[action].has(state);
}

/** Human reason an action is unavailable, for disabled-control tooltips. */
export function ineligibleReason(action: ContainerAction, state: string): string | undefined {
  if (isEligible(action, state)) return undefined;
  if (state === "removing") return "Container is being removed";
  switch (action) {
    case "start":
      return state === "running" ? "Already running" : `Cannot start a container in state "${state}"`;
    case "stop":
      return `Not running (state: ${state})`;
    case "restart":
      return `Cannot restart a container in state "${state}"`;
    case "remove":
      return `Cannot delete a container in state "${state}"`;
    case "pause":
      return `Only running containers can be paused (state: ${state})`;
    case "unpause":
      return `Not paused (state: ${state})`;
    case "kill":
      return `Not running (state: ${state})`;
  }
}

type Target = Pick<Container, "id" | "name" | "state">;

export interface EligibilityPartition<T extends Target = Target> {
  eligible: T[];
  skipped: T[];
  eligibleIds: string[];
  skippedCount: number;
}

/** Mixed selection => eligible containers + skipped count (skipped are never sent to the Engine). */
export function partitionByEligibility<T extends Target>(
  action: ContainerAction,
  targets: readonly T[],
): EligibilityPartition<T> {
  const eligible: T[] = [];
  const skipped: T[] = [];
  for (const t of targets) (isEligible(action, t.state) ? eligible : skipped).push(t);
  return { eligible, skipped, eligibleIds: eligible.map((t) => t.id), skippedCount: skipped.length };
}

/**
 * Resolve explicit selected ids to containers. Only ids that are selected are ever returned:
 * hidden/filtered-out containers are included only if they were explicitly selected.
 */
export function resolveSelection<T extends Target>(
  all: readonly T[],
  selectedIds: ReadonlySet<string>,
): T[] {
  return all.filter((c) => selectedIds.has(c.id));
}

/** Whether removing these requires force (any running/restarting/paused container). */
export function needsForceRemove(targets: readonly Target[]): boolean {
  return targets.some((t) => t.state === "running" || t.state === "restarting" || t.state === "paused");
}
