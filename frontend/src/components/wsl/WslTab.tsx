import { Activity, RefreshCw } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { EmptyState } from "@/components/shared/EmptyState";
import { useWslActions, useWslInfo } from "@/hooks/useWsl";
import { errorMessage } from "@/lib/errors";
import { WslActionsCard } from "./WslActionsCard";
import { WslDetails, WslDiskNote } from "./WslDetails";
import { WslSummary } from "./WslSummary";

/** System page tab for a connection whose engine runs in a WSL distribution (Windows only). */
export function WslTab({ connectionId }: { connectionId: string }) {
  const q = useWslInfo(connectionId);
  const run = useWslActions(connectionId);
  if (q.isLoading) return <Skeleton className="h-72 w-full" />;
  if (q.error || !q.data)
    return (
      <EmptyState icon={<Activity />} title="Could not read the WSL distribution">
        <p className="text-destructive">{errorMessage(q.error)}</p>
        <Button className="mt-3" onClick={() => void q.refetch()}>Retry</Button>
      </EmptyState>
    );
  const info = q.data;
  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between gap-3">
        <p className="text-[13px] text-muted-foreground">
          State of <span className="font-medium text-foreground">{info.distro}</span>. Refreshes every 10 seconds while this tab is open.
        </p>
        <Button size="sm" variant="outline" disabled={q.isFetching} onClick={() => void q.refetch()}>
          <RefreshCw className={q.isFetching ? "animate-spin" : undefined} />
          Refresh
        </Button>
      </div>
      <WslSummary info={info} />
      {(info.notes ?? []).length > 0 && (
        <ul className="space-y-1 text-[13px] text-muted-foreground">
          {(info.notes ?? []).map((n) => <li key={n}>{n}</li>)}
        </ul>
      )}
      <div className="grid grid-cols-1 gap-4 xl:grid-cols-2">
        <div className="space-y-4">
          <WslDetails info={info} />
          <WslDiskNote info={info} />
        </div>
        <WslActionsCard info={info} busy={run.isPending} onRun={(id) => run.mutate(id)} />
      </div>
    </div>
  );
}
