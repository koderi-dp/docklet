import { describe, expect, it } from "vitest";
import { sameImageRef } from "./refMatch";

describe("sameImageRef", () => {
  it("treats Docker Hub defaults as equal", () => {
    expect(sameImageRef("nginx", "nginx:latest")).toBe(true);
    expect(sameImageRef("docker.io/library/nginx:1.27", "nginx:1.27")).toBe(true);
  });

  it("distinguishes tags, repositories and registries with ports", () => {
    expect(sameImageRef("nginx:1.26", "nginx:1.27")).toBe(false);
    expect(sameImageRef("grafana/grafana", "grafana/grafana:latest")).toBe(true);
    expect(sameImageRef("localhost:5000/app", "localhost:5000/app:latest")).toBe(true);
    expect(sameImageRef("localhost:5000/app", "localhost:5000/app:2")).toBe(false);
  });
});
