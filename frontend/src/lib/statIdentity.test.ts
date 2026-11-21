import { describe, expect, it } from "vitest";
import type { ContainerStat } from "@bindings/model";
import { mergeStats, sameStat } from "./statIdentity";

const st = (id: string, cpuPercent: number | null = 1, memoryBytes: number | null = 10, memoryLimit: number | null = 100) =>
  ({ id, cpuPercent, memoryBytes, memoryLimit }) as ContainerStat;

describe("sameStat", () => {
  it("is true for equal values and treats null and undefined alike", () => {
    expect(sameStat(st("a"), st("a"))).toBe(true);
    expect(sameStat({ id: "a", cpuPercent: null } as ContainerStat, { id: "a" } as ContainerStat)).toBe(true);
  });
  it("is false when any compared field differs", () => {
    expect(sameStat(st("a"), st("b"))).toBe(false);
    expect(sameStat(st("a", 1), st("a", 2))).toBe(false);
    expect(sameStat(st("a", 1, 10), st("a", 1, 11))).toBe(false);
    expect(sameStat(st("a", 1, 10, 100), st("a", 1, 10, 200))).toBe(false);
    expect(sameStat(st("a", null), st("a", 0))).toBe(false);
  });
});

describe("mergeStats", () => {
  const wanted = new Set(["a", "b"]);
  it("keeps the previous object when values are equal, takes the incoming one when they changed", () => {
    const old = st("a", 1);
    const prev = new Map([["a", old]]);
    expect(mergeStats(prev, [st("a", 1)], wanted).get("a")).toBe(old);
    const changed = st("a", 2);
    expect(mergeStats(prev, [changed], wanted).get("a")).toBe(changed);
  });
  it("drops ids that are not wanted from prev and incoming", () => {
    const prev = new Map([["x", st("x")], ["a", st("a")]]);
    const out = mergeStats(prev, [st("y"), st("b")], wanted);
    expect([...out.keys()].sort()).toEqual(["a", "b"]);
  });
  it("keeps a prev entry that has no incoming sample", () => {
    const old = st("a");
    expect(mergeStats(new Map([["a", old]]), [st("b")], wanted).get("a")).toBe(old);
  });
  it("always returns a new Map, even when nothing changed", () => {
    const old = st("a");
    const prev = new Map([["a", old]]);
    const out = mergeStats(prev, [st("a")], wanted);
    expect(out).not.toBe(prev);
    expect(out.get("a")).toBe(old);
  });
});
