import { describe, expect, it } from "vitest";
import { ariaSort, compareValues, nextSort, sortItems } from "./sorting";

interface Row {
  n: string;
  v?: number | null;
}
const acc = { n: (r: Row) => r.n, v: (r: Row) => r.v };
const names = (rows: Row[]) => rows.map((r) => r.n);

describe("sortItems", () => {
  const rows: Row[] = [{ n: "b", v: 10 }, { n: "a", v: null }, { n: "c", v: 2 }, { n: "d" }, { n: "e", v: 100 }];

  it("sorts numbers numerically, not as text", () => {
    expect(names(sortItems(rows, { id: "v", desc: false }, acc))).toEqual(["c", "b", "e", "a", "d"]);
  });
  it("keeps unknown values last when descending too", () => {
    expect(names(sortItems(rows, { id: "v", desc: true }, acc))).toEqual(["e", "b", "c", "a", "d"]);
  });
  it("is stable for equal values", () => {
    const same: Row[] = [{ n: "x", v: 1 }, { n: "y", v: 1 }, { n: "z", v: 1 }];
    expect(names(sortItems(same, { id: "v", desc: true }, acc))).toEqual(["x", "y", "z"]);
  });
  it("sorts names naturally and case-insensitively", () => {
    const r: Row[] = [{ n: "web-10" }, { n: "Web-2" }, { n: "api" }];
    expect(names(sortItems(r, { id: "n", desc: false }, acc))).toEqual(["api", "Web-2", "web-10"]);
  });
  it("treats empty strings as unknown and last", () => {
    const r: Row[] = [{ n: "" }, { n: "b" }, { n: "a" }];
    expect(names(sortItems(r, { id: "n", desc: true }, acc))).toEqual(["b", "a", ""]);
  });
  it("returns the input order for no sort or an unknown column, without mutating", () => {
    const copy = [...rows];
    expect(sortItems(rows, undefined, acc)).toEqual(copy);
    expect(sortItems(rows, { id: "zzz", desc: false }, acc)).toEqual(copy);
    expect(rows).toEqual(copy);
  });
});

describe("sort state helpers", () => {
  it("toggles direction on the same column and resets to ascending on a new one", () => {
    expect(nextSort(undefined, "name")).toEqual({ id: "name", desc: false });
    expect(nextSort({ id: "name", desc: false }, "name")).toEqual({ id: "name", desc: true });
    expect(nextSort({ id: "name", desc: true }, "size")).toEqual({ id: "size", desc: false });
  });
  it("reports aria-sort", () => {
    expect(ariaSort({ id: "a", desc: true }, "a")).toBe("descending");
    expect(ariaSort({ id: "a", desc: false }, "a")).toBe("ascending");
    expect(ariaSort({ id: "a", desc: false }, "b")).toBe("none");
  });
  it("orders unknown after known", () => {
    expect(compareValues(1, null)).toBeLessThan(0);
    expect(compareValues(undefined, 1)).toBeGreaterThan(0);
  });
});
