import { describe, expect, it } from "vitest";
import { compileSearch, findRanges, matchingSeqs, positionOf, stepMatch } from "./search";

const must = (q: string) => {
  const { search } = compileSearch(q);
  if (!search) throw new Error("expected a valid search");
  return search;
};

describe("compileSearch", () => {
  it("is empty for blank text and reports an invalid regex", () => {
    expect(compileSearch("")).toEqual({ search: null, error: null });
    expect(compileSearch("   ").search).toBeNull();
    const bad = compileSearch("/(/");
    expect(bad.search).toBeNull();
    expect(bad.error).toBeTruthy();
  });

  it("marks plain text case-insensitively and every occurrence", () => {
    expect(must("err").ranges("Error: err in ERR")).toEqual([
      { start: 0, end: 3 },
      { start: 7, end: 10 },
      { start: 14, end: 17 },
    ]);
  });

  it("treats regex characters in plain text literally", () => {
    const s = must("a.b");
    expect(s.test("xa.bx")).toBe(true);
    expect(s.test("axb")).toBe(false);
  });

  it("supports wildcards and /regex/", () => {
    expect(must("GET /api/*").ranges("hit get /api/users 200")).toEqual([{ start: 4, end: 22 }]);
    expect(must("/[0-9]+ms/").ranges("took 120ms and 7ms")).toEqual([
      { start: 5, end: 10 },
      { start: 15, end: 18 },
    ]);
  });
});

describe("findRanges", () => {
  it("skips empty matches and stops at the limit", () => {
    expect(findRanges("abc", /x*/g)).toEqual([]);
    expect(findRanges("aaaa", /a/g, 2)).toHaveLength(2);
  });
});

describe("matchingSeqs", () => {
  const lines = [
    { seq: 1, text: "start" },
    { seq: 2, text: "ERROR one" },
    { seq: 3, text: "ok" },
    { seq: 4, text: "error two" },
  ];
  it("lists matching lines in display order", () => {
    expect(matchingSeqs(lines, must("error"))).toEqual([2, 4]);
  });
  it("limits the search to the lines some pane shows", () => {
    expect(matchingSeqs(lines, must("error"), new Set([1, 4]))).toEqual([4]);
  });
});

describe("stepMatch / positionOf", () => {
  const m = [10, 20, 30];
  it("walks forward and backward with wrap-around", () => {
    expect(stepMatch(m, 10, 1)).toBe(20);
    expect(stepMatch(m, 30, 1)).toBe(10);
    expect(stepMatch(m, 10, -1)).toBe(30);
    expect(stepMatch(m, 20, -1)).toBe(10);
  });
  it("starts at the first or last match when nothing is current", () => {
    expect(stepMatch(m, null, 1)).toBe(10);
    expect(stepMatch(m, null, -1)).toBe(30);
    expect(stepMatch(m, 99, 1)).toBe(10);
    expect(stepMatch([], null, 1)).toBeNull();
  });
  it("keeps the current line when matches are appended or inserted before it", () => {
    expect(positionOf(m, 20)).toBe(2);
    expect(positionOf([...m, 40, 50], 20)).toBe(2);
    expect(positionOf([5, ...m], 20)).toBe(3);
  });
  it("has no position when the line is gone", () => {
    expect(positionOf([10, 30], 20)).toBe(0);
    expect(positionOf(m, null)).toBe(0);
  });
});
