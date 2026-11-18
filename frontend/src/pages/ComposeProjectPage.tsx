import { useMemo, useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import { Layers } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { ConnectionGate } from "@/components/shared/ConnectionGate";
import { EmptyState } from "@/components/shared/EmptyState";
import { PageHeader } from "@/components/shared/PageHeader";
import { ConfigTab } from "@/components/compose/ConfigTab";
import { LogsTab } from "@/components/compose/LogsTab";
import { ProjectActions } from "@/components/compose/ProjectActions";
import { ProjectHealthText, ProjectStateBadge } from "@/components/compose/ProjectStateBadge";
import { ProjectTotalsStrip } from "@/components/compose/ProjectTotalsStrip";
import { ResourcesTab } from "@/components/compose/ResourcesTab";
import { ServicesTab } from "@/components/compose/ServicesTab";
import { projectContainers, useComposeProject } from "@/hooks/useCompose";
import { useContainersQuery, useContainerStats } from "@/hooks/useContainers";
import { useProjectTotals } from "@/hooks/useProjectTotals";

export default function ComposeProjectPage() {
  const { projectName = "" } = useParams();
  return <ConnectionGate>{({ connectionId }) => <ProjectView connectionId={connectionId} name={projectName} />}</ConnectionGate>;
}

function ProjectView({ connectionId, name }: { connectionId: string; name: string }) {
  const [tab, setTab] = useState("services");
  const navigate = useNavigate();
  const q = useComposeProject(connectionId, name);
  const all = useContainersQuery(connectionId, true);
  const containers = useMemo(() => projectContainers(all.data, { name }), [all.data, name]);
  const runningIds = useMemo(() => containers.filter((c) => c.state === "running").map((c) => c.id), [containers]);
  const { stats } = useContainerStats(connectionId, runningIds, true);
  const live = useProjectTotals(connectionId, containers, stats);

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

  return (
    <div className="h-full overflow-auto p-5">
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
        actions={<ProjectActions connectionId={connectionId} project={p} containers={containers} labels onForgotten={() => (p.notCreated ? navigate("/compose") : undefined)} />}
      />
      <ProjectTotalsStrip connectionId={connectionId} project={p} containers={containers} totals={live.totals.get(name)} idle={live.idle.get(name)} />
      <Tabs value={tab} onValueChange={setTab} className="gap-3">
        <TabsList className="w-fit">
          <TabsTrigger value="services">Services</TabsTrigger>
          <TabsTrigger value="logs">Logs</TabsTrigger>
          <TabsTrigger value="configuration">Configuration</TabsTrigger>
          <TabsTrigger value="resources">Resources</TabsTrigger>
        </TabsList>
        <TabsContent value="services">
          <ServicesTab connectionId={connectionId} project={p} containers={containers} />
        </TabsContent>
        <TabsContent value="logs">
          <LogsTab connectionId={connectionId} project={p.name} services={services} active={tab === "logs"} />
        </TabsContent>
        <TabsContent value="configuration">
          <ConfigTab connectionId={connectionId} project={p} active={tab === "configuration"} />
        </TabsContent>
        <TabsContent value="resources">
          <ResourcesTab connectionId={connectionId} project={p.name} />
        </TabsContent>
      </Tabs>
    </div>
  );
}
