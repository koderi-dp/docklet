import { describe, expect, it } from "vitest";
import type { Container } from "@bindings/model";
import { buildRows, countRows, filterRows, imageSizeFor } from "./serviceRows";

const c = (name: string, service: string | undefined, over: Partial<Container> = {}): Container =>
  ({ id: `id-${name}`, name, image: "img", state: "running", status: "Up", health: "none", created: 1, ports: null, composeService: service, ...over }) as Container;

describe("buildRows", () => {
  it("orders services as defined, then unknown ones, then unlabelled containers last", () => {
    const rows = buildRows(["web", "db"], [c("loose", undefined), c("cache-1", "cache"), c("db-1", "db"), c("web-1", "web")]);
    expect(rows.map((r) => r.service)).toEqual(["web", "db", "cache", ""]);
  });

  it("sorts replicas by numeric name and numbers them", () => {
    const rows = buildRows(["web"], [c("web-10", "web"), c("web-2", "web")]);
    expect(rows.map((r) => [r.container?.name, r.replica, r.replicas])).toEqual([
      ["web-2", 1, 2],
      ["web-10", 2, 2],
    ]);
  });

  it("gives a service without container one row without container", () => {
    const rows = buildRows(["web", "job"], [c("web-1", "web")]);
    expect(rows[1]).toMatchObject({ service: "job", container: undefined, replica: 1, replicas: 1, running: false });
  });
});

describe("imageSizeFor", () => {
  const images = [
    { id: "sha256:abcdef0123456789", repository: "nginx", tag: "1.27", size: 100 },
    { id: "sha256:1111111111111111", repository: "redis", tag: "latest", size: 50 },
    { id: "sha256:2222222222222222", repository: "", tag: "", size: 7 },
  ];
  it("matches by repo tag, implicit latest, and image id", () => {
    expect(imageSizeFor({ image: "nginx:1.27" }, images)).toBe(100);
    expect(imageSizeFor({ image: "redis" }, images)).toBe(50);
    expect(imageSizeFor({ image: "sha256:222222222222" }, images)).toBe(7);
    expect(imageSizeFor({ image: "nginx:1.26" }, images)).toBeUndefined();
  });
});

describe("countRows and filterRows", () => {
  const rows = buildRows(
    ["web", "db", "migrate", "bad", "job"],
    [
      c("web-1", "web"),
      c("web-2", "web", { state: "exited", exitCode: 0, restartPolicy: "always" }),
      c("db-1", "db"),
      c("migrate-1", "migrate", { state: "exited", exitCode: 0, restartPolicy: "no" }),
      c("bad-1", "bad", { state: "exited", exitCode: 1 }),
    ],
  );

  it("counts every non-running row as stopped, including completed, failed and not created", () => {
    expect(countRows(rows)).toEqual({ all: 6, running: 2, stopped: 4 });
  });

  it("filters to exactly the counted rows", () => {
    expect(filterRows(rows, "running").map((r) => r.key)).toEqual(["id-web-1", "id-db-1"]);
    expect(filterRows(rows, "stopped")).toHaveLength(4);
    expect(filterRows(rows, "all")).toHaveLength(6);
  });
});
