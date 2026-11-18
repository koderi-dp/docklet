import { Cable, Container, Network, Server, Terminal, TerminalSquare, type LucideIcon } from "lucide-react";
import { ConnectionState, ConnectionType, type ConnectionProfile, type ConnectionStatus } from "@bindings/model";

export const TYPE_META: Record<string, { label: string; short: string; icon: LucideIcon; hint: string }> = {
  [ConnectionType.ConnWSL]: { label: "WSL2", short: "WSL2", icon: TerminalSquare, hint: "Docker Engine inside a WSL2 distribution" },
  [ConnectionType.ConnNamedPipe]: { label: "Windows named pipe", short: "Named pipe", icon: Cable, hint: "Local Windows Docker pipe" },
  [ConnectionType.ConnUnix]: { label: "Unix socket", short: "Unix socket", icon: Server, hint: "Local Linux/macOS socket" },
  [ConnectionType.ConnTCP]: { label: "TCP/TLS", short: "TCP/TLS", icon: Network, hint: "Remote Engine over TCP with TLS" },
  [ConnectionType.ConnSSH]: { label: "SSH", short: "SSH", icon: Terminal, hint: "Remote Engine through OpenSSH" },
  [ConnectionType.ConnContext]: { label: "Docker Context", short: "Context", icon: Container, hint: "Import a Docker CLI context" },
};

export function typeLabel(t: ConnectionType | string | undefined): string {
  return (t && TYPE_META[t]?.label) || "Unknown";
}

export function TypeIcon({ type, className }: { type: ConnectionType | string | undefined; className?: string }) {
  const Icon = (type && TYPE_META[type]?.icon) || Server;
  return <Icon className={className} aria-hidden />;
}

/** Endpoint text without credentials (a password in the userinfo is dropped, the user name kept). */
export function endpointSummary(p: ConnectionProfile): string {
  if (p.type === ConnectionType.ConnWSL) return p.wslDistro ? `WSL distribution: ${p.wslDistro}` : "WSL";
  return (p.endpoint ?? "").replace(/:\/\/([^/:@]*):[^@/]*@/, "://$1@");
}

export function stateText(s?: ConnectionState): string {
  switch (s) {
    case ConnectionState.StateConnected:
      return "Connected";
    case ConnectionState.StateConnecting:
      return "Connecting";
    case ConnectionState.StateError:
      return "Failed";
    default:
      return "Disconnected";
  }
}

export function statusTone(s?: ConnectionState): "success" | "warning" | "destructive" | "muted" {
  switch (s) {
    case ConnectionState.StateConnected:
      return "success";
    case ConnectionState.StateConnecting:
      return "warning";
    case ConnectionState.StateError:
      return "destructive";
    default:
      return "muted";
  }
}

/** "Docker 27.1 - linux/x86_64": only facts the Engine reported. */
export function engineSummary(st?: ConnectionStatus): string | null {
  if (!st || st.state !== ConnectionState.StateConnected) return null;
  const parts = [st.serverVersion && `Docker ${st.serverVersion}`, st.os && `${st.os}${st.arch ? `/${st.arch}` : ""}`].filter(Boolean);
  return parts.length ? parts.join(" - ") : null;
}
