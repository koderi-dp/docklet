import { useEffect, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { RefreshCw } from "lucide-react";
import { ContainerService } from "@bindings/services";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { EmptyState } from "@/components/shared/EmptyState";

/** Engine "top" output; refreshed every 5s while this tab is mounted and the window is visible. */
export function ProcessesPanel({ connectionId, containerId, running }: { connectionId: string; containerId: string; running: boolean }) {
  const [visible, setVisible] = useState(() => document.visibilityState !== "hidden");
  useEffect(() => {
    const on = () => setVisible(document.visibilityState !== "hidden");
    document.addEventListener("visibilitychange", on);
    return () => document.removeEventListener("visibilitychange", on);
  }, []);
  const q = useQuery({
    queryKey: ["container-top", connectionId, containerId],
    queryFn: () => ContainerService.Top(connectionId, containerId),
    enabled: running,
    refetchInterval: visible ? 5000 : false,
    retry: false,
  });
  if (!running)
    return (
      <EmptyState title="Container is not running">
        <p>The process list is available only while the container is running.</p>
      </EmptyState>
    );
  if (q.isLoading) return <Skeleton className="h-40 w-full" />;
  if (q.error) return <p className="text-sm text-destructive">Could not list processes: {String(q.error)}</p>;
  const titles = q.data?.titles ?? [];
  const rows = q.data?.processes ?? [];
  return (
    <div className="space-y-2">
      <div className="flex items-center justify-between">
        <span className="text-xs text-muted-foreground">
          {rows.length} process{rows.length === 1 ? "" : "es"} · refreshes every 5s
        </span>
        <Button size="sm" variant="outline" onClick={() => void q.refetch()} disabled={q.isFetching}>
          <RefreshCw className="size-3.5" /> Refresh
        </Button>
      </div>
      <div className="overflow-auto rounded-[10px] border border-border bg-card">
        <table className="w-full border-collapse text-[13px]">
          <thead>
            <tr className="border-b border-border text-left text-xs text-muted-foreground">
              {titles.map((t) => (
                <th key={t} className="h-9 whitespace-nowrap px-3 font-medium">
                  {t}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {rows.map((r, i) => (
              <tr key={i} className="border-b border-border/60 last:border-0">
                {(r ?? []).map((c, j) => (
                  <td key={j} className="px-3 py-1.5 font-mono text-xs">
                    {c}
                  </td>
                ))}
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}
