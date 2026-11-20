import { useState } from "react";
import { Copy } from "lucide-react";
import { OperationStatus, type Operation } from "@bindings/model";
import { Button } from "@/components/ui/button";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { useOperationLogs } from "@/hooks/useOperations";
import { copyToClipboard } from "@/lib/clipboard";
import { RegistryAuthOffer } from "@/components/registries/RegistryAuthOffer";
import { BuildProgressView } from "@/components/images/build/BuildProgressView";
import { StatusIcon } from "./OperationStatusIcon";
import { OperationLog } from "./OperationLog";
import { Elapsed, ProgressBar } from "./operationParts";

function time(at: number): string {
  const d = new Date(at);
  const today = d.toDateString() === new Date().toDateString();
  return today ? d.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" }) : d.toLocaleString([], { dateStyle: "medium", timeStyle: "short" });
}

/** Log (and, for image builds, the build steps) of an operation of this session. Mount it with key={op.id}. */
function Output({ op }: { op: Operation }) {
  const { lines } = useOperationLogs(op.id, true);
  const isBuild = op.kind === "image.build";
  const [view, setView] = useState<"logs" | "steps">("logs");
  return (
    <div className="flex min-h-0 flex-1 flex-col gap-2">
      <div className="flex h-8 shrink-0 items-center justify-between gap-2">
        {isBuild ? (
          <Tabs value={view} onValueChange={(v) => setView(v as "logs" | "steps")}>
            <TabsList variant="line" className="h-8 p-0">
              <TabsTrigger value="logs">Logs</TabsTrigger>
              <TabsTrigger value="steps">Steps</TabsTrigger>
            </TabsList>
          </Tabs>
        ) : (
          <h3 className="text-sm font-medium">Logs</h3>
        )}
        {view === "logs" && (
          <Button variant="ghost" size="sm" disabled={lines.length === 0} onClick={() => copyToClipboard(lines.join("\n"), "Log")}>
            <Copy />
            Copy
          </Button>
        )}
      </div>
      {view === "steps" ? (
        <div className="min-h-0 flex-1 overflow-auto">
          <BuildProgressView op={op} />
        </div>
      ) : (
        <OperationLog lines={lines} />
      )}
    </div>
  );
}

interface Props {
  op: Operation;
  /** Cleared or from an earlier session: only its record is kept, not its log. */
  past: boolean;
  connectionName: string;
  onCancel: () => void;
}

/** Right side of the Operations panel: one operation with its state and output. */
export function OperationDetail({ op, past, connectionName, onCancel }: Props) {
  const running = op.status === OperationStatus.OpRunning;
  const failed = op.status === OperationStatus.OpFailed;
  return (
    <div className="flex h-full min-w-0 flex-col gap-3 p-4">
      <div className="shrink-0">
        <div className="flex items-start gap-2">
          <span className="pt-1">
            <StatusIcon status={op.status} />
          </span>
          <h2 className="min-w-0 flex-1 break-words text-[15px] font-semibold leading-6">{op.title}</h2>
          {running && op.cancellable && (
            <Button size="sm" variant="outline" onClick={onCancel}>
              Cancel
            </Button>
          )}
        </div>
        <div className="mt-1 flex flex-wrap items-center gap-x-2 gap-y-0.5 pl-6 text-xs text-muted-foreground">
          <span className="truncate">{connectionName}</span>
          <span aria-hidden>·</span>
          <span>Started {time(op.startedAt)}</span>
          <span aria-hidden>·</span>
          <Elapsed op={op} />
          {op.status === OperationStatus.OpCancelled && (
            <>
              <span aria-hidden>·</span>
              <span>Cancelled</span>
            </>
          )}
          {running && op.progress != null && (
            <>
              <span aria-hidden>·</span>
              <span className="tabular-nums">{Math.round(op.progress * 100)}%</span>
            </>
          )}
        </div>
        {running && (
          <div className="mt-2.5 pl-6">
            <ProgressBar op={op} />
          </div>
        )}
        {failed && op.error ? (
          <p className="mt-2 break-words pl-6 text-[13px] text-destructive">{op.error}</p>
        ) : (
          op.detail && <p className="mt-2 break-words pl-6 text-[13px] text-muted-foreground">{op.detail}</p>
        )}
        <div className="pl-6">
          <RegistryAuthOffer op={op} />
        </div>
      </div>
      {past ? (
        <p className="rounded-md border border-dashed border-border px-3 py-2.5 text-[13px] text-muted-foreground">
          No log is kept for this operation. Logs last until an operation is cleared or Docklet restarts.
        </p>
      ) : (
        <Output key={op.id} op={op} />
      )}
    </div>
  );
}
