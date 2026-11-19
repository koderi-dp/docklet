import { describe, expect, it } from "vitest";
import type { ImageRow } from "@bindings/model";
import { IMAGE_SORT, matchesImage, quickRefProblem, summarizeImages } from "./imageFilters";

const row = (o: Partial<ImageRow> = {}): ImageRow =>
  ({ id: "sha256:abcdef123456", ref: "nginx:1.27", size: 100, inUse: false, dangling: false, containerCount: 0, created: 1000, ...o }) as ImageRow;

describe("matchesImage", () => {
  it("filters by usage and dangling state", () => {
    const used = row({ inUse: true });
    const unused = row();
    const dangling = row({ dangling: true });
    expect(matchesImage(used, "inuse", "")).toBe(true);
    expect(matchesImage(unused, "inuse", "")).toBe(false);
    expect(matchesImage(unused, "unused", "")).toBe(true);
    expect(matchesImage(used, "unused", "")).toBe(false);
    expect(matchesImage(dangling, "dangling", "")).toBe(true);
    expect(matchesImage(unused, "dangling", "")).toBe(false);
    expect(matchesImage(used, "all", "")).toBe(true);
  });
  it("matches the reference case-insensitively and ignores surrounding spaces", () => {
    expect(matchesImage(row({ ref: "Docker.io/Library/Nginx:1.27" }), "all", "  nginx ")).toBe(true);
    expect(matchesImage(row(), "all", "redis")).toBe(false);
  });
  it("matches the id with or without the sha256: prefix", () => {
    expect(matchesImage(row(), "all", "abcdef")).toBe(true);
    expect(matchesImage(row(), "all", "sha256:abcdef")).toBe(true);
    expect(matchesImage(row(), "all", "SHA256:ABC")).toBe(true);
    expect(matchesImage(row(), "all", "ffff")).toBe(false);
  });
  it("does not match the 'sha256' prefix text itself against the id", () => {
    expect(matchesImage(row({ ref: "nginx" }), "all", "256:ab")).toBe(false);
  });
  it("combines the filter with the query (both must hold)", () => {
    expect(matchesImage(row({ inUse: true }), "unused", "nginx")).toBe(false);
  });
});

describe("summarizeImages", () => {
  it("is all zeros for no rows", () => {
    expect(summarizeImages([])).toEqual({ images: 0, tagRows: 0, totalSize: 0, unused: 0, dangling: 0 });
  });
  it("counts an image with several tags once for images/size/unused but every tag row in tagRows", () => {
    const rows = [row({ ref: "a:1", size: 50 }), row({ ref: "a:2", size: 50 }), row({ id: "sha256:other", ref: "b:1", size: 30, inUse: true })];
    expect(summarizeImages(rows)).toEqual({ images: 2, tagRows: 3, totalSize: 80, unused: 1, dangling: 0 });
  });
  it("counts dangling rows", () => {
    const rows = [row({ id: "sha256:x", dangling: true, ref: "sha256:x" }), row({ id: "sha256:y", dangling: true, ref: "sha256:y" }), row()];
    expect(summarizeImages(rows).dangling).toBe(2);
  });
});

describe("IMAGE_SORT accessors", () => {
  it("sorts dangling images last by name (no name)", () => {
    expect(IMAGE_SORT.name(row({ dangling: true, ref: "sha256:x" }))).toBeUndefined();
    expect(IMAGE_SORT.name(row())).toBe("nginx:1.27");
  });
  it("treats a zero created time as unknown", () => {
    expect(IMAGE_SORT.created(row({ created: 0 }))).toBeUndefined();
    expect(IMAGE_SORT.created(row({ created: 5 }))).toBe(5);
  });
  it("sorts by size and container count numerically", () => {
    expect(IMAGE_SORT.size(row({ size: 7 }))).toBe(7);
    expect(IMAGE_SORT.inuse(row({ containerCount: 3 }))).toBe(3);
  });
});

describe("quickRefProblem", () => {
  it("accepts typical references", () => {
    for (const r of ["nginx", "nginx:1.27", "library/nginx", "ghcr.io/org/app:v1.2.3", "localhost:5000/app", "app@sha256:0123abcd", "  nginx:latest  "]) {
      expect(quickRefProblem(r), r).toBeNull();
    }
  });
  it("asks for input when empty or blank", () => {
    expect(quickRefProblem("")).toMatch(/Enter an image reference/);
    expect(quickRefProblem("   ")).toMatch(/Enter an image reference/);
  });
  it("rejects inner whitespace", () => {
    expect(quickRefProblem("nginx latest")).toMatch(/spaces/);
    expect(quickRefProblem("a\tb")).toMatch(/spaces/);
  });
  it("rejects a leading separator and illegal characters", () => {
    for (const r of ["-nginx", ".nginx", "/nginx", ":1.27", "nginx!", "ngi$nx", "ngínx"]) {
      expect(quickRefProblem(r), r).toMatch(/Invalid characters/);
    }
  });
  it("rejects a reference ending in ':' or '/'", () => {
    expect(quickRefProblem("nginx:")).toMatch(/incomplete/);
    expect(quickRefProblem("library/")).toMatch(/incomplete/);
  });
});
