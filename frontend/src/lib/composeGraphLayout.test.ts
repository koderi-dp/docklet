import { describe, expect, it } from "vitest";
import { layoutGraph } from "./composeGraphLayout";

const n = (name: string, ...dependsOn: string[]) => ({ name, dependsOn });
const col = (l: ReturnType<typeof layoutGraph>, name: string) => l.nodes.find((x) => x.name === name)!.column;

describe("layoutGraph", () => {
  it("lays a chain out left to right", () => {
    const l = layoutGraph([n("a", "b"), n("b", "c"), n("c")]);
    expect([col(l, "a"), col(l, "b"), col(l, "c")]).toEqual([0, 1, 2]);
    expect(l.cyclic).toEqual([]);
    expect(l.edges.every((e) => !e.inCycle)).toBe(true);
  });

  it("places a diamond with the shared dependency last", () => {
    const l = layoutGraph([n("web", "api", "worker"), n("api", "db"), n("worker", "db"), n("db")]);
    expect(col(l, "web")).toBe(0);
    expect(col(l, "api")).toBe(1);
    expect(col(l, "worker")).toBe(1);
    expect(col(l, "db")).toBe(2);
  });

  it("puts independent services in one column, stacked", () => {
    const l = layoutGraph([n("c"), n("a"), n("b")]);
    expect(l.nodes.map((x) => x.column)).toEqual([0, 0, 0]);
    expect(l.nodes.map((x) => x.name).sort()).toEqual(["a", "b", "c"]);
    expect(new Set(l.nodes.map((x) => x.y)).size).toBe(3);
  });

  it("puts a service with no links in the last column", () => {
    const l = layoutGraph([n("a", "b"), n("b"), n("solo")]);
    expect(col(l, "solo")).toBe(1);
  });

  it("terminates on a two-node cycle and flags it", () => {
    const l = layoutGraph([n("a", "b"), n("b", "a")]);
    expect(l.cyclic).toEqual(["a", "b"]);
    expect(l.edges.filter((e) => e.inCycle)).toHaveLength(1);
    expect(l.edges).toHaveLength(2);
  });

  it("terminates on a three-node cycle with a tail", () => {
    const l = layoutGraph([n("a", "b"), n("b", "c"), n("c", "a", "d"), n("d")]);
    expect(l.cyclic).toEqual(["a", "b", "c"]);
    expect(l.edges.filter((e) => e.inCycle)).toHaveLength(1);
    expect(col(l, "d")).toBeGreaterThan(col(l, "c"));
  });

  it("ignores a self dependency but flags it", () => {
    const l = layoutGraph([n("a", "a")]);
    expect(l.cyclic).toEqual(["a"]);
    expect(l.edges).toEqual([expect.objectContaining({ from: "a", to: "a", inCycle: true })]);
    expect(col(l, "a")).toBe(0);
  });

  it("ignores an unknown dependency", () => {
    const l = layoutGraph([n("a", "ghost")]);
    expect(l.edges).toEqual([]);
    expect(l.cyclic).toEqual([]);
  });

  it("computes positions and the bounding box from the size", () => {
    const l = layoutGraph([n("a", "b"), n("b")], { nodeWidth: 100, nodeHeight: 50, columnGap: 20, rowGap: 10 });
    expect(l.nodes.find((x) => x.name === "b")).toMatchObject({ column: 1, x: 120 });
    expect(l.width).toBe(220);
    expect(l.height).toBe(50);
  });

  it("draws an edge from the right middle to the left middle", () => {
    const l = layoutGraph([n("a", "b"), n("b")], { nodeWidth: 100, nodeHeight: 50, columnGap: 20, rowGap: 10 });
    expect(l.edges[0].path).toBe("M 100 25 C 140 25, 80 25, 120 25");
  });

  it("centres a short column against the tallest", () => {
    const l = layoutGraph([n("a", "x"), n("b", "x"), n("c", "x"), n("x")], { nodeHeight: 100, rowGap: 20 });
    expect(l.nodes.find((q) => q.name === "x")!.y).toBe(120);
    expect(l.height).toBe(340);
  });

  it("handles 50 random services quickly and sanely", () => {
    let seed = 42;
    const rnd = () => (seed = (seed * 1664525 + 1013904223) % 4294967296) / 4294967296;
    const nodes = Array.from({ length: 50 }, (_, i) => n(`s${i}`));
    for (const node of nodes) {
      for (let k = 0; k < 3; k++) if (rnd() < 0.6) node.dependsOn.push(`s${Math.floor(rnd() * 50)}`);
    }
    const t = performance.now();
    const l = layoutGraph(nodes);
    expect(performance.now() - t).toBeLessThan(100);
    const c = new Map(l.nodes.map((x) => [x.name, x.column]));
    for (const e of l.edges) if (!e.inCycle) expect(c.get(e.to)!).toBeGreaterThan(c.get(e.from)!);
    expect(new Set(l.nodes.map((x) => `${x.x},${x.y}`)).size).toBe(50);
  });

  it("is deterministic", () => {
    const input = [n("a", "b", "c"), n("b", "d"), n("c", "d"), n("d"), n("e")];
    expect(layoutGraph(input)).toEqual(layoutGraph(input));
  });
});
