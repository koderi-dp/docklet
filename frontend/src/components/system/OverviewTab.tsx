import { Activity } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { EmptyState } from "@/components/shared/EmptyState";
import { formatBytes } from "@/components/containers/format";
import { useEngineInfo } from "@/hooks/useSystem";
import { Card, InfoGrid } from "./parts";

export function OverviewTab({ connectionId }: { connectionId: string }) {
  const q = useEngineInfo(connectionId);
  if (q.isLoading) return <Skeleton className="h-72 w-full" />;
  if (q.error || !q.data)
    return (
      <EmptyState icon={<Activity />} title="Could not load Engine information">
        <p className="text-destructive">{String(q.error)}</p>
        <Button className="mt-3" onClick={() => void q.refetch()}>Retry</Button>
      </EmptyState>
    );
  const i = q.data;
  return (
    <div className="space-y-4">
      <Card title="Engine" subtitle="As reported by the Docker daemon of this connection">
        <InfoGrid
          items={[
            { label: "Daemon name", value: i.name },
            { label: "Server version", value: i.serverVersion },
            { label: "API version", value: i.apiVersion ? `${i.apiVersion}${i.minApiVersion ? ` (min ${i.minApiVersion})` : ""}` : "" },
            { label: "Operating system", value: [i.operatingSystem, i.osType].filter(Boolean).join(" · ") },
            { label: "Kernel", value: i.kernelVersion },
            { label: "Architecture", value: i.architecture },
            { label: "CPUs", value: i.ncpu ? String(i.ncpu) : "" },
            { label: "Total memory", value: i.memTotal ? formatBytes(i.memTotal) : "" },
            { label: "Server time", value: i.serverTime ? new Date(i.serverTime).toLocaleString() : "" },
          ]}
        />
      </Card>
      <Card title="Runtime and storage">
        <InfoGrid
          items={[
            { label: "Storage driver", value: i.storageDriver },
            { label: "Docker root dir", value: <span className="font-mono text-xs">{i.dockerRootDir}</span> },
            { label: "Logging driver", value: i.loggingDriver },
            { label: "Cgroup", value: [i.cgroupDriver, i.cgroupVersion && `v${i.cgroupVersion}`].filter(Boolean).join(" · ") },
            { label: "Default runtime", value: i.defaultRuntime },
            { label: "Runtimes", value: (i.runtimes ?? []).join(", ") },
            { label: "Security options", value: (i.securityOptions ?? []).join(", ") },
          ]}
        />
      </Card>
      <Card title="Objects">
        <InfoGrid
          items={[
            { label: "Containers", value: `${i.containers} (${i.containersRunning} running, ${i.containersPaused} paused, ${i.containersStopped} stopped)` },
            { label: "Images", value: String(i.images) },
          ]}
        />
      </Card>
      {(i.warnings ?? []).length > 0 && (
        <Card title="Engine warnings">
          <ul className="list-disc space-y-1 pl-5 text-[13px] text-warning">
            {(i.warnings ?? []).map((w) => <li key={w}>{w}</li>)}
          </ul>
        </Card>
      )}
    </div>
  );
}
