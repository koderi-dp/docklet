import { lazy, Suspense, useEffect, useMemo, useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import { Layers } from "lucide-react";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { Skeleton } from "@/components/ui/skeleton";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { ConnectionGate } from "@/components/shared/ConnectionGate";
import { EmptyState } from "@/components/shared/EmptyState";
import { PageHeader } from "@/components/shared/PageHeader";
import { ConfigTab } from "@/components/compose/ConfigTab";
import { ApplyButton } from "@/components/compose/file/ApplyButton";
import { ApplyPanel } from "@/components/compose/file/ApplyPanel";
import { DiscardChangesDialog } from "@/components/compose/file/DiscardChangesDialog";
import { LogsTab } from "@/components/compose/LogsTab";
import { ProjectActions } from "@/components/compose/ProjectActions";
import { ProjectHealthText, ProjectStateBadge } from "@/components/compose/ProjectStateBadge";
import { ProjectTotalsStrip } from "@/components/compose/ProjectTotalsStrip";
import { ResourcesTab } from "@/components/compose/ResourcesTab";
import { ServicesTab } from "@/components/compose/ServicesTab";
import type { ServiceFilter } from "@/lib/serviceRows";
import { StructureTab } from "@/components/compose/structure/StructureTab";
import { WatchTab } from "@/components/compose/watch/WatchTab";
import { projectContainers, useComposeProject } from "@/hooks/useCompose";
import { useContainersQuery, useContainerStats } from "@/hooks/useContainers";
import { useProjectTotals } from "@/hooks/useProjectTotals";
import { useApplyPlan, useComposeFiles } from "@/hooks/useComposeFiles";
import { useWatchState } from "@/hooks/useComposeWatch";
import { errorMessage } from "@/lib/errors";
import { pendingChanges, type ApplyOptions } from "@/lib/applyPlanView";

// CodeMirror stays out of the page's own chunk until the file tab is opened.
const ComposeFileTab = lazy(() => import("@/components/compose/file/ComposeFileTab").then((m) => ({ default: m.ComposeFileTab })));

export default function ComposeProjectPage() {
  const { projectName = "" } = useParams();
  return <ConnectionGate>{({ connectionId }) => <ProjectView connectionId={connectionId} name={projectName} />}</ConnectionGate>;
}

function ProjectView({ connectionId, name }: { connectionId: string; name: string }) {
  const [tab, setTab] = useState("structure");
  const [serviceFocus, setServiceFocus] = useState<string | undefined>();
  const [serviceFilter, setServiceFilter] = useState<ServiceFilter>("all");
  useEffect(() => setServiceFilter("all"), [name]);
  const [fileDirty, setFileDirty] = useState(false);
  const [pendingTab, setPendingTab] = useState<string | null>(null);
  const [applyOpts, setApplyOpts] = useState<ApplyOptions>({ removeOrphans: false, profiles: [] });
  const [reveal, setReveal] = useState<{ service: string; nonce: number }>();
  // A request is for one visit of the file tab; coming back later must not jump again.
  useEffect(() => {
    if (tab !== "file") setReveal(undefined);
  }, [tab]);
  const navigate = useNavigate();
  const q = useComposeProject(connectionId, name);
  const canShowInFile = useComposeFiles(connectionId, name, tab === "structure").data?.canEdit === true;
  const cap = q.data?.capability;
  const plan = useApplyPlan(connectionId, name, applyOpts.profiles, tab === "file" && !!q.data && !!cap?.canRun);
  const all = useContainersQuery(connectionId, true);
  const containers = useMemo(() => projectContainers(all.data, { name }), [all.data, name]);
  const runningIds = useMemo(() => containers.filter((c) => c.state === "running").map((c) => c.id), [containers]);
  const { stats } = useContainerStats(connectionId, runningIds, true);
  const live = useProjectTotals(connectionId, containers, stats);
  const watch = useWatchState(connectionId, name).data;
  const watchActive = watch?.active === true;
  useEffect(() => {
    if (tab === "watch" && !watchActive) setTab("structure");
  }, [tab, watchActive]);

  const back = (
    <Link to="/compose" className="hover:underline">
      Compose
    </Link>
  );

  if (q.isLoading)
    return (
      <div className="space-y-4 p-5">
        <Skeleton className="h-10 w-64" />
        <Skeleton className="h-64 w-full" />
      </div>
    );
  if (q.error || !q.data)
    return (
      <EmptyState icon={<Layers />} title={`Project "${name}" was not found`}>
        <p>{q.error ? (q.error as Error).message : "No container on this Engine carries this Compose project label and no saved project has this name."}</p>
        <Button className="mt-3" size="sm" variant="outline" asChild>
          <Link to="/compose">Back to Compose</Link>
        </Button>
      </EmptyState>
    );

  const p = q.data;
  const services = (p.services ?? []).map((s) => s.name).filter(Boolean);
  const pendingApply = !plan.data?.error && pendingChanges(plan.data?.items ?? [], applyOpts.removeOrphans);

  return (
    <div className={cn("h-full overflow-auto p-5", (tab === "logs" || tab === "structure" || tab === "file") && "flex flex-col")}>
      <PageHeader
        breadcrumb={back}
        title={p.name}
        subtitle={
          <span className="flex flex-wrap items-center gap-2">
            {p.notCreated ? <ProjectStateBadge containers={[]} notCreated /> : all.data ? <ProjectStateBadge containers={containers} /> : null}
            {all.data && !p.notCreated ? <ProjectHealthText containers={containers} /> : null}
            <span className="text-xs">
              {p.source} · {(p.services ?? []).length} services · {p.running} of {p.total} containers running
            </span>
          </span>
        }
        actions={
          <div className="flex items-center gap-2">
            {tab === "file" && (
              <ApplyButton
                connectionId={connectionId}
                name={name}
                options={applyOpts}
                dirty={fileDirty}
                planError={!!plan.data?.error || !!plan.error}
                planLoading={plan.isFetching}
                canRun={p.capability.canRun}
                capReason={p.capability.reason ?? ""}
                pending={pendingApply}
              />
            )}
            <ProjectActions connectionId={connectionId} project={p} containers={containers} labels onForgotten={() => (p.notCreated ? navigate("/compose") : undefined)} />
          </div>
        }
      />
      <ProjectTotalsStrip connectionId={connectionId} project={p} containers={containers} totals={live.totals.get(name)} idle={live.idle.get(name)} />
      <Tabs value={tab} onValueChange={(v) => {
          if (tab === "file" && v !== "file" && fileDirty) return setPendingTab(v);
          setTab(v);
          setServiceFocus(undefined);
        }} className={cn("gap-3", tab === "logs" && "min-h-[320px] flex-1", (tab === "structure" || tab === "file") && "min-h-[420px] flex-1")}>
        <TabsList className="w-fit">
          <TabsTrigger value="structure">Structure</TabsTrigger>
          <TabsTrigger value="services">Services</TabsTrigger>
          <TabsTrigger value="file">Compose file</TabsTrigger>
          <TabsTrigger value="logs">Logs</TabsTrigger>
          <TabsTrigger value="configuration">Configuration</TabsTrigger>
          <TabsTrigger value="resources">Resources</TabsTrigger>
          {watch?.active && (
            <TabsTrigger value="watch" className="gap-1.5">
              Watch
              <span className={cn("size-1.5 rounded-full", watch.running ? "bg-success" : "bg-destructive")} aria-hidden />
            </TabsTrigger>
          )}
        </TabsList>
        <TabsContent value="structure" className="min-h-0 data-[state=active]:flex data-[state=active]:flex-col">
          <StructureTab
            connectionId={connectionId}
            project={p}
            containers={containers}
            active={tab === "structure"}
            onShowService={(service) => {
              setServiceFocus(service);
              setTab("services");
            }}
            canShowInFile={canShowInFile}
            onShowInFile={(service) => {
              setReveal((r) => ({ service, nonce: (r?.nonce ?? 0) + 1 }));
              setTab("file");
            }}
          />
        </TabsContent>
        <TabsContent value="services">
          <ServicesTab connectionId={connectionId} project={p} containers={containers} focusService={serviceFocus} filter={serviceFilter} onFilterChange={setServiceFilter} />
        </TabsContent>
        <TabsContent value="file" className="min-h-0 data-[state=active]:flex data-[state=active]:flex-col">
          <Suspense fallback={<Skeleton className="h-full min-h-64 w-full" />}>
            <ComposeFileTab
              connectionId={connectionId}
              project={p}
              containers={containers}
              active={tab === "file"}
              onDirtyChange={setFileDirty}
              pendingApply={pendingApply}
              reveal={reveal}
              panel={
                <ApplyPanel
                  plan={plan.data}
                  loading={plan.isLoading}
                  dirty={fileDirty}
                  options={applyOpts}
                  onOptions={setApplyOpts}
                  canRun={p.capability.canRun}
                  capReason={p.capability.reason ?? ""}
                  queryError={plan.error ? errorMessage(plan.error) : undefined}
                />
              }
            />
          </Suspense>
        </TabsContent>
        <TabsContent value="logs" className="min-h-0 data-[state=active]:flex data-[state=active]:flex-col">
          <LogsTab connectionId={connectionId} project={p.name} services={services} active={tab === "logs"} />
        </TabsContent>
        <TabsContent value="configuration">
          <ConfigTab connectionId={connectionId} project={p} active={tab === "configuration"} />
        </TabsContent>
        <TabsContent value="watch">
          <WatchTab connectionId={connectionId} project={p} />
        </TabsContent>
        <TabsContent value="resources">
          <ResourcesTab connectionId={connectionId} project={p.name} />
        </TabsContent>
      </Tabs>
      <DiscardChangesDialog
        open={pendingTab !== null}
        onKeep={() => setPendingTab(null)}
        onDiscard={() => {
          setFileDirty(false);
          setTab(pendingTab ?? "structure");
          setServiceFocus(undefined);
          setPendingTab(null);
        }}
      />
    </div>
  );
}
