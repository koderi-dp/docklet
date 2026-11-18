import { describe, expect, it } from "vitest";
import { composeFilesFromDrop } from "./composeDrop";

describe("composeFilesFromDrop", () => {
  it("keeps only yaml files and accepts the array-wrapped event payload", () => {
    const files = ["C:\\app\\compose.yaml", "C:\\app\\notes.txt", "/srv/docker-compose.YML"];
    expect(composeFilesFromDrop({ files })).toEqual(["C:\\app\\compose.yaml", "/srv/docker-compose.YML"]);
    expect(composeFilesFromDrop([{ files }])).toHaveLength(2);
  });
  it("is empty for anything unexpected", () => {
    expect(composeFilesFromDrop(undefined)).toEqual([]);
    expect(composeFilesFromDrop({ files: "x.yml" })).toEqual([]);
  });
});
