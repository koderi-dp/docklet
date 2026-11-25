import { describe, expect, it } from "vitest";
import type { ComposeClonePreview } from "@bindings/model";
import { cloneBlocked, cloneSizeText, renameBlocked, renameUnavailable } from "./cloneProject";

const projects = [{ name: "shop" }, { name: "taken" }];

describe("renameUnavailable", () => {
  it("allows a saved project without containers", () => expect(renameUnavailable({ managed: true, total: 0 })).toBeUndefined());
  it("refuses a project with containers and says what to do", () =>
    expect(renameUnavailable({ managed: true, total: 2 })).toMatch(/Run Down first, or clone/));
  it("refuses a project that is only known from containers", () => expect(renameUnavailable({ managed: false, total: 0 })).toMatch(/saved project/));
});

describe("renameBlocked", () => {
  const ok = { current: "shop", next: "store", projects };
  it("accepts a free, valid name", () => expect(renameBlocked(ok)).toBeUndefined());
  it("ignores surrounding spaces", () => expect(renameBlocked({ ...ok, next: " store " })).toBeUndefined());
  it("asks for a name", () => expect(renameBlocked({ ...ok, next: "  " })).toBe("Enter a project name"));
  it("refuses the current name", () => expect(renameBlocked({ ...ok, next: "shop" })).toBe("That is the current name"));
  it("shows the naming rule", () => expect(renameBlocked({ ...ok, next: "My Shop" })).toMatch(/lower-case/));
  it("refuses a name in use", () => expect(renameBlocked({ ...ok, next: "taken" })).toMatch(/already exists/));
});

describe("cloneBlocked", () => {
  const preview = { blocked: "" } as ComposeClonePreview;
  const ok = { preview, name: "shop-copy", dir: "C:\\apps\\shop-copy", projects };
  it("allows the copy when everything is in place", () => expect(cloneBlocked(ok)).toBeUndefined());
  it("waits for the preview", () => expect(cloneBlocked({ ...ok, preview: undefined })).toBe("Checking the project…"));
  it("gives the backend's reason first", () =>
    expect(cloneBlocked({ ...ok, name: "", preview: { blocked: "The project has no folder to copy." } as ComposeClonePreview })).toBe("The project has no folder to copy."));
  it("refuses the original's own name", () => expect(cloneBlocked({ ...ok, name: "shop" })).toMatch(/already exists/));
  it("shows the naming rule", () => expect(cloneBlocked({ ...ok, name: "Shop Copy" })).toMatch(/lower-case/));
  it("asks for a folder", () => expect(cloneBlocked({ ...ok, dir: " " })).toBe("Choose a folder for the copy"));
});

describe("cloneSizeText", () => {
  it.each([
    [{ sizeKnown: true, sizeBytes: 512, files: 1 }, "1 file, 512 B"],
    [{ sizeKnown: true, sizeBytes: 1536, files: 3 }, "3 files, 1.5 KB"],
    [{ sizeKnown: true, sizeBytes: 3 * 1024 ** 3, files: 12345 }, "12,345 files, 3.0 GB"],
  ])("%o -> %s", (p, want) => expect(cloneSizeText(p)).toBe(want));
  it("says so when the size is unknown", () => expect(cloneSizeText({ sizeKnown: false, sizeBytes: 0, files: 0 })).toMatch(/could not be measured/));
});
