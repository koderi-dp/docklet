import { describe, expect, it } from "vitest";
import type { Container, ContainerStat } from "@bindings/model";
import { cpuCores, formatMemoryUsage, formatPercent, topConsumers } from "./statsFormat";

const MB = 1024 * 1024;

describe("formatMemoryUsage", () => {
  it("shows used / limit with the percentage", () => {
    const m = formatMemoryUsage(256 * MB, 1024 * MB);
    expect(m.text).toBe("256 MB / 1 GB");
    expect(m.percent).toBe(25);
  });
  it("shows only the used amount without a limit", () => {
    expect(formatMemoryUsage(256 * MB, 0)).toEqual({ text: "256 MB" });
    expect(formatMemoryUsage(256 * MB, undefined)).toEqual({ text: "256 MB" });
  });
});

describe("percent and cores", () => {
  it("formats precision by magnitude", () => {
    expect(formatPercent(3.456)).toBe("3.5%");
    expect(formatPercent(42.4)).toBe("42%");
  });
  it("converts one-core-relative percent to cores", () => {
    expect(cpuCores(250)).toBe("2.50 cores");
  });
});

describe("topConsumers", () => {
  const c = (id: string, state = "running") => ({ id, name: id, state }) as Container;
  const containers = [c("a"), c("b"), c("c"), c("d", "exited"), c("e")];
  const stats = new Map<string, ContainerStat>([
    ["a", { id: "a", cpuPercent: 5, memoryBytes: 300 }],
    ["b", { id: "b", cpuPercent: 50 }], // no memory sample
    ["c", { id: "c", cpuPercent: 0, memoryBytes: 900 }],
    ["d", { id: "d", cpuPercent: 99, memoryBytes: 999 }], // stopped: ignored
  ]);
  it("ranks running containers and skips ones without a sample", () => {
    expect(topConsumers(containers, stats, "cpu").map((x) => x.container.id)).toEqual(["b", "a", "c"]);
    expect(topConsumers(containers, stats, "memory").map((x) => x.container.id)).toEqual(["c", "a"]);
  });
  it("limits to n and is empty before the first sample", () => {
    expect(topConsumers(containers, stats, "cpu", 1)).toHaveLength(1);
    expect(topConsumers(containers, new Map(), "cpu")).toEqual([]);
  });
});
