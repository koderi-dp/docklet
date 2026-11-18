import { describe, expect, it } from "vitest";
import type { ComposeProjectDisk } from "@bindings/model";
import { projectData } from "./projectDisk";

const base: ComposeProjectDisk = {
  project: "p",
  containersBytes: null,
  containerCount: 0,
  containersKnown: 0,
  volumesBytes: null,
  volumeCount: 0,
  volumesKnown: 0,
  imagesBytes: 5000,
  imageCount: 1,
  imagesKnown: 1,
};

describe("projectData", () => {
  it("sums containers and volumes but never images", () => {
    const r = projectData({ ...base, containersBytes: 100, containerCount: 2, containersKnown: 2, volumesBytes: 400, volumeCount: 1, volumesKnown: 1 });
    expect(r).toEqual({ bytes: 500, note: undefined });
  });
  it("is null when the Engine gave no size for either part", () => {
    expect(projectData(base)).toBeNull();
    expect(projectData(undefined)).toBeNull();
  });
  it("flags partial parts", () => {
    const r = projectData({ ...base, containersBytes: 100, containerCount: 3, containersKnown: 2 });
    expect(r?.bytes).toBe(100);
    expect(r?.note).toContain("2 of 3 containers");
  });
  it("does not complain about a project without volumes", () => {
    expect(projectData({ ...base, containersBytes: 10, containerCount: 1, containersKnown: 1 })?.note).toBeUndefined();
  });
});
