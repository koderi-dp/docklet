import { describe, expect, it } from "vitest";
import { Status, type Entry, type Holder, type PortCheck } from "@bindings/ports";
import { checkText, conflictingPortsInError, filterEntries, holderText, requestsFromRows } from "./portCheck";

const holder = (h: Partial<Holder>): Holder => ({
  kind: "process" as Holder["kind"], location: "This machine", bindIp: "", containerId: "", containerName: "", composeProject: "",
  running: false, pid: 0, processName: "", executablePath: "", user: "", ...h,
});

describe("requestsFromRows", () => {
  it("checks only rows with a valid fixed host port and keeps their index", () => {
    const rows = [
      { hostIp: "", hostPort: "8080", containerPort: "80", protocol: "tcp" },
      { hostIp: "", hostPort: "", containerPort: "81", protocol: "tcp" }, // random host port: nothing to check
      { hostIp: " 127.0.0.1 ", hostPort: "5353", containerPort: "53", protocol: "udp" },
      { hostIp: "", hostPort: "99999", containerPort: "1", protocol: "tcp" },
    ];
    expect(requestsFromRows(rows)).toEqual([
      { index: 0, request: { hostIp: "", hostPort: 8080, protocol: "tcp" } },
      { index: 2, request: { hostIp: "127.0.0.1", hostPort: 5353, protocol: "udp" } },
    ]);
  });
});

describe("checkText", () => {
  const base: PortCheck = { hostIp: "", hostPort: 8080, protocol: "tcp", status: Status.StatusInUse, holder: null, also: [], note: "", detail: "" };
  it("names a container with its Compose project", () => {
    const h = holder({ kind: "container" as Holder["kind"], containerName: "shop-web-1", composeProject: "shop" });
    expect(checkText({ ...base, holder: h })).toBe("8080 is used by container shop-web-1 (project shop)");
  });
  it("names a program with its PID and the side of WSL it is on", () => {
    expect(checkText({ ...base, holder: holder({ processName: "nginx.exe", pid: 4321, location: "Windows" }) })).toBe("8080 is used by nginx.exe (PID 4321) on Windows");
  });
  it("never turns an unknown answer into 'free'", () => {
    const unknown = { ...base, status: Status.StatusUnknown, detail: "Could not check: the Engine runs on another computer" };
    expect(checkText(unknown)).toBe("Could not check: the Engine runs on another computer");
    expect(checkText({ ...unknown, detail: "" })).not.toMatch(/free/);
  });
  it("describes a program that could not be identified honestly", () => {
    expect(holderText(holder({ pid: 0 }))).toBe("a program that could not be identified");
  });
});

describe("conflictingPortsInError", () => {
  it("reads the port out of Docker's port conflict messages", () => {
    expect(conflictingPortsInError("Bind for 0.0.0.0:3001 failed: port is already allocated")).toEqual([3001]);
    expect(conflictingPortsInError("ports are not available: exposing port TCP 127.0.0.1:8080 -> 127.0.0.1:0: listen tcp 127.0.0.1:8080: bind: Only one usage of each socket address")).toEqual([8080]);
    expect(conflictingPortsInError("listen tcp4 0.0.0.0:5432: bind: address already in use")).toEqual([5432]);
    expect(conflictingPortsInError("Bind for [::]:9000 failed: port is already allocated")).toEqual([9000]);
  });
  it("ignores other errors", () => {
    expect(conflictingPortsInError("No such container: 10.0.0.1:2375")).toEqual([]);
  });
});

describe("filterEntries", () => {
  const e = (hostPort: number, h: Partial<Holder>): Entry => ({ hostIp: "", hostPort, protocol: "tcp", holder: holder(h) });
  const entries = [e(3001, { kind: "container" as Holder["kind"], containerName: "catalo-web-1" }), e(8080, { processName: "node.exe", pid: 77, executablePath: "C:/tools/node.exe" })];
  it("matches by port, program, container, PID and path, all words together", () => {
    expect(filterEntries(entries, "3001")).toHaveLength(1);
    expect(filterEntries(entries, "NODE")).toHaveLength(1);
    expect(filterEntries(entries, "catalo")).toHaveLength(1);
    expect(filterEntries(entries, "pid77")).toHaveLength(0);
    expect(filterEntries(entries, "77 tools")).toHaveLength(1);
    expect(filterEntries(entries, "")).toHaveLength(2);
  });
});
