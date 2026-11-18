import { describe, expect, it } from "vitest";
import type { Container } from "@bindings/model";
import {
  aggregateHealth,
  aggregateState,
  applyRange,
  buildGroups,
  checkState,
  EMPTY_FILTERS,
  flattenGrouped,
  groupId,
  oneShotNotes,
  STANDALONE_PROJECT,
  summarize,
  toggleIds,
} from "./composeGrouping";

function c(over: Partial<Omit<Container, "health">> & { health?: string; id: string; name: string }): Container {
  return {
    connectionId: "A",
    image: "img:1",
    state: "running",
    status: "Up",
    health: "none",
    created: 0,
    ports: [],
    ...over,
  } as Container;
}

const sample = [
  c({ id: "1", name: "web-1", composeProject: "myapp", composeService: "web", health: "healthy" }),
  c({ id: "2", name: "api-1", composeProject: "myapp", composeService: "api", health: "unhealthy" }),
  c({ id: "3", name: "db-1", composeProject: "platform", state: "running", health: "healthy" }),
  c({ id: "4", name: "redis-1", composeProject: "platform", state: "exited" }),
  c({ id: "5", name: "dev-shell", image: "ubuntu:24.04" }),
  // name prefix looks like a project but has no label: must stay standalone
  c({ id: "6", name: "myapp-extra" }),
];

describe("grouping", () => {
  it("groups by exact compose label only; standalone last", () => {
    const g = buildGroups("A", sample);
    expect(g.map((x) => x.project)).toEqual(["myapp", "platform", null]);
    expect(g[2].kind).toBe("standalone");
    expect(g[2].all.map((x) => x.name)).toEqual(["dev-shell", "myapp-extra"]);
    expect(g[0].all.map((x) => x.name)).toEqual(["api-1", "web-1"]);
  });

  it("aggregates state", () => {
    const g = buildGroups("A", sample);
    expect(g[0].state).toBe("running");
    expect(g[1].state).toBe("partial");
    expect(aggregateState([c({ id: "x", name: "x", state: "exited" })])).toBe("stopped");
    expect(aggregateState([c({ id: "x", name: "x", state: "running" }), c({ id: "y", name: "y", state: "restarting" })])).toBe("transitioning");
    expect(aggregateState([c({ id: "x", name: "x", state: "exited" })], new Set(["x"]))).toBe("transitioning");
  });

  it("health is computed only from real health values", () => {
    expect(aggregateHealth([c({ id: "a", name: "a" })]).kind).toBe("none");
    expect(aggregateHealth([c({ id: "a", name: "a", health: "healthy" }), c({ id: "b", name: "b" })])).toMatchObject({ kind: "healthy", withHealthcheck: 1, none: 1 });
    expect(aggregateHealth([c({ id: "a", name: "a", health: "unhealthy" })]).kind).toBe("degraded");
    const s = summarize(sample);
    expect(s).toMatchObject({ total: 6, running: 5, stopped: 1, healthy: 2, withHealthcheck: 3 });
  });

  it("filtering keeps the parent group, totals stay unfiltered", () => {
    const g = buildGroups("A", sample, { ...EMPTY_FILTERS, tab: "stopped" });
    expect(g).toHaveLength(1);
    expect(g[0].project).toBe("platform");
    expect(g[0].visible.map((x) => x.name)).toEqual(["redis-1"]);
    expect(g[0].all).toHaveLength(2);
    expect(g[0].filtered).toBe(true);
    // state still reflects all children
    expect(g[0].state).toBe("partial");
  });

  it("project name search exposes all children; service search matches", () => {
    const byProject = buildGroups("A", sample, { ...EMPTY_FILTERS, search: "platform" });
    expect(byProject).toHaveLength(1);
    expect(byProject[0].visible).toHaveLength(2);
    expect(byProject[0].filtered).toBe(false);
    const bySvc = buildGroups("A", sample, { ...EMPTY_FILTERS, search: "api" });
    expect(bySvc.map((x) => x.project)).toEqual(["myapp"]);
    expect(bySvc[0].visible).toHaveLength(1);
  });

  it("health filter 'none' excludes real healthchecks", () => {
    const g = buildGroups("A", sample, { ...EMPTY_FILTERS, health: "none" });
    const names = g.flatMap((x) => x.visible.map((v) => v.name));
    expect(names).toEqual(["redis-1", "dev-shell", "myapp-extra"]);
  });

  it("collapsed groups hide children but keep header", () => {
    const g = buildGroups("A", sample);
    const rows = flattenGrouped(g, (x) => x.project === "myapp");
    expect(rows.filter((r) => r.kind === "group")).toHaveLength(3);
    expect(rows.some((r) => r.kind === "container" && r.container.name === "web-1")).toBe(false);
  });
});

describe("cross-engine isolation", () => {
  it("same project on two engines yields distinct ids and never merges", () => {
    expect(groupId("A", "myapp")).not.toBe(groupId("B", "myapp"));
    expect(groupId("A", "")).toBe(`A:${STANDALONE_PROJECT}`);
    const mixed = [...sample, c({ id: "9", name: "web-1", connectionId: "B", composeProject: "myapp" })];
    const a = buildGroups("A", mixed);
    expect(a[0].all.map((x) => x.id)).not.toContain("9");
    const b = buildGroups("B", mixed);
    expect(b).toHaveLength(1);
    expect(b[0].id).toBe("B:myapp");
  });
});

describe("selection helpers", () => {
  it("tri-state and group toggle only touch given visible ids", () => {
    const sel = new Set(["x"]);
    expect(checkState(["a", "b"], sel)).toBe("unchecked");
    const next = toggleIds(sel, ["a", "b"]);
    expect([...next].sort()).toEqual(["a", "b", "x"]);
    expect(checkState(["a", "b"], next)).toBe("checked");
    expect(checkState(["a", "b", "c"], next)).toBe("indeterminate");
    const off = toggleIds(next, ["a", "b"]);
    expect([...off]).toEqual(["x"]);
    const partial = toggleIds(new Set(["a"]), ["a", "b"]);
    expect(checkState(["a", "b"], partial)).toBe("checked");
  });
});

describe("applyRange (shift+click)", () => {
  const order = ["a", "b", "c", "d", "e"];
  it("selects an inclusive range in either direction", () => {
    expect([...applyRange(new Set(), order, "b", "d", true)].sort()).toEqual(["b", "c", "d"]);
    expect([...applyRange(new Set(), order, "d", "b", true)].sort()).toEqual(["b", "c", "d"]);
  });
  it("deselects a range and keeps unrelated ids", () => {
    const next = applyRange(new Set(["a", "b", "c", "x"]), order, "a", "c", false);
    expect([...next]).toEqual(["x"]);
  });
  it("falls back to the target alone without a usable anchor", () => {
    expect([...applyRange(new Set(), order, null, "c", true)]).toEqual(["c"]);
    expect([...applyRange(new Set(), order, "zz", "c", true)]).toEqual(["c"]);
  });
});

describe("one-shot containers (completed / failed)", () => {
  const web = c({ id: "w", name: "web", state: "running" });
  const pg = c({ id: "p", name: "pg", state: "running" });
  const done = c({ id: "m", name: "migrate", state: "exited", exitCode: 0, restartPolicy: "no" });
  const failed = c({ id: "f", name: "seed", state: "exited", exitCode: 2, restartPolicy: "no" });
  const stoppedSvc = c({ id: "s", name: "cache", state: "exited", exitCode: 0, restartPolicy: "always" });
  const unknownPolicy = c({ id: "u", name: "u", state: "exited", exitCode: 0 });

  it("running services plus a completed migrate is Running, not Partial", () => {
    expect(aggregateState([web, pg, done])).toBe("running");
  });
  it("a failed or a stopped long-running service keeps the project Partial", () => {
    expect(aggregateState([web, pg, failed])).toBe("partial");
    expect(aggregateState([web, pg, stoppedSvc])).toBe("partial");
  });
  it("unknown restart policy is never treated as completed", () => {
    expect(aggregateState([web, unknownPolicy])).toBe("partial");
  });
  it("only completed containers: nothing runs, so Stopped", () => {
    expect(aggregateState([done])).toBe("stopped");
  });
  it("completed ones do not hide a fully stopped project", () => {
    expect(aggregateState([done, failed])).toBe("stopped");
  });
  it("counts completed and failed in summaries and groups", () => {
    const all = [web, pg, done, failed, stoppedSvc].map((x) => ({ ...x, composeProject: "catalo" }));
    expect(summarize(all)).toMatchObject({ running: 2, stopped: 3, completed: 1, failed: 1 });
    expect(buildGroups("A", all)[0]).toMatchObject({ completed: 1, failed: 1, state: "partial" });
    expect(buildGroups("A", [web, pg, done].map((x) => ({ ...x, composeProject: "catalo" })))[0]).toMatchObject({ state: "running", completed: 1, failed: 0 });
  });
  it("builds the header notes", () => {
    expect(oneShotNotes({ completed: 1, failed: 2 })).toEqual([
      { text: "1 completed", tone: "muted" },
      { text: "2 failed", tone: "warning" },
    ]);
    expect(oneShotNotes({ completed: 0, failed: 0 })).toEqual([]);
  });
});
