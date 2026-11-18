import { Activity, Box, Play, Square } from "lucide-react";
import { MetricCard } from "@/components/shared/MetricCard";
import type { SummaryCounts } from "@/lib/composeGrouping";

const pct = (n: number, total: number) => (total ? `${((n / total) * 100).toFixed(1)}% of total` : undefined);

/** Four compact KPI cards; the hint line drops out (container query) before the icon does. */
export function ContainerSummaryCards({ summary }: { summary: SummaryCounts }) {
  return (
    <div className="mb-4 grid grid-cols-4 gap-3">
      <MetricCard icon={<Box />} label="Total Containers" value={summary.total} />
      <MetricCard icon={<Play />} tone="success" label="Running" value={summary.running} hint={pct(summary.running, summary.total)} />
      <MetricCard icon={<Square />} tone="muted" label="Stopped" value={summary.stopped}
        hint={`${summary.completed} completed · ${summary.failed} failed`}
      />
      <MetricCard
        icon={<Activity />}
        tone="success"
        label="Healthy"
        value={summary.healthy}
        hint={`${summary.healthy} of ${summary.withHealthcheck} with healthchecks`}
      />
    </div>
  );
}
