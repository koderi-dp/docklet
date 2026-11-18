import { describe, expect, it } from "vitest";
import { baseName, breadcrumbs, joinPath, normalizeTyped, parentPath } from "./paths";

describe("paths", () => {
  it("handles parents, joins and names", () => {
    expect(parentPath("/usr/local/bin")).toBe("/usr/local");
    expect(parentPath("/usr")).toBe("/");
    expect(parentPath("/")).toBe("/");
    expect(joinPath("/", "etc")).toBe("/etc");
    expect(joinPath("/etc", "hosts")).toBe("/etc/hosts");
    expect(baseName("/etc/hosts")).toBe("hosts");
  });
  it("builds breadcrumbs", () => {
    expect(breadcrumbs("/a/b").map((c) => c.path)).toEqual(["/", "/a", "/a/b"]);
  });
  it("normalises typed paths and rejects relative ones", () => {
    expect(normalizeTyped(" /a//b/../c/ ")).toBe("/a/c");
    expect(normalizeTyped("a/b")).toBeNull();
    expect(normalizeTyped("/..")).toBe("/");
  });
});
