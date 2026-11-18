import { describe, expect, it } from "vitest";
import type { PortBinding } from "@bindings/model";
import { endpointHost, portUrl } from "./portUrl";

const b = (o: Partial<PortBinding> = {}): PortBinding => ({ hostIp: "0.0.0.0", hostPort: 8080, containerPort: 80, protocol: "tcp", ...o });
const prof = (type: string, endpoint = "") => ({ type, endpoint }) as never;

describe("portUrl", () => {
  it("local engines (npipe/unix/wsl) open on localhost for wildcard and loopback bindings", () => {
    for (const t of ["npipe", "unix", "wsl"]) {
      for (const ip of ["0.0.0.0", "127.0.0.1", "::", "::1", ""]) {
        expect(portUrl(prof(t), b({ hostIp: ip }))).toEqual({ url: "http://localhost:8080" });
      }
    }
  });
  it("a specific bind address is never guessed", () => {
    expect(portUrl(prof("wsl"), b({ hostIp: "172.20.0.5" })).url).toBeUndefined();
    expect(portUrl(prof("tcp", "tcp://10.0.0.2:2376"), b({ hostIp: "192.168.1.9" })).url).toBeUndefined();
  });
  it("tcp and ssh use the engine host from the endpoint", () => {
    expect(portUrl(prof("tcp", "tcp://build.example.com:2376"), b())).toEqual({ url: "http://build.example.com:8080" });
    expect(portUrl(prof("ssh", "ssh://me@10.0.0.7:22"), b())).toEqual({ url: "http://10.0.0.7:8080" });
  });
  it("brackets IPv6 engine hosts", () => {
    expect(portUrl(prof("tcp", "tcp://[fd00::1]:2376"), b({ hostIp: "::" })).url).toBe("http://[fd00::1]:8080");
  });
  it("loopback bindings on a remote engine are not reachable", () => {
    const r = portUrl(prof("tcp", "tcp://10.0.0.2:2376"), b({ hostIp: "127.0.0.1" }));
    expect(r.url).toBeUndefined();
    expect(r.reason).toMatch(/not reachable/);
  });
  it("rejects unpublished, non-TCP and unknown-profile ports", () => {
    expect(portUrl(prof("npipe"), b({ hostPort: 0 })).reason).toMatch(/Not published/);
    expect(portUrl(prof("npipe"), b({ protocol: "udp" })).reason).toMatch(/TCP/);
    expect(portUrl(undefined, b()).url).toBeUndefined();
  });
  it("imported ssh contexts follow their endpoint scheme", () => {
    expect(portUrl(prof("context", "ssh://u@host:22"), b()).url).toBe("http://host:8080");
    expect(portUrl(prof("context", "npipe:////./pipe/x"), b()).url).toBe("http://localhost:8080");
  });
  it("parses hosts out of endpoints", () => {
    expect(endpointHost("ssh://user@host.example:2222/path")).toBe("host.example");
    expect(endpointHost("tcp://[::1]:2375")).toBe("[::1]");
    expect(endpointHost("")).toBeNull();
  });
});
