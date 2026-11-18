import { describe, expect, it } from "vitest";
import { makeEntry, type LogEntry } from "./logEntry";
import { capEntries, mergeEntries, sortByTime } from "./logMerge";

let seq = 0;
const entry = (container: string, iso: string | undefined, text = "x"): LogEntry =>
  makeEntry({ timestamp: iso ?? "", stream: "stdout", text }, container, seq++, 0);
const order = (l: readonly LogEntry[]) => l.map((e) => e.text);

describe("makeEntry", () => {
  it("takes the time from the Engine timestamp and detects the level", () => {
    const e = makeEntry({ timestamp: "2026-01-02T03:04:05.123456789Z", stream: "stderr", text: "\u001b[31mERROR boom\u001b[0m" }, "web", 1, 0);
    expect(e.text).toBe("ERROR boom");
    expect(e.level).toBe("error");
    expect(e.time).toBe(Date.parse("2026-01-02T03:04:05.123Z"));
    expect(e.sortKey).toBe(e.time);
  });

  it("never invents a time: an unreadable timestamp keeps the fallback order key", () => {
    const e = makeEntry({ timestamp: "", stream: "stdout", text: "hi" }, "web", 1, 777);
    expect(e.time).toBeUndefined();
    expect(e.sortKey).toBe(777);
  });
});

describe("mergeEntries", () => {
  it("appends a newer batch and interleaves an older one by time", () => {
    const a = [entry("a", "2026-01-01T00:00:01Z", "a1"), entry("a", "2026-01-01T00:00:05Z", "a5")];
    const b = [entry("b", "2026-01-01T00:00:03Z", "b3"), entry("b", "2026-01-01T00:00:09Z", "b9")];
    expect(order(mergeEntries(a, b))).toEqual(["a1", "b3", "a5", "b9"]);
    expect(order(mergeEntries(a, [entry("a", "2026-01-01T00:00:06Z", "a6")]))).toEqual(["a1", "a5", "a6"]);
  });

  it("orders an unsorted batch and breaks equal times by arrival", () => {
    const t = "2026-01-01T00:00:01Z";
    const merged = mergeEntries([], [entry("b", "2026-01-01T00:00:02Z", "late"), entry("a", t, "first"), entry("b", t, "second")]);
    expect(order(merged)).toEqual(["first", "second", "late"]);
  });
});

describe("capEntries", () => {
  it("drops the oldest lines of a noisy container only", () => {
    const lines: LogEntry[] = [];
    for (let i = 0; i < 6; i++) lines.push(entry("noisy", `2026-01-01T00:00:0${i}Z`, `n${i}`));
    lines.push(entry("quiet", "2026-01-01T00:00:07Z", "q"));
    const r = capEntries(lines, 3, 100);
    expect(order(r.lines)).toEqual(["n3", "n4", "n5", "q"]);
    expect(r.dropped).toBe(3);
  });

  it("applies the overall cap after the per-container one", () => {
    const lines = [entry("a", "2026-01-01T00:00:01Z", "1"), entry("b", "2026-01-01T00:00:02Z", "2"), entry("a", "2026-01-01T00:00:03Z", "3")];
    const r = capEntries(lines, 10, 2);
    expect(order(r.lines)).toEqual(["2", "3"]);
    expect(r.dropped).toBe(1);
  });
});

describe("sortByTime", () => {
  it("puts lines delivered out of order (stdout and stderr interleaved by arrival) in timestamp order", () => {
    const l = [entry("a", "2026-01-01T04:09:57.911Z", "late"), entry("a", "2026-01-01T04:09:49.252Z", "early"), entry("a", "2026-01-01T04:09:57.911Z", "late2")];
    expect(order(sortByTime(l))).toEqual(["early", "late", "late2"]);
  });

  it("returns the very same list when it is already in order", () => {
    const l = [entry("a", "2026-01-01T00:00:01Z", "1"), entry("a", "2026-01-01T00:00:02Z", "2")];
    expect(sortByTime(l)).toBe(l);
  });
});
