import { describe, expect, it } from "vitest";
import { keysAt, valuesAt, type Suggestion } from "./schema";

const labels = (s: Suggestion[]) => s.map((x) => x.label);

describe("keysAt", () => {
  it("top level", () => expect(labels(keysAt([]))).toEqual(expect.arrayContaining(["name", "services", "networks", "volumes", "secrets", "configs"])));
  it("service keys through the pattern key", () =>
    expect(labels(keysAt(["services", "web"]))).toEqual(expect.arrayContaining(["image", "ports", "environment", "healthcheck", "depends_on", "restart", "volumes"])));
  it("nested map", () =>
    expect(labels(keysAt(["services", "web", "healthcheck"]))).toEqual(expect.arrayContaining(["test", "interval", "timeout", "retries", "start_period"])));
  it("alternatives are merged", () => expect(labels(keysAt(["services", "web", "build"]))).toEqual(expect.arrayContaining(["context", "dockerfile", "args"])));
  it("map form of depends_on", () => expect(labels(keysAt(["services", "web", "depends_on", "db"]))).toEqual(expect.arrayContaining(["condition"])));
  it("list item keys", () =>
    expect(labels(keysAt(["services", "web", "volumes", "[]"]))).toEqual(expect.arrayContaining(["type", "source", "target", "read_only"])));
  it("top-level volume definition", () => expect(labels(keysAt(["volumes", "data"]))).toEqual(expect.arrayContaining(["driver", "external", "name"])));
  it("carries the schema's description", () => expect(keysAt([]).find((s) => s.label === "services")?.info).toBeTruthy());
  it("no duplicates and sorted", () => {
    const l = labels(keysAt(["services", "web"]));
    expect(l).toEqual([...new Set(l)].sort());
    expect(l.length).toBeGreaterThan(40);
  });
  it("leaves out extension patterns and deprecated keys", () => {
    expect(labels(keysAt([]))).not.toContain("version");
    expect(labels(keysAt(["services", "web"])).some((k) => k.startsWith("^"))).toBe(false);
  });
});

describe("valuesAt", () => {
  it("enum from the schema", () =>
    expect(labels(valuesAt(["services", "web", "depends_on", "db"], "condition"))).toEqual(
      expect.arrayContaining(["service_started", "service_healthy", "service_completed_successfully"]),
    ));
  it("supplement for free-text keys", () => {
    expect(labels(valuesAt(["services", "web"], "restart"))).toEqual(["always", "no", "on-failure", "unless-stopped"]);
    expect(labels(valuesAt(["services", "web"], "pull_policy"))).toEqual(expect.arrayContaining(["always", "never", "missing", "build"]));
    expect(labels(valuesAt(["services", "web"], "network_mode"))).toEqual(expect.arrayContaining(["bridge", "host", "none"]));
  });
  it("the supplement applies to services only", () => expect(valuesAt(["volumes", "data"], "restart")).toEqual([]));
  it("booleans", () => expect(labels(valuesAt(["services", "web"], "privileged"))).toEqual(["false", "true"]));
  it("never throws for keys named like built-in object members", () => {
    for (const key of ["constructor", "toString", "valueOf", "__proto__", "hasOwnProperty"]) {
      expect(valuesAt(["services", "web"], key)).toEqual([]);
      expect(keysAt(["services", "web", key])).toEqual([]);
    }
  });
  it("no fixed values for free text", () => expect(valuesAt(["services", "web"], "image")).toEqual([]));
});

describe("unknown paths", () => {
  it.each([[["services", "web", "x-custom"]], [["nonsense"]], [["services", "web", "image", "deeper"]], [["services", "web", "healthcheck", "[]"]]])(
    "%j gives nothing",
    (p) => {
      expect(keysAt(p)).toEqual([]);
      expect(valuesAt(p, "anything")).toEqual([]);
    },
  );
});
