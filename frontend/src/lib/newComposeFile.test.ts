import { describe, expect, it } from "vitest";
import type { ComposeLocation, ComposeProject } from "@bindings/model";
import { createBlocked, defaultProjectName, folderName, nameProblem, nameState, type NameState } from "./newComposeFile";

describe("folderName", () => {
  it.each([
    ["C:\\apps\\Shop", "Shop"],
    ["/home/dev/my_app/", "my_app"],
    ["C:\\apps\\shop\\\\", "shop"],
    ["\\\\wsl$\\Ubuntu\\home\\dev\\api", "api"],
    ["", ""],
  ])("%s -> %s", (dir, want) => expect(folderName(dir)).toBe(want));
});

describe("defaultProjectName", () => {
  it.each([
    ["C:\\apps\\Shop", "shop"],
    ["/home/dev/my_app/", "my_app"],
    ["C:\\apps\\My Shop!", "myshop"],
    ["/srv/-edge", "edge"],
    ["C:\\_", ""],
    ["/srv/!!!", ""],
  ])("%s -> %s", (dir, want) => expect(defaultProjectName(dir)).toBe(want));
});

describe("nameProblem", () => {
  it("accepts a Compose project name", () => {
    expect(nameProblem("shop")).toBeUndefined();
    expect(nameProblem("my_app-2")).toBeUndefined();
  });
  it.each(["Shop", "-x", "_x", "a b", "a.b", "x".repeat(256)])("rejects %s", (n) => expect(nameProblem(n)).toMatch(/lower-case/));
});

const project = (p: Partial<ComposeProject>) => p as ComposeProject;

describe("nameState", () => {
  const projects = [project({ name: "saved", managed: true, containerIds: ["a"] }), project({ name: "live", managed: false, containerIds: ["x", "y", "z"] })];
  it("is free for an unknown name", () => expect(nameState(projects, "new")).toEqual({ kind: "free" }));
  it("is saved for a project that has its files", () => expect(nameState(projects, "saved")).toEqual({ kind: "saved" }));
  it("is a takeover for a project that only runs", () =>
    expect(nameState(projects, "live")).toEqual({ kind: "takeover", containers: 3, containerIds: ["x", "y", "z"] }));
  it("survives a project without a container list", () =>
    expect(nameState([project({ name: "live", managed: false })], "live")).toEqual({ kind: "takeover", containers: 0, containerIds: [] }));
});

describe("createBlocked", () => {
  const location = { available: true, managedAvailable: true } as ComposeLocation;
  const free: NameState = { kind: "free" };
  const ok = { location, dir: "C:\\apps\\shop", name: "shop", nameState: free, source: "minimal" as const, ticked: 0, previewReady: true };

  it("allows creation when everything is in place", () => expect(createBlocked(ok)).toBeUndefined());
  it("waits for the location", () => expect(createBlocked({ ...ok, location: undefined })).toBe("Checking where Compose runs…"));
  it("gives the connection's reason", () =>
    expect(createBlocked({ ...ok, location: { available: false, reason: "No Compose over this context" } as ComposeLocation })).toBe("No Compose over this context"));
  it("explains a missing database", () =>
    expect(createBlocked({ ...ok, location: { available: true, managedAvailable: false } as ComposeLocation })).toMatch(/application database is not open/));
  it("asks for a folder", () => expect(createBlocked({ ...ok, dir: "  " })).toBe("Choose a folder"));
  it("asks for a name when the folder gives none", () => expect(createBlocked({ ...ok, dir: "C:\\_", name: defaultProjectName("C:\\_") })).toBe("Enter a project name"));
  it("shows the naming rule", () => expect(createBlocked({ ...ok, name: "My Shop" })).toMatch(/lower-case/));
  it("refuses a saved name", () => expect(createBlocked({ ...ok, nameState: { kind: "saved" } })).toBe("A project with this name is already saved"));
  it("allows a takeover", () => expect(createBlocked({ ...ok, nameState: { kind: "takeover", containers: 2, containerIds: ["a", "b"] } })).toBeUndefined());
  it("needs a ticked container for the containers source", () => {
    expect(createBlocked({ ...ok, source: "containers", ticked: 0 })).toBe("Tick at least one container");
    expect(createBlocked({ ...ok, source: "containers", ticked: 1 })).toBeUndefined();
  });
  it("waits for the preview last", () => {
    expect(createBlocked({ ...ok, previewReady: false })).toBe("Preparing the preview…");
    expect(createBlocked({ ...ok, dir: "", previewReady: false })).toBe("Choose a folder");
  });
});
