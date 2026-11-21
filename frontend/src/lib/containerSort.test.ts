import { describe, expect, it } from "vitest";
import type { Container, ContainerStat } from "@bindings/model";
import { buildGroups, type ComposeGroup } from "./composeGrouping";
import { sortGroups } from "./containerSort";
import { mergeStats } from "./statIdentity";

const c = (name: string, project?: string, state = "running"): Container =>
  ({ connectionId: "A", id: name, name, image: "i", state, status: state === "running" ? "Up 1 hour" : "Exited (0) 1 hour ago", health: "none", created: 1, ports: [], composeProject: project }) as Container;

describe("sortGroups", () => {
  const all = [c("web", "shop"), c("db", "shop"), c("cache", "shop", "exited"), c("zed", "alpha"), c("solo")];
  const stats = new Map<string, ContainerStat>([
    ["web", { id: "web", cpuPercent: 5 }],
    ["db", { id: "db", cpuPercent: 40 }],
    ["cache", { id: "cache", cpuPercent: 99 }], // stopped: sample must be ignored
  ]);
  const shop = (sort: Parameters<typeof sortGroups>[1]) => sortGroups(buildGroups("A", all), sort, stats).find((g) => g.project === "shop")!.visible.map((x) => x.name);

  it("sorts containers within a group by CPU, unknown last in both directions", () => {
    expect(shop({ id: "cpu", desc: true })).toEqual(["db", "web", "cache"]);
    expect(shop({ id: "cpu", desc: false })).toEqual(["web", "db", "cache"]);
  });
  it("keeps a group object when its order is unchanged and reorders it when a stat changes the order", () => {
    const groups = buildGroups("A", all);
    const first = sortGroups(groups, { id: "cpu", desc: true }, stats);
    const sortedShop = first.find((g) => g.project === "shop")!;
    // Feed the sorted group back in: nothing to reorder, so the same object comes out.
    expect(sortGroups([sortedShop], { id: "cpu", desc: true }, stats)[0]).toBe(sortedShop);
    const flipped = new Map(stats).set("web", { id: "web", cpuPercent: 90 } as ContainerStat);
    const again = sortGroups([sortedShop], { id: "cpu", desc: true }, flipped)[0];
    expect(again).not.toBe(sortedShop);
    expect(again.visible.map((x) => x.name)).toEqual(["web", "db", "cache"]);
  });
  it("keeps group objects across ticks under a non-name sort, replacing only a group whose order changed", () => {
    const built = buildGroups("A", all);
    const sort = { id: "cpu", desc: true };
    const wanted = new Set(["web", "db", "cache"]);
    const incoming = [...stats.values()];
    const tick1 = mergeStats(new Map(), incoming, wanted);
    const tick2 = mergeStats(tick1, incoming.map((s) => ({ ...s })), wanted);
    const a = sortGroups(built, sort, tick1);
    const b = sortGroups(built, sort, tick2, a);
    b.forEach((g, i) => expect(g).toBe(a[i]));
    const tick3 = mergeStats(tick2, [{ id: "web", cpuPercent: 90 } as ContainerStat, { id: "db", cpuPercent: 40 } as ContainerStat], wanted);
    const d = sortGroups(built, sort, tick3, b);
    const idx = (r: ComposeGroup[], p: string) => r.findIndex((g) => (g.project ?? "") === p);
    expect(d[idx(d, "shop")]).not.toBe(b[idx(b, "shop")]);
    expect(d[idx(d, "shop")].visible.map((x) => x.name)).toEqual(["web", "db", "cache"]);
    expect(d[idx(d, "alpha")]).toBe(b[idx(b, "alpha")]);
    expect(d[idx(d, "")]).toBe(b[idx(b, "")]);
  });
  it("keeps groups ordered by name with Standalone last, reversing only for a descending name sort", () => {
    expect(sortGroups(buildGroups("A", all), { id: "cpu", desc: true }, stats).map((g) => g.project)).toEqual(["alpha", "shop", null]);
    expect(sortGroups(buildGroups("A", all), { id: "name", desc: true }, stats).map((g) => g.project)).toEqual(["shop", "alpha", null]);
  });
});
