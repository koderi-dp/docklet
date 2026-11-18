import { useNavigate } from "react-router-dom";
import { Box, Download, FileStack, HardDrive, Image as ImageIcon, Network as NetworkIcon, Plus } from "lucide-react";
import { Button } from "@/components/ui/button";
import { ConnectionGate } from "@/components/shared/ConnectionGate";
import { MetricCard } from "@/components/shared/MetricCard";
import { PageHeader } from "@/components/shared/PageHeader";
import { EngineCard, RecentContainersCard, RecentEventsCard, ResourceUsageCard } from "@/components/dashboard/DashboardCards";
import { useContainersQuery } from "@/hooks/useContainers";
import { useNetworksQuery } from "@/hooks/useNetworks";
import { useDiskUsage, useEngineInfo, useRecentEvents } from "@/hooks/useSystem";
import { useVolumesQuery } from "@/hooks/useVolumes";

export default function DashboardPage() {
  return <ConnectionGate>{({ connectionId }) => <DashboardView connectionId={connectionId} />}</ConnectionGate>;
}

/** Count or an em dash while loading / on error; never a fabricated zero. */
const num = (n: number | undefined) => (n == null ? "—" : n);

function DashboardView({ connectionId }: { connectionId: string }) {
  const navigate = useNavigate();
  const containers = useContainersQuery(connectionId, true);
  const info = useEngineInfo(connectionId);
  const volumes = useVolumesQuery(connectionId);
  const networks = useNetworksQuery(connectionId);
  const disk = useDiskUsage(connectionId);
  const events = useRecentEvents(connectionId, 3600, 8, true);

  const list = containers.data?.filter((c) => c.connectionId === connectionId);
  const running = list?.filter((c) => c.state === "running").length;
  const stopped = list && running != null ? list.length - running : undefined;

  return (
    <div className="h-full overflow-auto p-5">
      <PageHeader
        title="Dashboard"
        subtitle="Overview of your Docker Engine"
        actions={
          <>
            <Button size="sm" onClick={() => navigate("/containers")}>
              <Plus className="size-4" /> New Container
            </Button>
            <Button size="sm" variant="outline" onClick={() => navigate("/images")}>
              <Download className="size-4" /> Pull Image
            </Button>
            <Button size="sm" variant="outline" onClick={() => navigate("/compose")}>
              <FileStack className="size-4" /> Open Compose
            </Button>
          </>
        }
      />

      <div className="mb-4 grid grid-cols-4 gap-3">
        <MetricCard icon={<Box />} label="Containers" value={num(list?.length)} hint={list ? `${running} running · ${stopped} stopped` : undefined} />
        <MetricCard icon={<ImageIcon />} tone="info" label="Images" value={num(info.data?.images)} />
        <MetricCard icon={<HardDrive />} tone="warning" label="Volumes" value={num(volumes.data?.length)} />
        <MetricCard icon={<NetworkIcon />} tone="success" label="Networks" value={num(networks.data?.length)} />
      </div>

      <div className="grid grid-cols-1 gap-4 xl:grid-cols-2">
        <EngineCard info={info} />
        <ResourceUsageCard info={info} disk={disk} />
        <RecentContainersCard containers={containers} onViewAll={() => navigate("/containers")} />
        <RecentEventsCard events={events} />
      </div>
    </div>
  );
}
