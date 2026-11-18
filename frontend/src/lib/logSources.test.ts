import { describe, expect, it } from "vitest";
import type { Container } from "@bindings/model";
import { defaultSelection, groupSources } from "./logSources";

const c = (name: string, state: string, composeProject?: string): Container => ({ id: `id-${name}`, name, state, composeProject }) as Container;

const LIST = [c("web-1", "running", "shop"), c("db-1", "running", "shop"), c("api-1", "exited", "shop"), c("tool", "running"), c("alpha-1", "running", "alpha")];

describe("groupSources", () => {
  it("groups by compose project, alphabetical, standalone last", () => {
    const g = groupSources(LIST, { search: "", runningOnly: false });
    expect(g.map((x) => x.project)).toEqual(["alpha", "shop", null]);
    expect(g[1].containers.map((x) => x.name)).toEqual(["api-1", "db-1", "web-1"]);
  });

  it("running only hides stopped containers unless they are ticked", () => {
    expect(groupSources(LIST, { search: "", runningOnly: true })[1].containers.map((x) => x.name)).toEqual(["db-1", "web-1"]);
    const kept = groupSources(LIST, { search: "", runningOnly: true, keep: new Set(["id-api-1"]) });
    expect(kept[1].containers.map((x) => x.name)).toEqual(["api-1", "db-1", "web-1"]);
  });

  it("search matches name, project and service", () => {
    expect(groupSources(LIST, { search: "SHOP", runningOnly: false })).toHaveLength(1);
    expect(groupSources(LIST, { search: "tool", runningOnly: false })[0].project).toBeNull();
  });
});

describe("defaultSelection", () => {
  it("ticks running containers only and reports how many were left out by the cap", () => {
    const all = defaultSelection(LIST);
    expect(all.total).toBe(4);
    expect(all.ids).not.toContain("id-api-1");
    const capped = defaultSelection(LIST, 2);
    expect(capped.ids).toHaveLength(2);
    expect(capped.total).toBe(4);
  });
});
