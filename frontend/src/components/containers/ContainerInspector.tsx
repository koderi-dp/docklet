import { useState } from "react";
import { Link } from "react-router-dom";
import { AppWindow, Box, ExternalLink, Play, RotateCw, Square, SquareTerminal, X } from "lucide-react";
import type { Container, ContainerStat } from "@bindings/model";
import { Button } from "@/components/ui/button";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Skeleton } from "@/components/ui/skeleton";
import { IconButton } from "@/components/shared/IconButton";
import { InspectorActions } from "@/components/shared/InspectorActions";
import { ShowFolderButton } from "@/components/host/ShowFolderButton";
import { CopyValue, PathText } from "@/components/shared/CopyValue";
import { HealthBadge } from "@/components/shared/StatusBadge";
import { isEligible, type ContainerAction } from "@/lib/actionEligibility";
import { useContainerDetail } from "@/hooks/useContainers";
import { LogViewer } from "./LogViewer";
import { formatCreated, shortId, uniquePortsLong } from "./format";
import { cpuCores, formatMemoryUsage, formatPercent } from "@/lib/statsFormat";
import { PortList } from "./PortLink";
import { externalTerminalReason, useOpenExternalTerminal } from "./externalTerminal";
import { ContainerStatusBadge } from "./ContainerStatusBadge";

interface Props {
  connectionId: string;
  container: Container;
  stat?: ContainerStat;
  busy: boolean;
  width: number;
  onResize: (w: number) => void;
  onClose: () => void;
  onAction: (action: ContainerAction, c: Container) => void;
}

function Row({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="grid grid-cols-[96px_1fr] gap-3 border-b border-border/50 py-1.5 text-[13px]">
      <dt className="text-muted-foreground">{label}</dt>
      <dd className="min-w-0 [overflow-wrap:anywhere]">{children}</dd>
    </div>
  );
}

function Overview({ connectionId, c }: { connectionId: string; c: Container }) {
  const { data: d, error, isLoading } = useContainerDetail(connectionId, c.id);
  const [reveal, setReveal] = useState(false);
  const ports = uniquePortsLong(c.ports ?? []);
  return (
    <dl>
      <Row label="Container ID"><CopyValue value={c.id} what="Container ID">{shortId(c.id)}</CopyValue></Row>
      <Row label="Image"><CopyValue value={c.image} what="Image name">{c.image}</CopyValue></Row>
      <Row label="Project">
        {c.composeProject ? (
          <Link to={`/compose/${encodeURIComponent(c.composeProject)}`} className="text-primary hover:underline">
            {c.composeProject}
          </Link>
        ) : (
          <span className="text-muted-foreground">Standalone</span>
        )}
      </Row>
      {c.composeService && <Row label="Service">{c.composeService}</Row>}
      <Row label="Created">{formatCreated(c.created)}</Row>
      <Row label="Ports">
        {ports.length ? (
          <PortList ports={c.ports ?? []} />
        ) : (
          <span className="text-muted-foreground">None</span>
        )}
      </Row>
      {error && <p className="py-2 text-xs text-destructive">Could not load details: {String(error)}</p>}
      {isLoading && <Skeleton className="mt-2 h-24 w-full" />}
      {d && (
        <>
          <Row label="Mounts">
            {d.mounts?.length ? (
              <div className="flex flex-col gap-1.5 font-mono text-xs">
                {d.mounts.map((m) => (
                  <div key={m.destination} className="min-w-0 rounded-md bg-muted/40 px-2 py-1.5">
                    <div className="flex items-center gap-1.5">
                      <span className="rounded bg-muted px-1 font-sans text-[11px] text-muted-foreground">{m.type}</span>
                      {m.readOnly && <span className="font-sans text-[11px] text-muted-foreground">read-only</span>}
                      {m.type === "bind" && <span className="ml-auto"><ShowFolderButton connectionId={c.connectionId} path={m.source} /></span>}
                    </div>
                    <div className="mt-0.5 text-muted-foreground"><PathText value={m.source} /></div>
                    <div>→ <PathText value={m.destination} /></div>
                  </div>
                ))}
              </div>
            ) : (
              <span className="text-muted-foreground">None</span>
            )}
          </Row>
          <Row label="Restart policy">{d.restartPolicy || "no"}</Row>
          <Row label="Networks">{d.networks?.join(", ") || "—"}</Row>
          <div className="pt-3">
            <div className="mb-1 flex items-center justify-between">
              <h4 className="text-xs font-medium text-muted-foreground">Environment</h4>
              <Button variant="ghost" size="sm" className="h-6 px-2 text-xs" onClick={() => setReveal((v) => !v)}>
                {reveal ? "Hide" : "Show"} values
              </Button>
            </div>
            {d.env?.length ? (
              <ul className="max-h-48 overflow-auto rounded-md bg-muted/40 p-2 font-mono text-xs">
                {d.env.map((e) => (
                  <li key={e.key} className="break-all">
                    <span className="text-muted-foreground">{e.key}=</span>
                    {e.masked ? <span title="Secret-like key: value withheld by the backend">••••••••</span> : reveal ? e.value : "••••••••"}
                  </li>
                ))}
              </ul>
            ) : (
              <p className="text-xs text-muted-foreground">No environment variables.</p>
            )}
          </div>
        </>
      )}
    </dl>
  );
}

function Logs({ connectionId, c }: { connectionId: string; c: Container }) {
  return (
    <div>
      <div className="mb-2 flex items-center justify-between">
        <span className="text-xs text-muted-foreground">Live stream</span>
        <Link to={`/containers/${c.id}?tab=logs`} className="text-xs text-primary hover:underline">
          Open full logs
        </Link>
      </div>
      <LogViewer connectionId={connectionId} containerId={c.id} containerName={c.name} compact className="h-[420px]" />
    </div>
  );
}

function Stats({ c, stat }: { c: Container; stat?: ContainerStat }) {
  if (c.state !== "running") return <p className="text-xs text-muted-foreground">Stats are available only while the container is running.</p>;
  const mem = stat?.memoryBytes != null ? formatMemoryUsage(stat.memoryBytes, stat.memoryLimit) : undefined;
  const cell = (label: string, v?: string) => (
    <div className="rounded-md border border-border bg-muted/30 p-3">
      <div className="text-xs text-muted-foreground">{label}</div>
      <div className="mt-1 text-base font-medium">{v ?? <span title="Not available">—</span>}</div>
    </div>
  );
  return (
    <div className="grid grid-cols-2 gap-2">
      {cell("CPU", stat?.cpuPercent != null ? `${stat.cpuPercent.toFixed(1)}% · ${cpuCores(stat.cpuPercent)}` : undefined)}
      {cell("Memory", mem ? `${mem.text}${mem.percent != null ? ` · ${formatPercent(mem.percent)}` : ""}` : undefined)}
      <p className="col-span-2 text-xs text-muted-foreground">
        Current sample. Network, block I/O and history graphs are on the full details page.
      </p>
    </div>
  );
}

export function ContainerInspector(p: Props) {
  const c = p.container;
  const openExternal = useOpenExternalTerminal();
  const startDrag = (e: React.PointerEvent) => {
    e.preventDefault();
    const startX = e.clientX;
    const startW = p.width;
    const move = (ev: PointerEvent) => p.onResize(startW + (startX - ev.clientX));
    const up = () => {
      window.removeEventListener("pointermove", move);
      window.removeEventListener("pointerup", up);
    };
    window.addEventListener("pointermove", move);
    window.addEventListener("pointerup", up);
  };
  return (
    <aside
      style={{ width: p.width }}
      className="relative flex shrink-0 flex-col border-l border-border bg-card max-[1199px]:absolute max-[1199px]:inset-y-0 max-[1199px]:right-0 max-[1199px]:z-30 max-[1199px]:max-w-[92%] max-[1199px]:shadow-2xl"
      aria-label={`Inspector for ${c.name}`}
    >
      <div
        role="separator"
        aria-orientation="vertical"
        aria-label="Resize inspector"
        onPointerDown={startDrag}
        className="absolute inset-y-0 -left-1 z-10 w-2 cursor-col-resize hover:bg-primary/30"
      />
      <div className="flex items-start gap-3 border-b border-border p-4">
        <Box className="mt-0.5 size-6 shrink-0 text-primary" />
        <div className="min-w-0 flex-1">
          <h2 className="truncate text-lg font-semibold" title={c.name}>{c.name}</h2>
          <div className="mt-1 flex items-center gap-2">
            <ContainerStatusBadge container={c} />
            <HealthBadge health={c.health} />
          </div>
        </div>
        <IconButton label="Close inspector" className="size-7" onClick={p.onClose}>
          <X className="size-4" />
        </IconButton>
      </div>
      <InspectorActions>
        <Button size="sm" variant="outline" className="px-2.5" disabled={p.busy || !isEligible("start", c.state)} onClick={() => p.onAction("start", c)}>
          <Play className="size-3.5" /> Start
        </Button>
        <Button size="sm" variant="outline" className="px-2.5" disabled={p.busy || !isEligible("stop", c.state)} onClick={() => p.onAction("stop", c)}>
          <Square className="size-3.5" /> Stop
        </Button>
        <Button size="sm" variant="outline" className="px-2.5" disabled={p.busy || !isEligible("restart", c.state)} onClick={() => p.onAction("restart", c)}>
          <RotateCw className="size-3.5" /> Restart
        </Button>
        <span aria-hidden className="mx-0.5 h-5 w-px bg-border" />
        {c.state === "running" ? (
          <IconButton label="Open terminal" variant="outline" asChild>
            <Link to={`/containers/${c.id}?tab=terminal`}><SquareTerminal className="size-4" /></Link>
          </IconButton>
        ) : (
          <IconButton label="Open terminal" tooltip={`Container is not running (state: ${c.state})`} variant="outline" disabled>
            <SquareTerminal className="size-4" />
          </IconButton>
        )}
        <IconButton
          label="Open in external terminal"
          tooltip={externalTerminalReason(c.state)}
          variant="outline"
          disabled={!!externalTerminalReason(c.state)}
          onClick={() => void openExternal(c.id)}
        >
          <AppWindow className="size-4" />
        </IconButton>
        <IconButton label="Open details" variant="outline" asChild>
          <Link to={`/containers/${c.id}`}><ExternalLink className="size-4" /></Link>
        </IconButton>
      </InspectorActions>
      <Tabs defaultValue="overview" className="min-h-0 flex-1 gap-0 px-4 pb-4 pt-3" key={c.id}>
        <TabsList className="mb-3 w-fit">
          <TabsTrigger value="overview">Overview</TabsTrigger>
          <TabsTrigger value="logs">Logs</TabsTrigger>
          <TabsTrigger value="stats">Stats</TabsTrigger>
        </TabsList>
        <div className="min-h-0 flex-1 overflow-auto">
          <TabsContent value="overview"><Overview connectionId={p.connectionId} c={c} /></TabsContent>
          <TabsContent value="logs"><Logs connectionId={p.connectionId} c={c} /></TabsContent>
          <TabsContent value="stats"><Stats c={c} stat={p.stat} /></TabsContent>
        </div>
      </Tabs>
    </aside>
  );
}
