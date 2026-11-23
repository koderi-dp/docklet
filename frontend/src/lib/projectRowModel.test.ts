import { describe, expect, it } from "vitest";
import type { ComposeProject, Container } from "@bindings/model";
import {
  configSubline,
  countProjectFilters,
  filterProjects,
  isProjectRunning,
  projectBar,
  projectHealthLine,
} from "./projectRowModel";

function c(over: Partial<Omit<Container, "health">> & { health?: string; id: string }): Container {
  return {
    connectionId: "A",
    name: over.id,
    image: "img:1",
    state: "running",
    status: "Up",
    health: "none",
    created: 0,
    ports: [],
    ...over,
  } as Container;
}

const running = (id: string, health = "none") => c({ id, state: "running", health });
const failed = (id: string) => c({ id, state: "exited", exitCode: 137, restartPolicy: "unless-stopped" });
const completed = (id: string) => c({ id, state: "exited", exitCode: 0, restartPolicy: "no" });
const stopped = (id: string) => c({ id, state: "created" });

function proj(over: Record<string, unknown> = {}): Pick<ComposeProject, "services" | "notCreated"> {
  return { services: [], notCreated: false, ...over } as Pick<ComposeProject, "services" | "notCreated">;
}
const svc = (total: number) => ({ name: "s", total }) as unknown as NonNullable<ComposeProject["services"]>[number];

describe("projectBar", () => {
  it("all running", () => {
    const b = projectBar(proj(), [running("1"), running("2"), running("3")]);
    expect(b.segments).toEqual(["running", "running", "running"]);
    expect(b.summary).toBe("3 of 3 running");
    expect(b.label).toBe("3 running");
    expect(b.total).toBe(3);
  });

  it("stopped containers (created, exited without a code) land in stopped", () => {
    const b = projectBar(proj(), [stopped("1"), c({ id: "2", state: "exited" }), c({ id: "3", state: "paused" })]);
    expect(b.counts.stopped).toBe(3);
    expect(b.summary).toBe("0 of 3 running");
  });

  it("partial with failed, completed, stopped and not created services", () => {
    const b = projectBar(proj({ services: [svc(1), svc(0), svc(0)] }), [
      running("1"),
      running("2"),
      running("3"),
      failed("4"),
      completed("5"),
      completed("6"),
      stopped("7"),
    ]);
    expect(b.counts).toEqual({ running: 3, failed: 1, completed: 2, stopped: 1, notCreated: 2 });
    expect(b.segments).toEqual([
      "running", "running", "running", "failed", "completed", "completed", "stopped", "notCreated", "notCreated",
    ]);
    expect(b.total).toBe(9);
    expect(b.running).toBe(3);
    expect(b.summary).toBe("3 of 9 running · 2 completed");
    expect(b.label).toBe("3 running, 1 failed, 2 completed, 1 stopped, 2 not created");
  });

  it("a not created project has services with total 0 and no containers", () => {
    const b = projectBar(proj({ notCreated: true, services: [svc(0), svc(0)] }), []);
    expect(b.segments).toEqual(["notCreated", "notCreated"]);
    expect(b.summary).toBe("0 of 2 running");
    expect(b.label).toBe("2 not created");
  });

  it("no containers and no services", () => {
    for (const services of [[], null, undefined]) {
      const b = projectBar(proj({ services }), []);
      expect(b.segments).toEqual([]);
      expect(b.total).toBe(0);
      expect(b.summary).toBe("No containers");
      expect(b.label).toBe("No containers");
    }
  });
});

describe("projectHealthLine", () => {
  it("is undefined without healthchecks or failures", () => {
    expect(projectHealthLine([running("1"), completed("2")])).toBeUndefined();
    expect(projectHealthLine([])).toBeUndefined();
  });
  it("counts healthy of those with a healthcheck, muted", () => {
    expect(projectHealthLine([running("1", "healthy"), running("2", "starting"), running("3")])).toEqual({
      text: "1 of 2 healthy",
      tone: "muted",
    });
  });
  it("is danger when unhealthy", () => {
    expect(projectHealthLine([running("1", "healthy"), running("2", "unhealthy")])).toEqual({
      text: "1 of 2 healthy",
      tone: "danger",
    });
  });
  it("adds failed and is danger", () => {
    expect(projectHealthLine([running("1", "healthy"), failed("2")])).toEqual({
      text: "1 of 1 healthy · 1 failed",
      tone: "danger",
    });
    expect(projectHealthLine([failed("1")])).toEqual({ text: "1 failed", tone: "danger" });
  });
});

describe("filters", () => {
  it("isProjectRunning prefers the container list", () => {
    expect(isProjectRunning({ running: 0 }, [running("1")])).toBe(true);
    expect(isProjectRunning({ running: 5 }, [stopped("1")])).toBe(false);
    expect(isProjectRunning({ running: 2 }, undefined)).toBe(true);
    expect(isProjectRunning({ running: 0 }, undefined)).toBe(false);
  });
  it("counts and filters keep order", () => {
    const items = [
      { n: "a", r: true },
      { n: "b", r: false },
      { n: "c", r: true },
    ];
    const r = (t: { r: boolean }) => t.r;
    expect(countProjectFilters(items, r)).toEqual({ all: 3, running: 2, stopped: 1 });
    expect(filterProjects(items, "all", r).map((t) => t.n)).toEqual(["a", "b", "c"]);
    expect(filterProjects(items, "running", r).map((t) => t.n)).toEqual(["a", "c"]);
    expect(filterProjects(items, "stopped", r).map((t) => t.n)).toEqual(["b"]);
  });
});

describe("configSubline", () => {
  const cap = (executionLocation: string) => ({ executionLocation }) as ComposeProject["capability"];
  it("uses the base name of a Windows path", () => {
    expect(
      configSubline({ configFiles: ["C:\\src\\app\\compose.yaml"], capability: cap("This machine"), source: "labels" }),
    ).toBe("compose.yaml · This machine");
  });
  it("uses the base name of a WSL path, +N for more files, and drops the WSL prefix", () => {
    expect(
      configSubline({
        configFiles: ["/home/u/app/docker-compose.yml", "/home/u/app/override.yml", "/x/y.yml"],
        capability: cap("WSL: Ubuntu-24.04"),
        source: "labels",
      }),
    ).toBe("docker-compose.yml +2 · Ubuntu-24.04");
  });
  it("falls back to the source without a config file", () => {
    expect(configSubline({ configFiles: null, capability: cap("WSL: Ubuntu-24.04"), source: "Container labels" })).toBe(
      "Container labels · Ubuntu-24.04",
    );
    expect(configSubline({ configFiles: [], capability: cap(""), source: "Container labels" })).toBe("Container labels");
  });
});
