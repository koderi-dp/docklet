import { describe, expect, it } from "vitest";
import { afterSave, barState, fileLabel, isDirty, pickFile, savedContent, type EditState } from "./composeFileState";

const state: EditState = { loadedText: "a: 1\n", text: "a: 2\n", version: "v1", conflict: false, error: null };
const res = (r: object) => r as Parameters<typeof afterSave>[0];

describe("savedContent", () => {
  it("is the text that was sent with the new version", () => {
    expect(savedContent("/p/c.yml", "a: 2\n", "v2")).toEqual({ path: "/p/c.yml", text: "a: 2\n", version: "v2", readOnly: false });
  });
});

describe("barState", () => {
  it("picks read-only, unsaved, saved-not-applied, saved in that order", () => {
    expect(barState(true, true, true)).toBe("Read-only");
    expect(barState(false, true, true)).toBe("Unsaved changes");
    expect(barState(false, false, true)).toBe("Saved, not applied");
    expect(barState(false, false, false)).toBe("Saved");
  });
});

describe("isDirty", () => {
  it("is false only when the texts are identical", () => {
    expect(isDirty("a\n", "a\n")).toBe(false);
    expect(isDirty("a\n", "a\n ")).toBe(true);
  });
});

describe("afterSave", () => {
  it("clears dirty and stores the new version on saved", () => {
    const next = afterSave(res({ saved: true, version: "v2" }), state);
    expect(isDirty(next.loadedText, next.text)).toBe(false);
    expect(next.version).toBe("v2");
    expect(next.conflict).toBe(false);
  });
  it("stays dirty for text typed while the save ran", () => {
    const next = afterSave(res({ saved: true, version: "v2" }), { ...state, text: "a: 3\n" }, "a: 2\n");
    expect(next.loadedText).toBe("a: 2\n");
    expect(isDirty(next.loadedText, next.text)).toBe(true);
  });
  it("keeps the text and flags a conflict", () => {
    const next = afterSave(res({ conflict: true }), state);
    expect(next).toMatchObject({ text: "a: 2\n", loadedText: "a: 1\n", version: "v1", conflict: true });
  });
  it("keeps the text and stores the YAML error", () => {
    const next = afterSave(res({ error: { line: 4, message: "bad" } }), state);
    expect(next.text).toBe("a: 2\n");
    expect(next.error).toEqual({ line: 4, message: "bad" });
    expect(next.conflict).toBe(false);
  });
});

describe("pickFile", () => {
  const files = [
    { path: "/p/a.yml", kind: "config" },
    { path: "/p/.env", kind: "env" },
    { path: "/p/b.yml", kind: "config" },
  ];
  it("keeps the wanted path while it is listed", () => {
    expect(pickFile(files, "/p/b.yml")).toBe("/p/b.yml");
  });
  it("falls back to the first config file", () => {
    expect(pickFile(files, "/gone.yml")).toBe("/p/a.yml");
    expect(pickFile([files[1], files[2]], "")).toBe("/p/b.yml");
  });
  it("returns empty when there is no config file", () => {
    expect(pickFile([files[1]], "")).toBe("");
  });
});

describe("fileLabel", () => {
  it("uses the base name for both separators", () => {
    expect(fileLabel("/srv/app/compose.yaml")).toBe("compose.yaml");
    expect(fileLabel("C:\\srv\\app\\compose.yaml")).toBe("compose.yaml");
  });
  it("adds the parent folder when base names collide", () => {
    const all = ["/srv/app/compose.yaml", "/srv/over/compose.yaml", "/srv/app/.env"];
    expect(fileLabel(all[0], all)).toBe("app/compose.yaml");
    expect(fileLabel("C:\\srv\\over\\compose.yaml", ["C:\\srv\\over\\compose.yaml", "C:\\srv\\app\\compose.yaml"])).toBe("over/compose.yaml");
    expect(fileLabel(all[2], all)).toBe(".env");
  });
});
