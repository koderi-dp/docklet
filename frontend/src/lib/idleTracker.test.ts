import { describe, expect, it } from "vitest";
import { formatIdleDuration, IdleTracker, type IdleObservation } from "./idleTracker";

const cfg = { cpuThreshold: 1, minMs: 10_000, maxGapMs: 3_000 };
const quiet: IdleObservation = { cpuPercent: 0.4, complete: true, membersKey: "a,b" };

function run(t: IdleTracker, from: number, to: number, o = quiet, step = 1000) {
  for (let n = from; n <= to; n += step) t.observe("p", n, o);
}

describe("IdleTracker", () => {
  it("reports idle only after the quiet window reached its minimum", () => {
    const t = new IdleTracker(cfg);
    run(t, 0, 9000);
    expect(t.idle("p", 9000)).toBeNull();
    run(t, 10_000, 12_000);
    expect(t.idle("p", 12_000)).toEqual({ since: 0, idleForMs: 12_000 });
  });
  it("a CPU spike restarts the window", () => {
    const t = new IdleTracker(cfg);
    run(t, 0, 8000);
    t.observe("p", 9000, { ...quiet, cpuPercent: 40 });
    run(t, 10_000, 15_000);
    expect(t.idle("p", 15_000)).toBeNull();
    run(t, 16_000, 20_000);
    expect(t.idle("p", 20_000)?.since).toBe(10_000);
  });
  it("partial or missing samples never count as quiet", () => {
    const t = new IdleTracker(cfg);
    run(t, 0, 20_000, { ...quiet, complete: false });
    expect(t.idle("p", 20_000)).toBeNull();
    run(t, 21_000, 40_000, { cpuPercent: null, complete: false, membersKey: "a,b" });
    expect(t.idle("p", 40_000)).toBeNull();
  });
  it("a gap in observations means we were not watching and restarts the window", () => {
    const t = new IdleTracker(cfg);
    run(t, 0, 8000);
    run(t, 20_000, 25_000);
    expect(t.idle("p", 25_000)).toBeNull();
    expect(t.idle("p", 40_000)).toBeNull(); // stale observations are not idle either
    run(t, 26_000, 31_000);
    expect(t.idle("p", 31_000)?.since).toBe(20_000);
  });
  it("a change of the running set restarts the window", () => {
    const t = new IdleTracker(cfg);
    run(t, 0, 9000);
    t.observe("p", 10_000, { ...quiet, membersKey: "a,b,c" });
    expect(t.idle("p", 10_000)).toBeNull();
  });
});

describe("formatIdleDuration", () => {
  it("formats minutes and hours", () => {
    expect(formatIdleDuration(12 * 60_000)).toBe("12 min");
    expect(formatIdleDuration(65 * 60_000)).toBe("1 h 5 min");
    expect(formatIdleDuration(120 * 60_000)).toBe("2 h");
  });
});
