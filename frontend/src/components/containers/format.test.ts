import { describe, expect, it } from "vitest";
import type { PortBinding } from "@bindings/model";
import { formatBytes, formatCreated, formatPortLong, publishedBindings, publishedPorts, shortId, uniquePortBindings, uniquePortsLong, uptimeLabel } from "./format";

const p = (o: Partial<PortBinding> = {}): PortBinding => ({ hostIp: "0.0.0.0", hostPort: 8080, containerPort: 80, protocol: "tcp", ...o });

describe("formatBytes", () => {
  it("shows plain bytes below 1 KB", () => {
    expect(formatBytes(0)).toBe("0 B");
    expect(formatBytes(1023)).toBe("1023 B");
  });
  it("switches unit exactly at each 1024 boundary", () => {
    expect(formatBytes(1024)).toBe("1 KB");
    expect(formatBytes(1024 ** 2)).toBe("1 MB");
    expect(formatBytes(1024 ** 3)).toBe("1 GB");
    expect(formatBytes(1024 ** 4)).toBe("1 TB");
  });
  it("keeps one decimal below 100 and drops it from 100 up", () => {
    expect(formatBytes(1536)).toBe("1.5 KB");
    expect(formatBytes(10 * 1024 + 512)).toBe("10.5 KB");
    expect(formatBytes(100 * 1024)).toBe("100 KB");
    expect(formatBytes(150.4 * 1024 * 1024)).toBe("150 MB");
  });
  it("does not print a trailing .0", () => {
    expect(formatBytes(2 * 1024)).toBe("2 KB");
  });
  it("stays in TB for very large values instead of running out of units", () => {
    expect(formatBytes(2048 * 1024 ** 4)).toBe("2048 TB");
  });
  // 1048575 B is 1023.999 KB, which used to print as "1024 KB".
  it("rounding up to 1024 promotes to the next unit", () => {
    expect(formatBytes(1024 ** 2 - 1)).toBe("1 MB");
    expect(formatBytes(1024 ** 3 - 1)).toBe("1 GB");
    expect(formatBytes(1023 * 1024)).toBe("1023 KB");
  });
});

describe("formatPortLong", () => {
  it("published port shows ip:host -> container/proto", () => {
    expect(formatPortLong(p({ hostIp: "127.0.0.1" }))).toBe("127.0.0.1:8080 → 80/tcp");
  });
  it("exposed-only (no host port) shows only the container side", () => {
    expect(formatPortLong(p({ hostPort: 0, hostIp: "" }))).toBe("80/tcp");
  });
  it("defaults the protocol to tcp and keeps udp", () => {
    expect(formatPortLong(p({ protocol: "" }))).toBe("0.0.0.0:8080 → 80/tcp");
    expect(formatPortLong(p({ protocol: "udp" }))).toBe("0.0.0.0:8080 → 80/udp");
  });
  it("brackets a bare IPv6 address but not an already bracketed one", () => {
    expect(formatPortLong(p({ hostIp: "::" }))).toBe("[::]:8080 → 80/tcp");
    expect(formatPortLong(p({ hostIp: "::1" }))).toBe("[::1]:8080 → 80/tcp");
    expect(formatPortLong(p({ hostIp: "[::1]" }))).toBe("[::1]:8080 → 80/tcp");
  });
  it("omits the address when the host ip is empty", () => {
    expect(formatPortLong(p({ hostIp: "" }))).toBe("8080 → 80/tcp");
  });
});

describe("publishedBindings / publishedPorts", () => {
  it("returns nothing for no ports and skips unpublished ones", () => {
    expect(publishedPorts([])).toEqual([]);
    expect(publishedPorts([p({ hostPort: 0 }), p({ hostPort: 0, containerPort: 443 })])).toEqual([]);
  });
  it("labels host:container and adds the protocol only when it is not tcp", () => {
    expect(publishedPorts([p(), p({ hostPort: 53, containerPort: 53, protocol: "udp" }), p({ hostPort: 9, containerPort: 9, protocol: "" })])).toEqual([
      "8080:80",
      "53:53/udp",
      "9:9",
    ]);
  });
  it("collapses the IPv4 and IPv6 binding of the same mapping and keeps the first binding", () => {
    const v4 = p({ hostIp: "0.0.0.0" });
    const v6 = p({ hostIp: "::" });
    const out = publishedBindings([v4, v6]);
    expect(out).toHaveLength(1);
    expect(out[0].binding).toBe(v4);
  });
  it("keeps tcp and udp of the same numbers apart, and different host ports apart", () => {
    expect(publishedPorts([p(), p({ protocol: "udp" }), p({ hostPort: 8081 })])).toEqual(["8080:80", "8080:80/udp", "8081:80"]);
  });
});

describe("uniquePortsLong / uniquePortBindings", () => {
  it("drops exact duplicates and keeps the first binding", () => {
    const a = p({ hostIp: "127.0.0.1" });
    const out = uniquePortBindings([a, { ...a }]);
    expect(out).toHaveLength(1);
    expect(out[0].binding).toBe(a);
  });
  it("lists exposed-only ports too", () => {
    expect(uniquePortsLong([p({ hostPort: 0, hostIp: "" }), p({ hostPort: 0, hostIp: "" })])).toEqual(["80/tcp"]);
  });
  it("keeps different specific host addresses of the same port apart", () => {
    expect(uniquePortsLong([p({ hostIp: "127.0.0.1" }), p({ hostIp: "192.168.1.5" })])).toHaveLength(2);
  });
  it("collapses the IPv4 and IPv6 wildcard mappings of the same host port, keeping the first", () => {
    const out = uniquePortsLong([p({ hostIp: "0.0.0.0" }), p({ hostIp: "::" })]);
    expect(out).toHaveLength(1);
    expect(out[0]).toContain("0.0.0.0:");
  });
  it("keeps a wildcard and a specific address of the same port apart", () => {
    expect(uniquePortsLong([p({ hostIp: "0.0.0.0" }), p({ hostIp: "127.0.0.1" })])).toHaveLength(2);
  });
  it("keeps wildcard mappings of different host ports apart", () => {
    expect(uniquePortsLong([p({ hostIp: "0.0.0.0" }), p({ hostIp: "::", hostPort: 9090 })])).toHaveLength(2);
  });
});

describe("uptimeLabel", () => {
  const up = (status: string, state = "running") => uptimeLabel({ state, status });
  it("extracts the duration and strips health/paused suffixes", () => {
    expect(up("Up 2 hours")).toBe("2 hours");
    expect(up("Up 2 hours (healthy)")).toBe("2 hours");
    expect(up("Up 5 minutes (unhealthy)")).toBe("5 minutes");
    expect(up("Up 10 seconds (health: starting)")).toBe("10 seconds");
    expect(up("Up About an hour (Paused)", "paused")).toBe("About an hour");
    expect(up("Up Less than a second")).toBe("Less than a second");
  });
  it("tolerates surrounding whitespace and lower case", () => {
    expect(up("  up 3 days  ")).toBe("3 days");
  });
  it("is undefined for containers that are not running or paused", () => {
    for (const s of ["exited", "created", "restarting", "dead", "removing"]) expect(up("Up 2 hours", s)).toBeUndefined();
  });
  it("is undefined when a running container has no parsable status", () => {
    expect(up("")).toBeUndefined();
    expect(up("Restarting (1) 5 seconds ago")).toBeUndefined();
  });
});

describe("formatCreated / shortId", () => {
  it("shows a dash for an unknown (0) creation time", () => {
    expect(formatCreated(0)).toBe("—");
  });
  it("formats a real timestamp as non-empty text containing the year (locale/timezone independent)", () => {
    const out = formatCreated(Date.UTC(2026, 5, 15, 12, 0, 0) / 1000);
    expect(out).not.toBe("—");
    expect(out).toContain("2026");
  });
  it("shortId keeps the first 12 characters and never pads shorter ids", () => {
    expect(shortId("0123456789abcdef0123")).toBe("0123456789ab");
    expect(shortId("abc")).toBe("abc");
    expect(shortId("")).toBe("");
  });
});
