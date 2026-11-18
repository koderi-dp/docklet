import { Eye, RotateCw, TriangleAlert, X } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from "@/components/ui/collapsible";
import { useNow, useWatchActions, useWatchInfo, useWatchState } from "@/hooks/useComposeWatch";
import type { ComposeProject } from "@bindings/model";
import { WatchActivity } from "./WatchActivity";
import { WatchChip } from "./WatchHeaderControl";
import { WatchRulesTable } from "./WatchRulesTable";

/** Rules, per-service counters, parsed activity and raw CLI output of the project's watch. */
export function WatchTab({ connectionId, project }: { connectionId: string; project: ComposeProject }) {
  const state = useWatchState(connectionId, project.name);
  const info = useWatchInfo(connectionId, project.name, project.capability.canRun);
  const actions = useWatchActions(connectionId, project.name);
  const st = state.data;
  const now = useNow(st?.running === true);
  if (!st?.active) return <p className="text-muted-foreground">No watch is running for this project.</p>;
  const raw = st.raw ?? [];
  const limited = (st.services ?? []).length > 0;

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-muted-foreground">
        {st.running ? <WatchChip ready={st.ready} startedAt={st.startedAt ?? now} now={now} /> : <span className="font-medium text-destructive">Watch stopped</span>}
        <span>{(st.running ? "Watching " : "Was watching ") + (limited ? (st.services ?? []).join(", ") : "every service with rules")}</span>
        {st.noUp && <span className="font-mono">--no-up</span>}
        <span>Started {new Date(st.startedAt ?? 0).toLocaleTimeString(undefined, { hour12: false })}</span>
      </div>

      {!st.running && (
        <div className="rounded-[10px] border border-destructive/40 bg-destructive/5 p-3" role="alert">
          <div className="flex items-center gap-2 font-medium text-destructive">
            <TriangleAlert className="size-4" /> The watch process ended
          </div>
          <pre className="mt-1.5 whitespace-pre-wrap break-words font-mono text-xs leading-5">{st.exit || "No reason was reported."}</pre>
          <div className="mt-2 flex gap-2">
            <Button
              size="sm"
              disabled={actions.busy || !info.data?.available}
              title={info.data?.available ? undefined : info.data?.reason}
              onClick={() => void actions.startWatch({ services: st.services ?? [], noUp: st.noUp })}
            >
              <RotateCw className="size-3.5" /> Start again
            </Button>
            <Button size="sm" variant="outline" onClick={() => void actions.dismiss()}>
              <X className="size-3.5" /> Dismiss
            </Button>
          </div>
        </div>
      )}

      <section className="space-y-1.5">
        <h3 className="text-sm font-semibold">Rules</h3>
        {info.data ? (
          <WatchRulesTable services={info.data.services ?? []} stats={st.stats ?? []} limitedTo={st.services ?? []} />
        ) : (
          <p className="text-muted-foreground">{info.error ? (info.error as Error).message : "Reading the rules…"}</p>
        )}
      </section>

      <section className="space-y-1.5">
        <h3 className="flex items-baseline gap-2 text-sm font-semibold">
          Activity <span className="text-xs font-normal text-muted-foreground">newest first, last 500 reported by the CLI</span>
        </h3>
        <WatchActivity events={st.events ?? []} running={st.running} />
      </section>

      <Collapsible>
        <CollapsibleTrigger className="group flex items-center gap-1.5 text-xs text-muted-foreground hover:text-foreground">
          <Eye className="size-3.5" /> Raw output ({raw.length} line{raw.length === 1 ? "" : "s"} not shown above)
        </CollapsibleTrigger>
        <CollapsibleContent>
          <pre className="mt-1.5 max-h-72 overflow-auto whitespace-pre-wrap break-all rounded-[10px] border border-border bg-card p-3 font-mono text-xs leading-5">
            {raw.length ? raw.join("\n") : "Nothing else was printed."}
          </pre>
        </CollapsibleContent>
      </Collapsible>
    </div>
  );
}
