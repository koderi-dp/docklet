import { describe, expect, it } from "vitest";
import { segmentLine } from "./ansiSegments";

const red = { fg: 1 };
const bold = { bold: true as const };

describe("segmentLine", () => {
  it("covers the whole line in order, styled pieces carrying their style", () => {
    expect(segmentLine(10, [{ start: 2, end: 5, style: red }], [])).toEqual([
      { start: 0, end: 2, mark: -1 },
      { start: 2, end: 5, mark: -1, style: red },
      { start: 5, end: 10, mark: -1 },
    ]);
  });
  it("splits a match that starts in one styled range and ends in another, keeping one match index", () => {
    const spans = [
      { start: 0, end: 4, style: red },
      { start: 4, end: 8, style: bold },
    ];
    expect(segmentLine(8, spans, [{ start: 2, end: 6 }])).toEqual([
      { start: 0, end: 2, mark: -1, style: red },
      { start: 2, end: 4, mark: 0, style: red },
      { start: 4, end: 6, mark: 0, style: bold },
      { start: 6, end: 8, mark: -1, style: bold },
    ]);
  });
  it("numbers the matches and handles a match that runs past a styled range into plain text", () => {
    const segs = segmentLine(
      12,
      [{ start: 3, end: 6, style: red }],
      [
        { start: 0, end: 1 },
        { start: 5, end: 9 },
      ],
    );
    expect(segs.map((s) => [s.start, s.end, s.mark, s.style !== undefined])).toEqual([
      [0, 1, 0, false],
      [1, 3, -1, false],
      [3, 5, -1, true],
      [5, 6, 1, true],
      [6, 9, 1, false],
      [9, 12, -1, false],
    ]);
  });
  it("gives no pieces for an empty line", () => {
    expect(segmentLine(0, [], [])).toEqual([]);
  });
});
