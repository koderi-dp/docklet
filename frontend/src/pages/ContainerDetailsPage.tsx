import { lazy, Suspense, useEffect, useMemo, useState } from "react";
import { useNavigate, useParams, useSearchParams } from "react-router-dom";
import { useQueryClient } from "@tanstack/react-query";
import { Events } from "@wailsio/runtime";
import { Box, Play } from "lucide-react";
import { toast } from "sonner";
import type { Container } from "@bindings/model";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { ConnectionGate } from "@/components/shared/ConnectionGate";
import { EmptyState } from "@/components/shared/EmptyState";
import { LogViewer } from "@/components/containers/LogViewer";
import { StatsPanel } from "@/components/containers/StatsPanel";
import { InspectPanel } from "@/components/containers/InspectPanel";
import { ProcessesPanel } from "@/components/containers/ProcessesPanel";
import { OverviewPanel } from "@/components/containers/OverviewPanel";
import { ContainerDetailHeader } from "@/components/containers/ContainerDetailHeader";
import { ContainerFilesTab } from "@/components/files/ContainerFilesTab";
import { useContainerAction, useContainerDetail, useContainersQuery } from "@/hooks/useContainers";
import { ACTION_LABEL, ineligibleReason, isEligible, type ContainerAction } from "@/lib/actionEligibility";
import { errorMessage } from "@/lib/errors";

// xterm is large: load it only when a terminal is actually opened.
const TerminalPanel = lazy(() => import("@/components/containers/TerminalPanel"));

const TABS = ["overview", "logs", "terminal", "stats", "files", "inspect", "processes"] as const;
type TabId = (typeof TABS)[number];
const isTab = (v: string | null): v is TabId => !!v && (TABS as readonly string[]).includes(v);

export default function ContainerDetailsPage() {
  const { containerId = "" } = useParams();
  return <ConnectionGate>{({ connectionId }) => <Detail connectionId={connectionId} param={containerId} />}</ConnectionGate>;
}

function Detail({ connectionId, param }: { connectionId: string; param: string }) {
  const navigate = useNavigate();
  const qc = useQueryClient();
  const [search, setSearch] = useSearchParams();
  const tab: TabId = isTab(search.get("tab")) ? (search.get("tab") as TabId) : "overview";

  const list = useContainersQuery(connectionId, true);
  const c: Container | undefined = useMemo(
    () => list.data?.find((x) => x.id === param) ?? list.data?.find((x) => x.id.startsWith(param) || x.name === param),
    [list.data, param],
  );
  const detail = useContainerDetail(connectionId, c?.id ?? null);
  const mutation = useContainerAction(connectionId);
  const [confirm, setConfirm] = useState<"kill" | "remove" | null>(null);
  const [busy, setBusy] = useState(false);

  // Keep detail data in step with Engine events for this connection.
  useEffect(() => {
    const off = Events.On("containers:changed", (ev: { data?: unknown }) => {
      const raw = Array.isArray(ev.data) ? ev.data[0] : ev.data;
      const evConn = (raw as { connectionId?: string } | undefined)?.connectionId;
      if (evConn && evConn !== connectionId) return;
      void qc.invalidateQueries({ queryKey: ["container-detail", connectionId] });
    });
    return () => off();
  }, [qc, connectionId]);

  if (list.isLoading)
    return (
      <div className="space-y-3 p-5">
        <Skeleton className="h-8 w-64" />
        <Skeleton className="h-10 w-full" />
        <Skeleton className="h-64 w-full" />
      </div>
    );
  if (list.error)
    return (
      <EmptyState title="Could not load containers">
        <p className="text-destructive">{String(list.error)}</p>
        <Button className="mt-3" onClick={() => void list.refetch()}>Retry</Button>
      </EmptyState>
    );
  if (!c)
    return (
      <EmptyState icon={<Box />} title="Container not found">
        <p>It may have been removed, or it belongs to another Engine.</p>
        <Button className="mt-3" onClick={() => navigate("/containers")}>Back to containers</Button>
      </EmptyState>
    );

  const running = c.state === "running";

  const run = async (action: ContainerAction) => {
    setBusy(true);
    try {
      const out = await mutation.mutateAsync({ action, ids: [c.id], skippedCount: 0, force: action === "remove" && (running || c.state === "paused" || c.state === "restarting") });
      const r = out.results[0];
      const verb = ACTION_LABEL[action];
      if (r?.ok) {
        toast.success(`${verb}: ${c.name}`);
        if (action === "remove") navigate("/containers", { replace: true });
      } else if (r?.skipped) {
        toast.info(`${verb}: nothing to do`, { description: `${c.name} is not eligible right now.` });
      } else {
        toast.error(`${verb} failed`, { description: `${c.name}: ${r?.error ?? "unknown error"}`, duration: Infinity, closeButton: true });
      }
    } catch (e) {
      toast.error(`${ACTION_LABEL[action]} failed`, { description: errorMessage(e), duration: Infinity, closeButton: true });
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="flex h-full min-h-0 flex-col p-5">
      <ContainerDetailHeader
        container={c}
        health={detail.data?.health}
        busy={busy}
        onRun={(a) => void run(a)}
        onConfirm={setConfirm}
      />

      <Tabs value={tab} onValueChange={(v) => setSearch(v === "overview" ? {} : { tab: v }, { replace: true })} className="min-h-0 flex-1 gap-0">
        <TabsList className="mb-3 w-fit">
          <TabsTrigger value="overview">Overview</TabsTrigger>
          <TabsTrigger value="logs">Logs</TabsTrigger>
          <TabsTrigger value="terminal">Terminal</TabsTrigger>
          <TabsTrigger value="stats">Stats</TabsTrigger>
          <TabsTrigger value="files">Files</TabsTrigger>
          <TabsTrigger value="inspect">Inspect</TabsTrigger>
          <TabsTrigger value="processes">Processes</TabsTrigger>
        </TabsList>

        <TabsContent value="overview" className="min-h-0 flex-1 overflow-auto">
          {detail.isLoading && <Skeleton className="h-64 w-full" />}
          {detail.error && <p className="text-sm text-destructive">Could not load details: {String(detail.error)}</p>}
          {detail.data && <OverviewPanel c={c} d={detail.data} />}
        </TabsContent>

        <TabsContent value="logs" className="flex min-h-0 flex-1 flex-col">
          <LogViewer connectionId={connectionId} containerId={c.id} containerName={c.name} className="flex-1" />
        </TabsContent>

        <TabsContent value="terminal" className="min-h-0 flex-1 overflow-auto">
          {running ? (
            <Suspense fallback={<Skeleton className="h-64 w-full" />}>
              <TerminalPanel connectionId={connectionId} containerId={c.id} running={running} />
            </Suspense>
          ) : (
            <EmptyState title="Start container to open a terminal">
              <p>A terminal needs a running container (current state: {c.state}).</p>
              {isEligible("start", c.state) ? (
                <Button className="mt-3" onClick={() => void run("start")} disabled={busy}>
                  <Play className="size-4" /> Start container
                </Button>
              ) : (
                <p className="mt-2">{ineligibleReason("start", c.state)}</p>
              )}
            </EmptyState>
          )}
        </TabsContent>

        <TabsContent value="stats" className="min-h-0 flex-1 overflow-auto">
          <StatsPanel connectionId={connectionId} containerId={c.id} running={running} />
        </TabsContent>

        <TabsContent value="files" className="flex min-h-0 flex-1 flex-col">
          <ContainerFilesTab connectionId={connectionId} containerId={c.id} containerName={c.name} state={c.state} />
        </TabsContent>

        <TabsContent value="inspect" className="flex min-h-0 flex-1 flex-col">
          <InspectPanel connectionId={connectionId} containerId={c.id} name={c.name} />
        </TabsContent>

        <TabsContent value="processes" className="min-h-0 flex-1 overflow-auto">
          <ProcessesPanel connectionId={connectionId} containerId={c.id} running={running} />
        </TabsContent>
      </Tabs>

      <AlertDialog open={confirm !== null} onOpenChange={(o) => !o && setConfirm(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{confirm === "kill" ? "Kill" : "Delete"} container {c.name}?</AlertDialogTitle>
            <AlertDialogDescription asChild>
              <div>
                {confirm === "kill" ? (
                  <p>Sends SIGKILL to the main process of <span className="font-mono">{c.name}</span> immediately, without a graceful shutdown. Unsaved data in the container may be lost.</p>
                ) : (
                  <>
                    <p>This permanently removes <span className="font-mono">{c.name}</span>. Volumes are not removed. This cannot be undone.</p>
                    {c.state !== "exited" && c.state !== "created" && c.state !== "dead" && (
                      <p className="mt-2 text-destructive">The container is {c.state} and will be force-killed before removal.</p>
                    )}
                  </>
                )}
              </div>
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction
              className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
              onClick={() => {
                const a = confirm as "kill" | "remove";
                setConfirm(null);
                void run(a);
              }}
            >
              {confirm === "kill" ? "Kill" : "Delete"}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}
