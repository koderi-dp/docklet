import { describe, expect, it } from "vitest";
import type { LogEntry } from "./logEntry";
import {
  adaptPanes,
  addTicked,
  allPane,
  defaultDistribution,
  describePane,
  emptyPane,
  reconcilePanes,
  resolveNames,
  sameLines,
  selectPaneLines,
  setNames,
  type PaneSource,
  type TickedItem,
} from "./logPanes";

const it_ = (name: string, project: string | null = null): TickedItem => ({ name, project });
const src = (...names: string[]): PaneSource => ({ all: false, names });
const names = (panes: PaneSource[]) => panes.map((p) => (p.all ? "ALL" : p.names.join(",")));

describe("defaultDistribution", () => {
  it("gives each of several projects its own pane, standalone after them", () => {
    const items = [it_("shop-api", "shop"), it_("shop-worker", "shop"), it_("blog-web", "blog"), it_("db"), it_("cache")];
    expect(names(defaultDistribution(items, 2))).toEqual(["blog-web", "shop-api,shop-worker,cache,db"]);
    expect(names(defaultDistribution(items, 4))).toEqual(["blog-web", "shop-api,shop-worker", "cache", "db"]);
  });

  it("falls back to one container per pane when there are fewer groups than panes", () => {
    const items = [it_("a-1", "a"), it_("a-2", "a"), it_("b-1", "b"), it_("b-2", "b")];
    expect(names(defaultDistribution(items, 4))).toEqual(["a-1", "a-2", "b-1", "b-2"]);
    expect(names(defaultDistribution(items, 2))).toEqual(["a-1,a-2", "b-1,b-2"]);
  });

  it("uses one container per pane without projects and puts the overflow into the last pane", () => {
    const items = [it_("c"), it_("a"), it_("b"), it_("d"), it_("e")];
    expect(names(defaultDistribution(items, 2))).toEqual(["a", "b,c,d,e"]);
  });

  it("leaves surplus panes empty and handles nothing ticked", () => {
    expect(names(defaultDistribution([it_("only")], 4))).toEqual(["only", "", "", ""]);
    expect(names(defaultDistribution([], 2))).toEqual(["", ""]);
  });

  it("shows everything in a single pane", () => {
    expect(defaultDistribution([it_("a"), it_("b")], 1)).toEqual([allPane()]);
  });
});

describe("resolveNames and describePane", () => {
  const items = [it_("api", "shop"), it_("worker", "shop"), it_("db")];
  it("keeps ticked order and ignores names that are not ticked", () => {
    expect(resolveNames(src("db", "gone", "api"), ["api", "worker", "db"])).toEqual(["api", "db"]);
    expect(resolveNames(allPane(), ["x", "y"])).toEqual(["x", "y"]);
  });
  it("describes a pane", () => {
    expect(describePane(emptyPane(), items).label).toBe("Nothing selected");
    expect(describePane(src("db"), items)).toMatchObject({ label: "db", single: "db" });
    expect(describePane(src("api", "worker"), items).label).toBe("shop");
    expect(describePane(src("api", "db"), items).label).toBe("2 containers");
    expect(describePane(allPane(), items).label).toBe("All ticked containers");
  });
});

describe("addTicked", () => {
  it("does nothing when a pane shows all ticked", () => {
    const r = addTicked([allPane(), src("a")], [it_("new")]);
    expect(names(r.panes)).toEqual(["ALL", "a"]);
    expect(r.placed).toEqual([]);
  });
  it("puts a new container into the pane with the fewest and a project together", () => {
    const r = addTicked([src("a", "b"), src("c")], [it_("x"), it_("p-1", "p"), it_("p-2", "p")]);
    expect(names(r.panes)).toEqual(["a,b,x", "c,p-1,p-2"]);
    expect(r.placed).toEqual([
      { name: "p-1", pane: 1 },
      { name: "p-2", pane: 1 },
      { name: "x", pane: 0 },
    ]);
  });
  it("skips containers a pane already shows", () => {
    expect(names(addTicked([src("a"), src("b")], [it_("a")]).panes)).toEqual(["a", "b"]);
  });
});

describe("reconcilePanes", () => {
  const panes = [src("a", "gone"), src("b")];
  it("drops unticked names and places newly ticked ones", () => {
    const r = reconcilePanes(panes, [it_("a"), it_("b"), it_("c")], new Set(["a", "b", "gone"]));
    expect(names(r.panes)).toEqual(["a,c", "b"]);
    expect(r.placed).toEqual([{ name: "c", pane: 0 }]);
  });
  it("returns the same array when nothing changed", () => {
    const same = [src("a"), src("b")];
    expect(reconcilePanes(same, [it_("a"), it_("b")], new Set(["a", "b"])).panes).toBe(same);
  });
  it("does not re-add a container the user took out of every pane", () => {
    const r = reconcilePanes([src("a"), src()], [it_("a"), it_("b")], new Set(["a", "b"]));
    expect(names(r.panes)).toEqual(["a", ""]);
  });
  it("on first load drops stale names and distributes afresh when nothing is left", () => {
    const r = reconcilePanes([src("old-1"), src("old-2")], [it_("a"), it_("b")], null);
    expect(names(r.panes)).toEqual(["a", "b"]);
  });
  it("on first load keeps valid names and places the rest", () => {
    const r = reconcilePanes([src("a"), src("old")], [it_("a"), it_("b")], null);
    expect(names(r.panes)).toEqual(["a", "b"]);
  });
  it("leaves a never-split view alone", () => {
    expect(reconcilePanes([], [it_("a")], null).panes).toEqual([]);
  });
});

describe("adaptPanes", () => {
  it("merges the removed panes into the last one when shrinking", () => {
    expect(names(adaptPanes([src("a"), src("b"), src("c"), src("d")], 2, []))).toEqual(["a", "b,c,d"]);
    expect(names(adaptPanes([src("a"), src("b"), allPane(), src("d")], 2, []))).toEqual(["a", "ALL"]);
  });
  it("distributes afresh when growing or starting", () => {
    expect(names(adaptPanes([src("a"), src("b")], 4, [it_("a"), it_("b"), it_("c"), it_("d")]))).toEqual(["a", "b", "c", "d"]);
    expect(names(adaptPanes([], 2, [it_("a"), it_("b")]))).toEqual(["a", "b"]);
  });
  it("keeps the panes when the count already fits", () => {
    const p = [src("a"), src("b")];
    expect(adaptPanes(p, 2, [])).toBe(p);
  });
});

describe("setNames", () => {
  const ticked = ["a", "b", "c"];
  it("turns 'all ticked' into the explicit list when one is unticked", () => {
    expect(setNames(allPane(), ticked, ["b"], false)).toEqual(src("a", "c"));
  });
  it("adds several at once in ticked order", () => {
    expect(setNames(src("c"), ticked, ["a", "b"], true)).toEqual(src("a", "b", "c"));
  });
});

describe("selectPaneLines", () => {
  const e = (seq: number, container: string, text: string) => ({ seq, container, text, level: "info", stream: "stdout", timestamp: "", sortKey: seq }) as LogEntry;
  const visible = [e(1, "a", "GET /x"), e(2, "b", "GET /y"), e(3, "a", "boom"), e(4, "c", "GET /z")];
  const marks = [undefined, "warning", "info", undefined] as const;

  it("returns the inputs untouched when there is nothing to select", () => {
    const hl = [...marks];
    const r = selectPaneLines(visible, hl, null, null);
    expect(r.entries).toBe(visible);
    expect(r.highlights).toBe(hl);
  });
  it("selects by container and keeps highlights aligned", () => {
    const r = selectPaneLines(visible, [...marks], new Set(["a", "c"]), null);
    expect(r.entries.map((x) => x.seq)).toEqual([1, 3, 4]);
    expect(r.highlights).toEqual([undefined, "info", undefined]);
  });
  it("adds the pane's own text test", () => {
    const r = selectPaneLines(visible, [...marks], new Set(["a", "b"]), (t) => t.startsWith("GET"));
    expect(r.entries.map((x) => x.seq)).toEqual([1, 2]);
  });
  it("sameLines recognises an unchanged selection", () => {
    const a = selectPaneLines(visible, [...marks], new Set(["a"]), null);
    const b = selectPaneLines([...visible, e(5, "b", "later")], [...marks, undefined], new Set(["a"]), null);
    const c = selectPaneLines([...visible, e(6, "a", "more")], [...marks, undefined], new Set(["a"]), null);
    expect(sameLines(a, b)).toBe(true);
    expect(sameLines(a, c)).toBe(false);
  });
});
