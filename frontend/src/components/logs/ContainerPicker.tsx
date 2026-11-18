import { useMemo, useState } from "react";
import { AlertTriangle, ChevronDown, ChevronRight, PanelLeftClose, Search } from "lucide-react";
import type { Container } from "@bindings/model";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import { Switch } from "@/components/ui/switch";
import { cn } from "@/lib/utils";
import { groupSources } from "@/lib/logSources";
import { containerTone } from "./logColours";
import type { StreamInfo } from "./useMergedLogStreams";

interface Props {
  containers: readonly Container[];
  ticked: ReadonlySet<string>;
  onSet: (ids: readonly string[], on: boolean) => void;
  onClear: () => void;
  infos: Record<string, StreamInfo>;
  /** Set when the default selection left running containers out. */
  capped: { shown: number; total: number } | null;
  onCollapse: () => void;
}

function Row({ c, on, info, onSet }: { c: Container; on: boolean; info?: StreamInfo; onSet: Props["onSet"] }) {
  const running = c.state === "running";
  return (
    <label className="flex h-7 cursor-pointer items-center gap-2 rounded-md pr-2 pl-7 text-[13px] hover:bg-muted/60">
      <Checkbox checked={on} onCheckedChange={(v) => onSet([c.id], v === true)} aria-label={`Show logs of ${c.name}`} />
      <span aria-hidden className={cn("size-2 shrink-0 rounded-full", containerTone(c.name).dot)} />
      <span className={cn("min-w-0 flex-1 truncate", !running && "text-muted-foreground")} title={c.name}>
        {c.composeService || c.name}
      </span>
      {info?.status === "error" ? (
        <span className="shrink-0" title={`Log stream failed: ${info.error ?? "unknown error"}`}>
          <AlertTriangle className="size-3.5 text-destructive" aria-label="Log stream failed" />
        </span>
      ) : on && info?.status === "ended" ? (
        <span className="shrink-0 text-[11px] text-muted-foreground" title="The log stream ended because the container stopped. It is followed again when the container starts.">ended</span>
      ) : !running ? (
        <span className="shrink-0 text-[11px] text-muted-foreground">{c.state}</span>
      ) : null}
    </label>
  );
}

/** Left panel of the Logs screen: containers grouped by Compose project with tri-state project boxes. */
export function ContainerPicker({ containers, ticked, onSet, onClear, infos, capped, onCollapse }: Props) {
  const [search, setSearch] = useState("");
  const [runningOnly, setRunningOnly] = useState(true);
  const [folded, setFolded] = useState<ReadonlySet<string>>(new Set());
  const groups = useMemo(() => groupSources(containers, { search, runningOnly, keep: ticked }), [containers, search, runningOnly, ticked]);
  const toggleFold = (key: string) =>
    setFolded((p) => {
      const n = new Set(p);
      if (n.has(key)) n.delete(key);
      else n.add(key);
      return n;
    });

  return (
    <div className="flex h-full min-h-0 w-[260px] shrink-0 flex-col rounded-lg border border-border bg-card">
      <div className="flex items-center gap-1.5 border-b border-border p-2">
        <div className="relative min-w-0 flex-1">
          <Search className="pointer-events-none absolute top-1/2 left-2 size-3.5 -translate-y-1/2 text-muted-foreground" />
          <Input value={search} onChange={(e) => setSearch(e.target.value)} placeholder="Search containers" aria-label="Search containers" className="h-8 pl-7 text-[13px]" />
        </div>
        <Button variant="ghost" size="icon-sm" className="size-8 shrink-0" aria-label="Hide container list" title="Hide container list" onClick={onCollapse}>
          <PanelLeftClose className="size-4" />
        </Button>
      </div>
      <div className="flex items-center justify-between gap-2 border-b border-border px-3 py-1.5 text-xs text-muted-foreground">
        <label className="flex cursor-pointer items-center gap-2">
          <Switch checked={runningOnly} onCheckedChange={setRunningOnly} aria-label="Running only" size="sm" />
          Running only
        </label>
        <span className="tabular-nums">{containers.length} total</span>
      </div>

      <div className="min-h-0 flex-1 overflow-y-auto p-1.5">
        {containers.length === 0 && <p className="p-3 text-xs text-muted-foreground">This engine has no containers.</p>}
        {containers.length > 0 && groups.length === 0 && <p className="p-3 text-xs text-muted-foreground">No containers match{runningOnly ? " (running only is on)" : ""}.</p>}
        {groups.map((g) => {
          const key = g.project ?? "__standalone__";
          const ids = g.containers.map((c) => c.id);
          const n = ids.filter((id) => ticked.has(id)).length;
          const state = n === 0 ? false : n === ids.length ? true : "indeterminate";
          const open = !folded.has(key) || search !== "";
          return (
            <div key={key} className="mb-0.5">
              <div className="flex h-7 items-center gap-1 rounded-md pr-2 pl-1 hover:bg-muted/60">
                <button type="button" aria-label={open ? "Collapse group" : "Expand group"} onClick={() => toggleFold(key)} className="flex size-5 shrink-0 items-center justify-center rounded text-muted-foreground hover:text-foreground">
                  {open ? <ChevronDown className="size-3.5" /> : <ChevronRight className="size-3.5" />}
                </button>
                <label className="flex min-w-0 flex-1 cursor-pointer items-center gap-2 text-[13px] font-medium">
                  <Checkbox checked={state} onCheckedChange={() => onSet(ids, n !== ids.length)} aria-label={`Show logs of all containers in ${g.project ?? "standalone"}`} />
                  <span className="truncate">{g.project ?? "Standalone"}</span>
                </label>
                <span className="shrink-0 text-xs text-muted-foreground tabular-nums">
                  {n}/{ids.length}
                </span>
              </div>
              {open && g.containers.map((c) => <Row key={c.id} c={c} on={ticked.has(c.id)} info={infos[c.id]} onSet={onSet} />)}
            </div>
          );
        })}
      </div>

      <div className="space-y-1 border-t border-border px-3 py-2 text-xs text-muted-foreground">
        <div className="flex items-center justify-between">
          <span>{ticked.size} selected</span>
          <Button variant="ghost" size="xs" disabled={ticked.size === 0} onClick={onClear}>
            Clear
          </Button>
        </div>
        {capped && (
          <p className="text-warning">
            Started with {capped.shown} of {capped.total} running containers to stay light. Tick more to add them.
          </p>
        )}
      </div>
    </div>
  );
}
