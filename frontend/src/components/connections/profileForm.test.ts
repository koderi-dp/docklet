import { describe, expect, it } from "vitest";
import { ConnectionType } from "@bindings/model";
import { buildProfile, defaultForm, formFromProfile } from "./profileForm";

describe("buildProfile", () => {
  it("defaults TCP to TLS on and requires explicit opt-in for plain remote TCP", () => {
    const f = { ...defaultForm(ConnectionType.ConnTCP), host: "10.0.0.5" };
    expect(f.tls).toBe(true);
    expect(buildProfile(f).profile?.tlsVerify).toBe(true);
    const plain = { ...f, tls: false, port: "2375" };
    expect(buildProfile(plain).error).toMatch(/insecure/i);
    const ok = buildProfile({ ...plain, allowInsecure: true }).profile;
    expect(ok?.allowInsecure).toBe(true);
    expect(ok?.endpoint).toBe("tcp://10.0.0.5:2375");
  });

  it("allows plain loopback TCP without the opt-in flag", () => {
    const p = buildProfile({ ...defaultForm(ConnectionType.ConnTCP), host: "127.0.0.1", port: "2375", tls: false }).profile;
    expect(p?.allowInsecure).toBeFalsy();
  });

  it("round-trips ssh user, host and port through the endpoint", () => {
    const built = buildProfile({ ...defaultForm(ConnectionType.ConnSSH), sshHost: "box.example.com", sshUser: "me", sshPort: "2222" }).profile!;
    expect(built.endpoint).toBe("ssh://me@box.example.com:2222");
    const back = formFromProfile(built);
    expect([back.sshHost, back.sshUser, back.sshPort]).toEqual(["box.example.com", "me", "2222"]);
  });
});

describe("ssh options", () => {
  it("carries identity file and ProxyJump, never a secret", () => {
    const f = { ...defaultForm(ConnectionType.ConnSSH), sshHost: "h", sshIdentityFile: "C:\\k\\id", sshProxyJump: "j@jump", sshUseAgent: true };
    const p = buildProfile(f).profile!;
    expect(p.sshIdentityFile).toBe("C:\\k\\id");
    expect(p.sshProxyJump).toBe("j@jump");
    expect(JSON.stringify(p)).not.toMatch(/password|passphrase/i);
    expect(formFromProfile(p).sshProxyJump).toBe("j@jump");
  });
});
