import { describe, expect, it } from "vitest";
import type { DebugShellResult } from "@bindings/model";
import { debugBanner, debugShellDisabledReason, isNoShellError, isPresetImage } from "./debugShell";

describe("isPresetImage", () => {
  it("recognises the offered toolbox images exactly", () => {
    expect(isPresetImage("busybox:latest")).toBe(true);
    expect(isPresetImage("nicolaka/netshoot:latest")).toBe(true);
  });
  it("treats anything else (untagged, other tag, other case, empty) as custom", () => {
    for (const i of ["busybox", "busybox:1.36", "Alpine:latest", "", "__custom__"]) expect(isPresetImage(i), i).toBe(false);
  });
});

describe("debugShellDisabledReason", () => {
  it("is enabled for a running container on a non-Windows engine", () => {
    expect(debugShellDisabledReason("running", "linux")).toBeUndefined();
    expect(debugShellDisabledReason("running")).toBeUndefined();
  });
  it("explains when the container is not running", () => {
    for (const s of ["exited", "paused", "created", "restarting", ""]) expect(debugShellDisabledReason(s, "linux"), s).toMatch(/not running/);
  });
  it("Windows engines are rejected first, even for a stopped container", () => {
    expect(debugShellDisabledReason("running", "windows")).toMatch(/Windows/);
    expect(debugShellDisabledReason("exited", "windows")).toMatch(/Windows/);
  });
});

describe("isNoShellError", () => {
  it("matches the Engine message case-insensitively anywhere in the text", () => {
    expect(isNoShellError("container has no usable shell")).toBe(true);
    expect(isNoShellError("exec: No Usable Shell found (tried sh, bash)")).toBe(true);
  });
  it("does not match other errors", () => {
    expect(isNoShellError("container not found")).toBe(false);
    expect(isNoShellError("")).toBe(false);
  });
});

describe("debugBanner", () => {
  const r = (o: Partial<DebugShellResult> = {}): DebugShellResult => ({ sessionId: "s", shell: "/bin/sh", image: "busybox:latest", sidecarName: "dbg-1", imageMissing: false, sharesIpc: true, notices: null, ...o });
  it("names the image and sidecar and prints each line dimmed with a CRLF terminator", () => {
    const out = debugBanner(r());
    const lines = out.split("\r\n");
    expect(lines.pop()).toBe("");
    expect(lines).toHaveLength(3);
    for (const l of lines) expect(l.startsWith("\x1b[2m# ") && l.endsWith("\x1b[0m")).toBe(true);
    expect(out).toContain("busybox:latest (dbg-1)");
  });
  it("appends notices after the fixed lines, and tolerates null or empty notices", () => {
    const out = debugBanner(r({ notices: ["IPC is not shared", "second"] }));
    const lines = out.split("\r\n").filter(Boolean);
    expect(lines).toHaveLength(5);
    expect(lines[3]).toContain("IPC is not shared");
    expect(lines[4]).toContain("second");
    expect(debugBanner(r({ notices: [] }))).toBe(debugBanner(r({ notices: null })));
  });
});
