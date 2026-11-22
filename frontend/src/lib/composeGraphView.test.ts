import { describe, expect, it } from "vitest";
import {
  kindLabel,
  matchesFind,
  portLabel,
  related,
  serviceKind,
  serviceState,
  stateLabel,
  volumeLabel,
} from "./composeGraphView";

describe("serviceKind", () => {
  it("classifies by image repository", () => {
    expect(serviceKind({ image: "postgres:16", ports: [] })).toBe("database");
    expect(serviceKind({ image: "docker.io/library/mysql", ports: [1] })).toBe("database");
    expect(serviceKind({ image: "localhost:5000/team/redis:7-alpine", ports: [] })).toBe("cache");
    expect(serviceKind({ image: "bitnami/rabbitmq@sha256:abc", ports: [] })).toBe("queue");
  });
  it("falls back on published ports", () => {
    expect(serviceKind({ image: "acme/app:1", ports: [{}] })).toBe("web");
    expect(serviceKind({ image: "acme/job", ports: [] })).toBe("image");
    expect(serviceKind({ ports: [] })).toBe("image");
  });
  it("labels", () => {
    expect(kindLabel("database")).toBe("Database");
  });
});

describe("serviceState", () => {
  const up = { state: "running" };
  const ok = { state: "exited", exitCode: 0, restartPolicy: "no" };
  const bad = { state: "exited", exitCode: 1, restartPolicy: "no" };
  it("covers the cases", () => {
    expect(serviceState([])).toBe("notCreated");
    expect(serviceState([ok])).toBe("completed");
    expect(serviceState([bad])).toBe("failed");
    expect(serviceState([up, up])).toBe("running");
    expect(serviceState([up, { state: "running", health: "unhealthy" }])).toBe("unhealthy");
    expect(serviceState([up, { state: "running", health: "starting" }])).toBe("starting");
    expect(serviceState([up, bad])).toBe("partial");
    expect(serviceState([up, { state: "exited", exitCode: 0, restartPolicy: "always" }])).toBe("partial");
    expect(serviceState([{ state: "exited", exitCode: 0, restartPolicy: "always" }])).toBe("stopped");
    expect(serviceState([{ state: "created" }])).toBe("stopped");
  });
  it("labels", () => {
    expect(stateLabel("notCreated")).toBe("Not created");
  });
});

describe("portLabel", () => {
  it("formats", () => {
    expect(portLabel({ published: "8080", target: 80 })).toBe("8080→80");
    expect(portLabel({ published: "", target: 5432 })).toBe("5432");
    expect(portLabel({ published: "5353", target: 53, protocol: "udp" })).toBe("5353→53/udp");
    expect(portLabel({ published: "", target: 53, protocol: "udp" })).toBe("53/udp");
    expect(portLabel({ published: "80", target: 80, protocol: "tcp" })).toBe("80→80");
  });
});

describe("volumeLabel", () => {
  it("shortens bind paths only", () => {
    expect(volumeLabel("/var/run/docker.sock")).toBe("docker.sock");
    expect(volumeLabel("C:\\data\\x")).toBe("x");
    expect(volumeLabel("/data/")).toBe("data");
    expect(volumeLabel("pgdata")).toBe("pgdata");
  });
});

describe("related", () => {
  const edges = [
    { from: "web", to: "api" },
    { from: "api", to: "db" },
    { from: "worker", to: "db" },
    { from: "api", to: "web" },
  ];
  it("returns ancestors and descendants but not siblings", () => {
    expect([...related("api", edges.slice(0, 3))].sort()).toEqual(["api", "db", "web"]);
    expect([...related("db", edges.slice(0, 3))].sort()).toEqual(["api", "db", "web", "worker"]);
  });
  it("terminates on cycles", () => {
    expect([...related("web", edges)].sort()).toEqual(["api", "db", "web"]);
  });
});

describe("matchesFind", () => {
  it("matches name or image case-insensitively", () => {
    expect(matchesFind({ name: "Web", image: "nginx" }, "")).toBe(true);
    expect(matchesFind({ name: "Web", image: "nginx:1" }, "WE")).toBe(true);
    expect(matchesFind({ name: "Web", image: "nginx:1" }, "ngi")).toBe(true);
    expect(matchesFind({ name: "Web" }, "ngi")).toBe(false);
  });
});
