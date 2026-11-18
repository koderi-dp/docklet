import { describe, expect, it } from "vitest";
import type { Container, ContainerStat } from "@bindings/model";
import { aggregateProject, partialNote, totalsByProject } from "./projectTotals";

const c = (id: string, state: string, project = "p"): Container => ({ id, state, composeProject: project }) as Container;
const stats = (...s: ContainerStat[]) => new Map(s.map((x) => [x.id, x]));

describe("aggregateProject", () => {
  it("sums CPU and memory of running containers only", () => {
    const t = aggregateProject(
      "p",
      [c("a", "running"), c("b", "running"), c("x", "exited")],
      stats({ id: "a", cpuPercent: 10, memoryBytes: 100 }, { id: "b", cpuPercent: 2.5, memoryBytes: 50 }, { id: "x", cpuPercent: 99, memoryBytes: 999 }),
    );
    expect(t.cpu).toEqual({ value: 12.5, reporting: 2, running: 2 });
    expect(t.memory).toEqual({ value: 150, reporting: 2, running: 2 });
  });
  it("marks partial data and never turns unknown into zero", () => {
    const t = aggregateProject("p", [c("a", "running"), c("b", "running")], stats({ id: "a", cpuPercent: 3, memoryBytes: null }));
    expect(t.cpu).toEqual({ value: 3, reporting: 1, running: 2 });
    expect(t.memory).toBeNull();
    expect(partialNote(t.cpu)).toBe("1 of 2 containers reporting");
    expect(partialNote({ value: 1, reporting: 2, running: 2 })).toBeUndefined();
  });
  it("is null when nothing runs or nothing reported, but a real 0 stays 0", () => {
    expect(aggregateProject("p", [c("a", "exited")], stats()).cpu).toBeNull();
    expect(aggregateProject("p", [c("a", "running")], stats()).cpu).toBeNull();
    expect(aggregateProject("p", [c("a", "running")], stats({ id: "a", cpuPercent: 0, memoryBytes: 0 })).cpu?.value).toBe(0);
  });
});

describe("totalsByProject", () => {
  it("groups only by the compose project label", () => {
    const m = totalsByProject(
      [c("a", "running", "one"), c("b", "running", "two"), { id: "z", state: "running" } as Container],
      stats({ id: "a", cpuPercent: 1 }, { id: "b", cpuPercent: 2 }, { id: "z", cpuPercent: 50 }),
    );
    expect([...m.keys()].sort()).toEqual(["one", "two"]);
    expect(m.get("two")?.cpu?.value).toBe(2);
  });
});
