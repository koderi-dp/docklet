import { describe, expect, it } from "vitest";
import { validateForm } from "./createContainerForm";

type Form = Parameters<typeof validateForm>[0];
const valid: Form = { image: "nginx:1.27", name: "", env: [], ports: [], mounts: [], memory: "", cpus: "", maxRetries: "" };
const check = (over: Partial<Form>) => validateForm({ ...valid, ...over });

describe("validateForm", () => {
  it("accepts an image alone and ignores empty rows", () => {
    expect(check({})).toBeNull();
    expect(check({ env: [{ key: "", value: "" }], ports: [{ hostIp: "", hostPort: "", containerPort: "", protocol: "tcp" }], mounts: [{ type: "bind", source: "", target: "", readOnly: false }] })).toBeNull();
  });

  const rejected: Array<[string, Partial<Form>]> = [
    ["missing image", { image: "  " }],
    ["image with spaces", { image: "nginx latest" }],
    ["name starting with a dash", { name: "-web" }],
    ["env name with a space", { env: [{ key: "A B", value: "1" }] }],
    ["duplicate env name", { env: [{ key: "A", value: "1" }, { key: "A", value: "2" }] }],
    ["container port out of range", { ports: [{ hostIp: "", hostPort: "", containerPort: "70000", protocol: "tcp" }] }],
    ["host port out of range", { ports: [{ hostIp: "", hostPort: "99999", containerPort: "80", protocol: "tcp" }] }],
    ["relative mount target", { mounts: [{ type: "volume", source: "data", target: "data", readOnly: false }] }],
    ["bind mount without host path", { mounts: [{ type: "bind", source: "", target: "/d", readOnly: false }] }],
    ["duplicate mount target", { mounts: [{ type: "volume", source: "a", target: "/d", readOnly: false }, { type: "volume", source: "b", target: "/d", readOnly: false }] }],
    ["memory below the engine minimum", { memory: "2" }],
    ["non-positive cpu limit", { cpus: "0" }],
    ["non-numeric max retries", { maxRetries: "x" }],
  ];
  it.each(rejected)("rejects %s", (_name, over) => {
    expect(check(over)).not.toBeNull();
  });

  it("accepts a published port with an empty host port and a named volume", () => {
    expect(check({ ports: [{ hostIp: "127.0.0.1", hostPort: "", containerPort: "80", protocol: "tcp" }], mounts: [{ type: "volume", source: "data", target: "/data", readOnly: true }] })).toBeNull();
  });
});
