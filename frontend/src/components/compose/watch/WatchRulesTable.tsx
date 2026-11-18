import { ArrowRight } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import type { ComposeWatchRule, ComposeWatchService, ComposeWatchStat } from "@bindings/model";
import { cn } from "@/lib/utils";
import { formatClock, ruleTarget, SYNC_ACTIONS } from "./watchFormat";

const ACTION_TONE: Record<string, string> = {
  sync: "text-info",
  "sync+restart": "text-info",
  "sync+exec": "text-info",
  rebuild: "text-warning",
  restart: "text-warning",
};

function Ignore({ patterns }: { patterns: readonly string[] }) {
  if (patterns.length === 0) return <span className="text-muted-foreground">—</span>;
  const text = patterns.join(", ");
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <span className="block truncate font-mono text-xs text-muted-foreground">{text}</span>
      </TooltipTrigger>
      <TooltipContent className="max-w-sm break-all font-mono">{patterns.join("\n")}</TooltipContent>
    </Tooltip>
  );
}

function Counters({ stat }: { stat?: ComposeWatchStat }) {
  if (!stat || (stat.syncs === 0 && stat.rebuilds === 0 && stat.restarts === 0)) return <span className="text-xs text-muted-foreground">No activity yet</span>;
  const parts = [
    stat.syncs ? `${stat.syncs} sync${stat.syncs === 1 ? "" : "s"}` : "",
    stat.rebuilds ? `${stat.rebuilds} rebuild${stat.rebuilds === 1 ? "" : "s"}` : "",
    stat.restarts ? `${stat.restarts} restart${stat.restarts === 1 ? "" : "s"}` : "",
  ].filter(Boolean);
  return (
    <span className="text-xs text-muted-foreground">
      {parts.join(" · ")} · last {formatClock(stat.lastAt)}
    </span>
  );
}

function RuleRow({ rule }: { rule: ComposeWatchRule }) {
  const target = ruleTarget(rule);
  return (
    <div className="grid min-w-0 grid-cols-[104px_minmax(0,1fr)_minmax(0,160px)] items-center gap-3">
      <Badge variant="outline" className={cn("justify-self-start font-mono", ACTION_TONE[rule.action])}>
        {rule.action}
      </Badge>
      <span className="flex min-w-0 items-center gap-1.5 font-mono text-xs" title={rule.path}>
        <span className="truncate">{rule.displayPath || rule.path}</span>
        {SYNC_ACTIONS.has(rule.action) && (
          <>
            <ArrowRight className="size-3 shrink-0 text-muted-foreground" />
            <span className="truncate">{target}</span>
          </>
        )}
      </span>
      <Ignore patterns={rule.ignore ?? []} />
    </div>
  );
}

/** Rules grouped per service, with what the CLI reported for the service so far. */
export function WatchRulesTable({
  services,
  stats,
  limitedTo,
}: {
  services: readonly ComposeWatchService[];
  stats: readonly ComposeWatchStat[];
  limitedTo: readonly string[];
}) {
  return (
    <div className="rounded-[10px] border border-border bg-card">
      <div className="grid grid-cols-[168px_minmax(0,1fr)] gap-3 border-b border-border px-3 py-1.5 text-xs text-muted-foreground">
        <span>Service</span>
        <div className="grid grid-cols-[104px_minmax(0,1fr)_minmax(0,160px)] gap-3">
          <span>Action</span>
          <span>Path → target</span>
          <span>Ignore</span>
        </div>
      </div>
      {services.map((s) => {
        const watched = limitedTo.length === 0 || limitedTo.includes(s.name);
        return (
          <div key={s.name} className={cn("grid grid-cols-[168px_minmax(0,1fr)] gap-3 border-b border-border/60 px-3 py-2 last:border-b-0", !watched && "opacity-60")}>
            <div className="min-w-0">
              <div className="truncate font-medium">{s.name}</div>
              {watched ? <Counters stat={stats.find((x) => x.service === s.name)} /> : <span className="text-xs text-muted-foreground">Not watched</span>}
            </div>
            <div className="min-w-0 space-y-1.5">
              {(s.rules ?? []).map((r, i) => (
                <RuleRow key={i} rule={r} />
              ))}
            </div>
          </div>
        );
      })}
    </div>
  );
}
