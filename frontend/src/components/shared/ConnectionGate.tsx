import type { ReactNode } from "react";
import { useNavigate } from "react-router-dom";
import { Activity, Plug } from "lucide-react";
import { ConnectionState, type ConnectionStatus } from "@bindings/model";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { useActiveConnection, useSelectConnection } from "@/hooks/useActiveConnection";
import { EmptyState } from "./EmptyState";

/**
 * Renders children only when the active Engine is connected, passing its id and
 * status. Handles the no-connection / connecting / disconnected states for every
 * resource page. Children are keyed by connectionId so page state (selection,
 * inspector, filters) resets on engine switch.
 */
export function ConnectionGate({
  children,
}: {
  children: (ctx: { connectionId: string; status: ConnectionStatus }) => ReactNode;
}) {
  const { connectionId, status, profile, isConnected } = useActiveConnection();
  const navigate = useNavigate();
  const select = useSelectConnection();

  if (!connectionId)
    return (
      <EmptyState icon={<Plug />} title="Connect to a Docker Engine">
        <p>Select an engine from the sidebar to see its resources.</p>
        <Button className="mt-3" onClick={() => navigate("/connections")}>
          Manage connections
        </Button>
      </EmptyState>
    );
  if (!status || status.state === ConnectionState.StateConnecting)
    return (
      <div className="space-y-4 p-5">
        <Skeleton className="h-10 w-64" />
        <Skeleton className="h-28 w-full" />
        <Skeleton className="h-64 w-full" />
      </div>
    );
  if (!isConnected)
    return (
      <EmptyState icon={<Activity />} title={`${profile?.name ?? "Engine"} is not connected`}>
        <p>{status.error ?? "Connect to load its resources."}</p>
        {profile && <p className="mt-1 font-mono text-xs">{profile.endpoint}</p>}
        <div className="mt-3 flex justify-center gap-2">
          <Button onClick={() => select(connectionId)}>Reconnect</Button>
          <Button variant="outline" onClick={() => navigate("/connections")}>
            Connections
          </Button>
        </div>
        {status.diagnostics && (
          <pre className="mt-3 max-h-40 overflow-auto rounded-md bg-muted/40 p-2 text-left font-mono text-[11px]">
            {status.diagnostics}
          </pre>
        )}
      </EmptyState>
    );
  return <div key={connectionId} className="h-full min-h-0">{children({ connectionId, status })}</div>;
}
