import { useState } from "react";
import { CheckCircle2, Copy, FlaskConical, Loader2, MoreHorizontal, Pencil, Plug, PlugZap, Plus, Trash2, Unplug } from "lucide-react";
import { ConnectionService } from "@bindings/services";
import { ConnectionState, ConnectionType, type ConnectionProfile, type ConnectionStatus } from "@bindings/model";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Skeleton } from "@/components/ui/skeleton";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { EmptyState } from "@/components/shared/EmptyState";
import { PageHeader } from "@/components/shared/PageHeader";
import { ConnectionStatusBadge } from "@/components/connections/ConnectionStatusBadge";
import { EngineBadge } from "@/components/connections/EngineBadge";
import { engineVersionLabel } from "@/lib/engineKind";
import { ErrorWithDiagnostics } from "@/components/connections/DiagnosticsBlock";
import { TypeIcon, endpointSummary, engineSummary, typeLabel } from "@/components/connections/connectionMeta";
import { WslDockerSetup } from "@/components/connections/wslinstall/WslDockerSetup";
import { useConnectionActions } from "@/components/connections/useConnectionActions";
import { useConnections, useConnectionStatus } from "@/hooks/useActiveConnection";
import { useUI } from "@/stores/uiStore";
import { errorMessage } from "@/lib/errors";

type TestResult = { kind: "ok"; status: ConnectionStatus } | { kind: "error"; message: string; diagnostics?: string };

function ConnectionCard({ c }: { c: ConnectionProfile }) {
  const active = useUI((s) => s.activeConnectionId) === c.id;
  const openDialog = useUI((s) => s.openConnectionDialog);
  const setRemove = useUI((s) => s.setRemoveConnectionId);
  const { data: status } = useConnectionStatus(c.id);
  const { connect, disconnect, duplicate } = useConnectionActions();
  const [testing, setTesting] = useState(false);
  const [test, setTest] = useState<TestResult | null>(null);
  const state = status?.state;
  const connected = state === ConnectionState.StateConnected;
  const connecting = state === ConnectionState.StateConnecting;
  const summary = engineSummary(status);
  const dockerMissing =
    c.type === ConnectionType.ConnWSL && !!c.wslDistro && state === ConnectionState.StateError && /was not found in the WSL distribution/.test(status?.error ?? "");

  const runTest = async () => {
    setTesting(true);
    setTest(null);
    try {
      const st = await ConnectionService.Test(c);
      setTest(
        st.state === ConnectionState.StateConnected
          ? { kind: "ok", status: st }
          : { kind: "error", message: st.error || "The connection test failed.", diagnostics: st.diagnostics },
      );
    } catch (e) {
      setTest({ kind: "error", message: errorMessage(e) });
    } finally {
      setTesting(false);
    }
  };

  return (
    <li
      className={`rounded-lg border bg-card p-3 ${active ? "border-primary/60 ring-1 ring-primary/30" : ""}`}
      aria-current={active ? "true" : undefined}
    >
      <div className="flex items-center gap-3">
        <div className="flex size-9 shrink-0 items-center justify-center rounded-md bg-muted text-muted-foreground">
          <TypeIcon type={c.type} className="size-[18px]" />
        </div>
        <div className="min-w-0 flex-1">
          <div className="flex flex-wrap items-center gap-2">
            <span className="truncate font-medium">{c.name}</span>
            {active && (
              <Badge variant="outline" className="rounded-md border-primary/40 px-1.5 py-0 text-[11px] text-primary">
                Active
              </Badge>
            )}
            {c.discovered && (
              <Badge variant="outline" className="rounded-md px-1.5 py-0 text-[11px] text-muted-foreground">
                Auto-detected
              </Badge>
            )}
            <EngineBadge kind={status?.kind} />
            <ConnectionStatusBadge state={state} />
          </div>
          <div className="mt-0.5 flex flex-wrap items-center gap-x-2 text-xs text-muted-foreground">
            <span>{typeLabel(c.type)}</span>
            <span aria-hidden>·</span>
            <span className="truncate font-mono">{endpointSummary(c)}</span>
            {summary && (
              <>
                <span aria-hidden>·</span>
                <span>{summary}</span>
              </>
            )}
          </div>
          {state === ConnectionState.StateError && status?.error && (
            <p className="mt-1 text-xs text-destructive">{status.error}</p>
          )}
        </div>
        <div className="flex shrink-0 items-center gap-1.5">
          {connected ? (
            <Button variant="outline" size="sm" onClick={() => disconnect(c.id, c.name)}>
              <Unplug />
              Disconnect
            </Button>
          ) : (
            <Button size="sm" onClick={() => connect(c.id)} disabled={connecting}>
              {connecting ? <Loader2 className="animate-spin" /> : <Plug />}
              {state === ConnectionState.StateError ? "Reconnect" : "Connect"}
            </Button>
          )}
          <EditButton c={c} onEdit={() => openDialog({ mode: "edit", id: c.id })} />
          <Button variant="outline" size="sm" onClick={runTest} disabled={testing} title="Connects with a temporary client; nothing is saved">
            {testing ? <Loader2 className="animate-spin" /> : <FlaskConical />}
            Test
          </Button>
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button variant="ghost" size="icon" className="size-8" aria-label={`More actions for ${c.name}`} title={`More actions for ${c.name}`}>
                <MoreHorizontal className="size-4" />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end">
              <DropdownMenuItem onSelect={() => void duplicate(c.id)}>
                <Copy />
                Duplicate
              </DropdownMenuItem>
              <DropdownMenuSeparator />
              <DropdownMenuItem
                variant="destructive"
                disabled={c.discovered}
                title={c.discovered ? "Auto-detected connections cannot be removed. Duplicate it to keep a saved copy." : undefined}
                onSelect={() => setRemove(c.id)}
              >
                <Trash2 />
                Remove…
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </div>
      </div>
      {test?.kind === "ok" && (
        <p className="mt-2 flex items-center gap-2 rounded-md border border-success/30 bg-success/10 px-3 py-2 text-xs text-success" role="status">
          <CheckCircle2 className="size-4 shrink-0" />
          Test passed
          {test.status.serverVersion ? `: ${engineVersionLabel(test.status.kind, test.status.serverVersion)}` : ""}
          {test.status.apiVersion ? `, API ${test.status.apiVersion}` : ""}
          {test.status.os ? `, ${test.status.os}` : ""}.
        </p>
      )}
      {test?.kind === "error" && <ErrorWithDiagnostics className="mt-2" message={test.message} diagnostics={test.diagnostics} />}
      {state === ConnectionState.StateError && !test && status?.diagnostics && (
        <ErrorWithDiagnostics className="mt-2" message={status.error} diagnostics={status.diagnostics} />
      )}
      {dockerMissing && (
        <WslDockerSetup className="mt-2" distro={c.wslDistro as string} successAction={{ label: "Connect", run: () => void connect(c.id) }} />
      )}
    </li>
  );
}

function EditButton({ c, onEdit }: { c: ConnectionProfile; onEdit: () => void }) {
  if (c.discovered)
    return (
      <Tooltip>
        <TooltipTrigger asChild>
          <span>
            <Button variant="outline" size="sm" disabled>
              <Pencil />
              Edit
            </Button>
          </span>
        </TooltipTrigger>
        <TooltipContent>Auto-detected connections cannot be edited. Duplicate it to get an editable copy.</TooltipContent>
      </Tooltip>
    );
  return (
    <Button variant="outline" size="sm" onClick={onEdit}>
      <Pencil />
      Edit
    </Button>
  );
}

export default function ConnectionsPage() {
  const { data, error, isLoading } = useConnections();
  const openDialog = useUI((s) => s.openConnectionDialog);
  return (
    <div className="max-w-5xl p-5">
      <PageHeader
        title="Connections"
        subtitle="Docker and Podman engines Docklet can manage. Each connection is isolated; resources are never mixed."
        actions={
          <Button onClick={() => openDialog({ mode: "add" })}>
            <Plus />
            Add connection
          </Button>
        }
      />
      {isLoading && (
        <div className="space-y-2">
          <Skeleton className="h-16 w-full" />
          <Skeleton className="h-16 w-full" />
        </div>
      )}
      {error && (
        <p className="text-destructive" role="alert">
          Could not load connections: {String(error)}
        </p>
      )}
      {data && data.length === 0 && (
        <EmptyState icon={<PlugZap />} title="No connections yet">
          <p>Add a Docker or Podman engine to get started.</p>
          <Button className="mt-3" onClick={() => openDialog({ mode: "add" })}>
            <Plus />
            Add connection
          </Button>
        </EmptyState>
      )}
      <ul className="space-y-2">{data?.map((c) => <ConnectionCard key={c.id} c={c} />)}</ul>
    </div>
  );
}
