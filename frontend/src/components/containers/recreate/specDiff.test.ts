import { describe, expect, it } from "vitest";
import type { ContainerCreateSpec } from "@bindings/model";
import { diffSpecs, isTagChange } from "./specDiff";

const base: ContainerCreateSpec = {
  name: "web", image: "nginx:1.26", command: "", env: [{ key: "MODE", value: "blue", keep: false }, { key: "DB_PASSWORD", value: "", keep: true }],
  ports: [{ hostIp: "", hostPort: 8080, containerPort: 80, protocol: "tcp" }], mounts: [], restartPolicy: "always", maxRetries: 0,
  memoryMiB: 0, cpus: 0, autoStart: true, entrypoint: "", workDir: "", user: "", hostname: "", labels: null, stopSignal: "", stopTimeout: null,
  networkMode: "", networks: null,
};

describe("diffSpecs", () => {
  it("reports nothing when the spec is untouched", () => {
    expect(diffSpecs(base, structuredClone(base))).toEqual([]);
  });

  it("describes image, env and port changes without revealing secrets", () => {
    const after: ContainerCreateSpec = {
      ...structuredClone(base),
      image: "nginx:1.27",
      env: [{ key: "MODE", value: "green", keep: false }, { key: "DB_PASSWORD", value: "new-secret-value", keep: false }],
      ports: [{ hostIp: "", hostPort: 9090, containerPort: 80, protocol: "tcp" }],
    };
    const changes = diffSpecs(base, after);
    expect(changes.map((c) => c.label)).toEqual(["Image", "Env MODE", "Env DB_PASSWORD", "Port removed", "Port added"]);
    expect(JSON.stringify(changes)).not.toContain("new-secret-value");
  });

  it("treats a kept secret as unchanged", () => {
    expect(diffSpecs(base, { ...structuredClone(base), env: [...(base.env ?? [])].reverse() })).toEqual([]);
  });
});

describe("isTagChange", () => {
  it("is true for the same repository with another tag", () => {
    expect(isTagChange("nginx:1.26", "nginx:1.27")).toBe(true);
    expect(isTagChange("nginx:1.26", "nginx:1.26")).toBe(false);
    expect(isTagChange("nginx:1.26", "redis:7")).toBe(false);
    expect(isTagChange("localhost:5000/app", "localhost:5000/app:2")).toBe(true);
  });
});
