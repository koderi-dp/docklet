import type { ImageLayersReport } from "@bindings/model";
import { formatBytes } from "@/components/containers/format";
import { largestLabel, totalsDiffer } from "./layerFormat";

function Stat({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="min-w-0">
      <div className="text-xs text-muted-foreground">{label}</div>
      <div className="truncate text-[13px] font-medium tabular-nums">{children}</div>
    </div>
  );
}

export function LayersSummary({ report }: { report: ImageLayersReport }) {
  const largest = largestLabel(report);
  const platform = report.os || report.architecture ? `${report.os}/${report.architecture}` : "—";
  return (
    <div className="space-y-2">
      <div className="grid grid-cols-2 gap-x-6 gap-y-2 rounded-[10px] border border-border bg-muted/30 px-4 py-3 sm:grid-cols-4">
        <Stat label="Total size">{report.totalSize > 0 ? formatBytes(report.totalSize) : "—"}</Stat>
        <Stat label="Layers with content">
          {report.contentSteps}
          <span className="font-normal text-muted-foreground">
            {" "}
            of {report.steps?.length ?? 0} steps{report.rootfsLayers > 0 ? ` · ${report.rootfsLayers} fs layers` : ""}
          </span>
        </Stat>
        <Stat label="Platform">{platform}</Stat>
        <Stat label="Built">{report.created ? new Date(report.created).toLocaleDateString() : "—"}</Stat>
      </div>
      {largest ? (
        <p className="flex min-w-0 items-baseline gap-1.5 text-[13px]" title={`${largest.text} · ${largest.size}, ${largest.share}`}>
          <span className="shrink-0 text-muted-foreground">Largest:</span>
          <span className="min-w-0 truncate font-mono text-xs">{largest.text}</span>
          <span className="shrink-0 text-muted-foreground">·</span>
          <span className="shrink-0 font-medium tabular-nums">
            {largest.size}, {largest.share}
          </span>
        </p>
      ) : (
        <p className="text-[13px] text-muted-foreground">The Engine reported no size for any step of this image.</p>
      )}
      {totalsDiffer(report) && (
        <p className="text-xs text-muted-foreground">
          Step sizes add up to {formatBytes(report.historySize)}, the image reports {formatBytes(report.totalSize)}. History sizes are what the Engine reports and the two can
          differ (for example with the containerd image store). Shares are relative to the step sizes.
        </p>
      )}
      {report.sharedBase && (
        <p className="text-xs text-muted-foreground">
          Shares {report.sharedBase.count} base layer{report.sharedBase.count === 1 ? "" : "s"} with <span className="font-mono text-foreground">{report.sharedBase.ref}</span>
        </p>
      )}
    </div>
  );
}
