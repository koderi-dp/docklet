import type { ReactNode } from "react";
import { Copy, Hammer, Layers, Trash2 } from "lucide-react";
import { OperationStatus, type BuildRecord, type Operation } from "@bindings/model";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { formatDateTime } from "@/components/containers/format";
import { copyToClipboard } from "@/lib/clipboard";
import { BuildProgressView } from "./BuildProgressView";
import { BuildResultBadge, builderLabel } from "./BuildResultBadge";
import { BuildStepList } from "./BuildStepList";
import { formatDuration } from "./buildFormat";

function Item({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="grid grid-cols-[110px_1fr] gap-2 py-0.5 text-[13px]">
      <dt className="text-muted-foreground">{label}</dt>
      <dd className="min-w-0 break-words">{children}</dd>
    </div>
  );
}

const mono = "font-mono text-xs";

function Options({ r }: { r: BuildRecord }) {
  const flags = [r.noCache && "no cache", r.pull && "always pull base images"].filter(Boolean).join(", ");
  const args = r.argNames ?? [];
  return (
    <dl>
      <Item label="Builder">{builderLabel(r.builder, r.method)}</Item>
      <Item label="Context">
        <span className={mono}>{r.contextDir}</span>
      </Item>
      <Item label="Dockerfile">
        <span className={mono}>{r.dockerfile}</span>
      </Item>
      <Item label="Tags">{r.tags?.length ? <span className={mono}>{r.tags.join(", ")}</span> : <span className="text-muted-foreground">None</span>}</Item>
      {r.target && (
        <Item label="Target stage">
          <span className={mono}>{r.target}</span>
        </Item>
      )}
      <Item label="Platform">{r.platform ? <span className={mono}>{r.platform}</span> : <span className="text-muted-foreground">Engine default</span>}</Item>
      {flags && <Item label="Flags">{flags}</Item>}
      <Item label="Build arguments">
        {args.length ? (
          <span>
            <span className={mono}>{args.join(", ")}</span> <span className="text-muted-foreground">(names only, values are never stored)</span>
          </span>
        ) : (
          <span className="text-muted-foreground">None</span>
        )}
      </Item>
      <Item label="Image ID">{r.imageId ? <span className={mono}>{r.imageId.replace(/^sha256:/, "").slice(0, 12)}</span> : <span className="text-muted-foreground">Not available</span>}</Item>
    </dl>
  );
}

interface Props {
  /** A finished build from the history. */
  record: BuildRecord | null;
  /** A build running now (or just finished) in this session. */
  live: Operation | null;
  imageExists: boolean;
  onClose: () => void;
  onBuildAgain: (r: BuildRecord) => void;
  onShowImage: (r: BuildRecord) => void;
  onRemove: (r: BuildRecord) => void;
  onCancel: (op: Operation) => void;
}

/** Details of one build: steps, output of a failed step, options used and the equivalent command. */
export function BuildDetailsDialog({ record, live, imageExists, onClose, onBuildAgain, onShowImage, onRemove, onCancel }: Props) {
  const open = !!record || !!live;
  const title = record ? (record.tags?.[0] ?? "Untagged build") : (live?.resource ?? "Build");
  const failedTail = record && record.result === "failed" ? (record.logTail ?? []) : [];
  return (
    <Dialog open={open} onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="max-h-[90vh] overflow-y-auto overflow-x-hidden sm:max-w-2xl [&>*]:min-w-0">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <span className="truncate font-mono text-base">{title}</span>
            {record ? <BuildResultBadge result={record.result} /> : live && <BuildResultBadge result={live.status === OperationStatus.OpRunning ? "running" : live.status === OperationStatus.OpSucceeded ? "success" : live.status === OperationStatus.OpFailed ? "failed" : "cancelled"} />}
          </DialogTitle>
          <DialogDescription>
            {record
              ? `Started ${formatDateTime(new Date(record.startedAt))} · took ${formatDuration(record.durationMs)}`
              : live && `Started ${formatDateTime(new Date(live.startedAt))}`}
          </DialogDescription>
        </DialogHeader>

        {record?.error && <p className="whitespace-pre-wrap break-words rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2 text-xs text-destructive">{record.error}</p>}
        {live && live.status === OperationStatus.OpFailed && live.error && (
          <p className="whitespace-pre-wrap break-words rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2 text-xs text-destructive">{live.error}</p>
        )}

        <section aria-label="Steps">
          <h3 className="mb-1 text-xs font-medium text-muted-foreground">
            Steps
            {record && record.stepsTotal > 0 && ` · ${record.stepsTotal} step${record.stepsTotal === 1 ? "" : "s"}, ${record.stepsCached} cached`}
          </h3>
          {live ? (
            <BuildProgressView op={live} />
          ) : record && record.steps?.length ? (
            <BuildStepList steps={record.steps} live={false} />
          ) : (
            <p className="text-xs text-muted-foreground">
              {record?.builder === "classic" ? "The Classic builder (Engine API) reports no steps." : "No steps were recorded."}
              {record && record.stepsTotal > 0 && ` ${record.stepsTotal} Dockerfile steps, ${record.stepsCached} from cache.`}
            </p>
          )}
        </section>

        {failedTail.length > 0 && (
          <section aria-label="Log tail">
            <h3 className="mb-1 text-xs font-medium text-muted-foreground">Last {failedTail.length} log lines</h3>
            <div tabIndex={0} className="max-h-48 overflow-auto rounded-md border border-border bg-muted/40 p-2 font-mono text-[11px] leading-snug">
              {failedTail.map((l, i) => (
                <div key={i} className="whitespace-pre-wrap break-all">
                  {l}
                </div>
              ))}
            </div>
          </section>
        )}

        {record && (
          <>
            <section aria-label="Options used">
              <h3 className="mb-1 text-xs font-medium text-muted-foreground">Options used</h3>
              <Options r={record} />
            </section>
            <section aria-label="Equivalent command">
              <h3 className="mb-1 text-xs font-medium text-muted-foreground">Equivalent command (argument values replaced by variables)</h3>
              <pre className="overflow-x-auto rounded-md border border-border bg-muted/40 p-2 font-mono text-[11px] leading-snug whitespace-pre-wrap break-all">{record.command}</pre>
            </section>
          </>
        )}

        <DialogFooter className="gap-2 sm:justify-between">
          <div>
            {record && (
              <Button variant="ghost" className="text-destructive hover:bg-destructive/10 hover:text-destructive" onClick={() => onRemove(record)}>
                <Trash2 className="size-4" /> Remove from history
              </Button>
            )}
            {live && live.status === OperationStatus.OpRunning && live.cancellable && (
              <Button variant="outline" onClick={() => onCancel(live)}>
                Cancel build
              </Button>
            )}
          </div>
          {record && (
            <div className="flex flex-wrap justify-end gap-2">
              <Button variant="outline" onClick={() => copyToClipboard(record.command, "Command")}>
                <Copy className="size-4" /> Copy command
              </Button>
              <Tooltip>
                <TooltipTrigger asChild>
                  <span>
                    <Button variant="outline" disabled={!imageExists} onClick={() => onShowImage(record)}>
                      <Layers className="size-4" /> Show image
                    </Button>
                  </span>
                </TooltipTrigger>
                {!imageExists && <TooltipContent>{record.imageId ? "This image no longer exists on the Engine." : "This build left no identifiable image (it was not successful or had no tag)."}</TooltipContent>}
              </Tooltip>
              <Button onClick={() => onBuildAgain(record)}>
                <Hammer className="size-4" /> Build again
              </Button>
            </div>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
