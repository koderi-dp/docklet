import { useActiveConnection } from "@/hooks/useActiveConnection";
import { ConnectionDot } from "@/components/connections/ConnectionStatus";
import { ConnectionState } from "@bindings/model";

export default function StatusBar() {
  const { profile, status } = useActiveConnection();
  const connected = status?.state === ConnectionState.StateConnected;
  return (
    <footer className="flex h-8 shrink-0 items-center gap-4 border-t border-border px-4 text-xs text-muted-foreground">
      <span className="flex items-center gap-2">
        <ConnectionDot state={status?.state} />
        {profile
          ? connected
            ? `Connected: ${profile.name}`
            : `${profile.name}: ${status?.state ?? "not connected"}`
          : "No engine selected"}
      </span>
      {connected && status?.serverVersion && <span>Docker {status.serverVersion}</span>}
      {connected && status?.apiVersion && <span>API {status.apiVersion}</span>}
    </footer>
  );
}
