import { Activity, RefreshCw } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { EmptyState } from "@/components/shared/EmptyState";
import { useDiskUsage } from "@/hooks/useSystem";
import { cn } from "@/lib/utils";
import { Bytes, Card } from "./parts";

export function DiskUsageTab({ connectionId }: { connectionId: string }) {
  const q = useDiskUsage(connectionId);
  if (q.isLoading) return <Skeleton className="h-60 w-full" />;
  if (q.error || !q.data)
    return (
      <EmptyState icon={<Activity />} title="Could not load Docker disk usage">
        <p className="text-destructive">{String(q.error)}</p>
        <Button className="mt-3" onClick={() => void q.refetch()}>Retry</Button>
      </EmptyState>
    );
  return (
    <Card
      title="Docker disk usage"
      subtitle="Space used by Docker objects, as reported by the Engine (docker system df). This is not the free or used space of the host disk."
      actions={
        <Button size="sm" variant="outline" onClick={() => void q.refetch()} disabled={q.isFetching}>
          <RefreshCw className={cn("size-3.5", q.isFetching && "animate-spin")} /> Refresh
        </Button>
      }
    >
      <table className="w-full border-collapse text-[13px]">
        <thead>
          <tr className="border-b border-border text-left text-xs font-medium text-muted-foreground">
            <th className="h-9 px-2 font-medium">Category</th>
            <th className="px-2 text-right font-medium">Total</th>
            <th className="px-2 text-right font-medium">Active</th>
            <th className="px-2 text-right font-medium">Size</th>
            <th className="px-2 text-right font-medium">Reclaimable</th>
          </tr>
        </thead>
        <tbody>
          {(q.data.categories ?? []).map((c) => (
            <tr key={c.key} className="h-[42px] border-b border-border/60">
              <td className="px-2 font-medium">{c.label}</td>
              <td className="px-2 text-right tabular-nums">{c.totalCount}</td>
              <td className="px-2 text-right tabular-nums">{c.activeCount}</td>
              <td className="px-2 text-right"><Bytes value={c.size} reason="The Engine did not report a size for this category (for example, non-local volume drivers)" /></td>
              <td className="px-2 text-right"><Bytes value={c.reclaimable} reason="The Engine did not report reclaimable space for this category" /></td>
            </tr>
          ))}
        </tbody>
      </table>
      <p className="mt-3 text-xs text-muted-foreground">
        Reclaimable is the Engine&apos;s estimate of space held by objects no container uses. Shared image layers can make category sizes overlap.
      </p>
    </Card>
  );
}
