import { ConnectionType } from "@bindings/model";
import type { ConnectionProfile } from "@bindings/model";

export interface FormState {
  type: ConnectionType;
  name: string;
  pipe: string;
  socket: string;
  host: string;
  port: string;
  tls: boolean;
  certDir: string;
  allowInsecure: boolean;
  sshHost: string;
  sshUser: string;
  sshPort: string;
  sshIdentityFile: string;
  sshProxyJump: string;
  sshUseAgent: boolean;
  distro: string;
  contextName: string;
}

export const DEFAULT_PIPE = "npipe:////./pipe/docker_engine";
export const DEFAULT_SOCKET = "unix:///var/run/docker.sock";

export function defaultForm(type: ConnectionType): FormState {
  return {
    type,
    name: "",
    pipe: DEFAULT_PIPE,
    socket: DEFAULT_SOCKET,
    host: "",
    port: "2376",
    tls: true, // TLS is on by default; plain TCP needs an explicit opt-in
    certDir: "",
    allowInsecure: false,
    sshHost: "",
    sshUser: "",
    sshPort: "22",
    sshIdentityFile: "",
    sshProxyJump: "",
    sshUseAgent: false,
    distro: "",
    contextName: "",
  };
}

export function isLoopbackHost(h: string): boolean {
  const x = h.trim().toLowerCase().replace(/^\[|\]$/g, "");
  return x === "localhost" || x === "::1" || /^127\.\d+\.\d+\.\d+$/.test(x);
}

/** Reads a saved profile back into form fields (used when editing). */
export function formFromProfile(p: ConnectionProfile): FormState {
  const f = { ...defaultForm(p.type), name: p.name };
  const parse = (): URL | null => {
    try {
      return new URL(p.endpoint);
    } catch {
      return null;
    }
  };
  switch (p.type) {
    case ConnectionType.ConnNamedPipe:
      f.pipe = p.endpoint;
      break;
    case ConnectionType.ConnUnix:
      f.socket = p.endpoint;
      break;
    case ConnectionType.ConnTCP: {
      const u = parse();
      f.host = u?.hostname.replace(/^\[|\]$/g, "") ?? "";
      f.port = u?.port ?? "";
      f.certDir = p.tlsCertDir ?? "";
      f.tls = !!p.tlsVerify || !!p.tlsCertDir;
      f.allowInsecure = !!p.allowInsecure;
      break;
    }
    case ConnectionType.ConnSSH: {
      const u = parse();
      f.sshHost = u?.hostname ?? "";
      f.sshUser = decodeURIComponent(u?.username ?? "");
      f.sshPort = u?.port || "22";
      f.sshIdentityFile = p.sshIdentityFile ?? "";
      f.sshProxyJump = p.sshProxyJump ?? "";
      f.sshUseAgent = !!p.sshUseAgent;
      break;
    }
    case ConnectionType.ConnWSL:
      f.distro = p.wslDistro ?? "";
      break;
  }
  return f;
}

export function defaultName(f: FormState): string {
  switch (f.type) {
    case ConnectionType.ConnWSL:
      return f.distro ? `WSL (${f.distro})` : "";
    case ConnectionType.ConnNamedPipe:
      return "Named pipe";
    case ConnectionType.ConnUnix:
      return "Unix socket";
    case ConnectionType.ConnTCP:
      return f.host ? `${f.host}:${f.port}` : "";
    case ConnectionType.ConnSSH:
      return f.sshHost ? `${f.sshUser ? f.sshUser + "@" : ""}${f.sshHost}` : "";
    default:
      return f.contextName ? `${f.contextName} (context)` : "";
  }
}

export interface BuildResult {
  profile?: ConnectionProfile;
  /** Human-readable reason the form cannot be submitted yet. */
  error?: string;
}

/**
 * Builds the profile to test/save from the form. Docker Context is resolved by
 * the backend (ContextProfile) and is not built here.
 */
export function buildProfile(f: FormState, id = ""): BuildResult {
  const name = f.name.trim() || defaultName(f);
  const mk = (extra: Partial<ConnectionProfile>) => ({ profile: { id, name, type: f.type, endpoint: "", ...extra } as ConnectionProfile });
  if (!name) return { error: "Enter a name" };
  switch (f.type) {
    case ConnectionType.ConnNamedPipe:
      return f.pipe.trim() ? mk({ endpoint: f.pipe.trim() }) : { error: "Enter the pipe path" };
    case ConnectionType.ConnUnix:
      return f.socket.trim() ? mk({ endpoint: f.socket.trim() }) : { error: "Enter the socket path" };
    case ConnectionType.ConnTCP: {
      const host = f.host.trim();
      const port = f.port.trim();
      if (!host) return { error: "Enter the host" };
      if (!/^\d{1,5}$/.test(port) || +port < 1 || +port > 65535) return { error: "Enter a valid port (1-65535)" };
      const hostPart = host.includes(":") && !host.startsWith("[") ? `[${host}]` : host;
      const endpoint = `tcp://${hostPart}:${port}`;
      if (f.tls) return mk({ endpoint, tlsVerify: true, tlsCertDir: f.certDir.trim() || undefined });
      if (!isLoopbackHost(host) && !f.allowInsecure)
        return { error: "Confirm the insecure plain TCP warning, or turn TLS on" };
      return mk({ endpoint, allowInsecure: !isLoopbackHost(host) && f.allowInsecure });
    }
    case ConnectionType.ConnSSH: {
      const host = f.sshHost.trim();
      const user = f.sshUser.trim();
      const port = f.sshPort.trim();
      if (!host) return { error: "Enter the SSH host" };
      if (port && (!/^\d{1,5}$/.test(port) || +port < 1 || +port > 65535)) return { error: "Enter a valid SSH port (1-65535)" };
      const endpoint = `ssh://${user ? encodeURIComponent(user) + "@" : ""}${host}${port && port !== "22" ? ":" + port : ""}`;
      return mk({
        endpoint,
        sshIdentityFile: f.sshIdentityFile.trim() || undefined,
        sshProxyJump: f.sshProxyJump.trim() || undefined,
        sshUseAgent: f.sshIdentityFile.trim() ? f.sshUseAgent : undefined,
      });
    }
    case ConnectionType.ConnWSL:
      return f.distro ? mk({ endpoint: "", wslDistro: f.distro }) : { error: "Select a WSL distribution" };
    default:
      return { error: "Select a Docker context" };
  }
}
