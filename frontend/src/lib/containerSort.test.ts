import { describe, expect, it } from "vitest";
import type { Container, ContainerStat } from "@bindings/model";
import { buildGroups } from "./composeGrouping";
import { sortGroups } from "./containerSort";

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
  it("keeps groups ordered by name with Standalone last, reversing only for a descending name sort", () => {
    expect(sortGroups(buildGroups("A", all), { id: "cpu", desc: true }, stats).map((g) => g.project)).toEqual(["alpha", "shop", null]);
    expect(sortGroups(buildGroups("A", all), { id: "name", desc: true }, stats).map((g) => g.project)).toEqual(["shop", "alpha", null]);
  });
});
