import { describe, expect, it } from "vitest";
import { compileSearch, matchRanges, segments } from "./logSearch";

describe("compileSearch", () => {
  it("treats plain text literally and case-insensitively", () => {
    const { re, error } = compileSearch("a.b(", false);
    expect(error).toBeNull();
    expect(matchRanges("xx A.B( yy aXb(", re!)).toEqual([[3, 7]]);
  });
  it("supports regular expressions", () => {
    const { re } = compileSearch("err(or)?\\s+\\d+", true);
    expect(matchRanges("Error 42, err 7, errors", re!)).toEqual([[0, 8], [10, 15]]);
  });
  it("reports an invalid regex and filters nothing", () => {
    const s = compileSearch("(unclosed", true);
    expect(s.re).toBeNull();
    expect(s.error).toBeTruthy();
  });
  it("an empty query is no filter and no error", () => {
    expect(compileSearch("", true)).toEqual({ re: null, error: null });
  });
});

describe("matchRanges", () => {
  it("skips empty matches instead of looping forever", () => {
    const { re } = compileSearch("x*", true);
    expect(matchRanges("axxb", re!)).toEqual([[1, 3]]);
  });
});

describe("segments", () => {
  it("splits a line around matches and numbers them", () => {
    expect(segments("a-ERR-b-ERR", [[2, 5], [8, 11]])).toEqual([
      { text: "a-", match: false },
      { text: "ERR", match: true, index: 0 },
      { text: "-b-", match: false },
      { text: "ERR", match: true, index: 1 },
    ]);
  });
});
