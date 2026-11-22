import { describe, expect, it } from "vitest";
import { actionLabel, applyDisabledReason, itemLabel, pendingChanges, summary, visibleItems } from "./applyPlanView";

const item = (service: string, action: string, detail = "") => ({ service, action, detail });
const base = { dirty: false, planError: false, planLoading: false, canRun: true, capReason: "", busy: false, pending: true };

describe("actionLabel", () => {
  it("names every action", () => {
    expect(["start", "recreate", "recreateImage", "new", "remove", "unchanged"].map(actionLabel)).toEqual([
      "Start",
      "Recreate",
      "Recreate (new image)",
      "New",
      "Remove",
      "Unchanged",
    ]);
  });
});

describe("visibleItems", () => {
  const items = [item("a", "remove", "x"), item("b", "new")];
  it("marks a remove skipped when orphans are kept", () => {
    const v = visibleItems(items, false);
    expect(v[0]).toMatchObject({ service: "a", skipped: true, detail: "Not in the file; stays until Remove orphans is on" });
    expect(v[1].skipped).toBe(false);
  });
  it("keeps the remove when orphans are removed", () => {
    expect(visibleItems(items, true)[0]).toMatchObject({ skipped: false, detail: "x" });
  });
});

describe("summary and pendingChanges", () => {
  it("says nothing to do when all are unchanged or skipped", () => {
    const items = [item("a", "unchanged"), item("b", "remove")];
    expect(summary(items, false)).toBe("Nothing to do");
    expect(pendingChanges(items, false)).toBe(false);
    expect(pendingChanges(items, true)).toBe(true);
  });
  it("counts in a fixed order", () => {
    const items = [item("a", "new"), item("b", "recreate"), item("c", "recreateImage"), item("d", "unchanged"), item("e", "unchanged"), item("f", "unchanged")];
    expect(summary(items, false)).toBe("2 recreate · 1 new · 3 unchanged");
    expect(pendingChanges(items, false)).toBe(true);
  });
});

describe("scale and one-shot items", () => {
  const job = { service: "migrate", action: "start", detail: "", oneShot: true };
  it("labels and counts scale after recreate", () => {
    expect(actionLabel("scale")).toBe("Scale");
    const items = [item("a", "scale"), item("b", "recreate"), item("c", "new")];
    expect(summary(items, false)).toBe("1 recreate · 1 scale · 1 new");
    expect(pendingChanges(items, false)).toBe(true);
  });
  it("keeps a finished one-shot as a quiet row", () => {
    const items = [item("web", "unchanged"), job];
    expect(itemLabel(job)).toBe("Runs again");
    expect(visibleItems(items, false)).toHaveLength(2);
    expect(pendingChanges(items, false)).toBe(false);
    expect(summary(items, false)).toBe("Nothing to do");
  });
  it("counts a one-shot as unchanged next to a real change", () => {
    expect(summary([job, item("web", "new")], false)).toBe("1 new · 1 unchanged");
  });
});

describe("applyDisabledReason", () => {
  it("is undefined when Apply can run", () => {
    expect(applyDisabledReason(base)).toBeUndefined();
  });
  it("picks the first that applies", () => {
    expect(applyDisabledReason({ ...base, dirty: true, planError: true, busy: true })).toBe("Save the file first");
    expect(applyDisabledReason({ ...base, canRun: false, capReason: "No CLI", planError: true })).toBe("No CLI");
    expect(applyDisabledReason({ ...base, planLoading: true, planError: true })).toBe("Checking what would change");
    expect(applyDisabledReason({ ...base, planError: true, busy: true })).toBe("Fix the configuration errors first");
    expect(applyDisabledReason({ ...base, busy: true, pending: false })).toBe("A Compose command is being submitted");
    expect(applyDisabledReason({ ...base, pending: false })).toBe("Nothing to apply");
  });
});

describe("itemLabel for an orphan", () => {
  it("says Orphan while Remove orphans is off and Remove when it is on", () => {
    const items = [{ service: "old", action: "remove", detail: "no longer in the file", oneShot: false }];
    expect(itemLabel(visibleItems(items, false)[0])).toBe("Orphan");
    expect(itemLabel(visibleItems(items, true)[0])).toBe("Remove");
  });
});
