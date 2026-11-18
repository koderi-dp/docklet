// Decides whether a published container port can be opened in the browser, and where.
// We only build a URL when the host is certain; otherwise we say why (never guess).
import type { ConnectionProfile, PortBinding } from "@bindings/model";

export type PortLink = { url: string; reason?: undefined } | { url?: undefined; reason: string };

type ProfileLike = Pick<ConnectionProfile, "type" | "endpoint">;

const WILDCARD_HOSTS = new Set(["", "0.0.0.0", "::", "[::]"]);
const LOOPBACK_HOSTS = new Set(["127.0.0.1", "::1", "[::1]", "localhost"]);

/** Host of an engine endpoint such as tcp://host:2376 or ssh://user@[fd00::1]:22; null when absent. */
export function endpointHost(endpoint: string): string | null {
  let rest = endpoint.trim().replace(/^[a-z][a-z0-9+.-]*:\/\//i, "");
  rest = rest.split(/[/?#]/)[0];
  const at = rest.lastIndexOf("@");
  if (at >= 0) rest = rest.slice(at + 1);
  if (rest.startsWith("[")) {
    const end = rest.indexOf("]");
    return end > 0 ? rest.slice(0, end + 1) : null;
  }
  const colon = rest.lastIndexOf(":");
  const host = colon >= 0 ? rest.slice(0, colon) : rest;
  return host || null;
}

function urlHost(host: string): string {
  return host.includes(":") && !host.startsWith("[") ? `[${host}]` : host;
}

function endpointScheme(p: ProfileLike): string {
  const m = /^([a-z][a-z0-9+.-]*):\/\//i.exec(p.endpoint.trim());
  return m ? m[1].toLowerCase() : "";
}

/**
 * - npipe/unix/wsl: the engine runs on this machine (WSL2 forwards published ports to Windows
 *   localhost), so a wildcard or loopback binding opens on `localhost`.
 * - tcp/ssh (and contexts using them): the engine host from the endpoint, only for wildcard
 *   bindings (a loopback binding on a remote engine is not reachable from here).
 */
export function portUrl(profile: ProfileLike | undefined, binding: PortBinding): PortLink {
  if (!binding.hostPort) return { reason: "Not published to the host" };
  if ((binding.protocol || "tcp").toLowerCase() !== "tcp") return { reason: "Only TCP ports can be opened in a browser" };
  if (!profile) return { reason: "The connection is not known, so the host cannot be determined" };

  const bind = (binding.hostIp ?? "").trim().toLowerCase();
  const wildcard = WILDCARD_HOSTS.has(bind);
  const loopback = LOOPBACK_HOSTS.has(bind);
  const port = binding.hostPort;

  const type = profile.type === "context" ? endpointScheme(profile) : profile.type;
  if (type === "npipe" || type === "unix" || type === "wsl") {
    if (!wildcard && !loopback) return { reason: `Bound to ${binding.hostIp}, which may not be reachable as localhost` };
    return { url: `http://localhost:${port}` };
  }
  if (type === "tcp" || type === "ssh") {
    const host = endpointHost(profile.endpoint);
    if (!host) return { reason: "The engine host could not be read from the connection endpoint" };
    if (!wildcard) {
      return {
        reason: loopback
          ? `Bound to ${binding.hostIp} on the engine host, so it is not reachable from this machine`
          : `Bound to ${binding.hostIp}; the browser host is not guessed`,
      };
    }
    return { url: `http://${urlHost(host)}:${port}` };
  }
  return { reason: "The host of this connection type cannot be determined" };
}
