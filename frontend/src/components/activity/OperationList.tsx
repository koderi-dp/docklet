import type { ReactNode } from "react";
import { OperationStatus, type Operation } from "@bindings/model";
import { cn } from "@/lib/utils";
import { StatusIcon } from "./OperationStatusIcon";
import { Elapsed, ProgressBar } from "./operationParts";

interface Props {
  live: Operation[];
  earlier: Operation[];
  earlierLoading: boolean;
  earlierError: string | null;
  selectedId: string | null;
  onSelect: (id: string) => void;
  nameOf: (connectionId: string) => string;
}

function Row({ op, past, selected, onSelect, nameOf }: { op: Operation; past?: boolean; selected: boolean; onSelect: () => void; nameOf: Props["nameOf"] }) {
  return (
    <li>
      <button
        type="button"
        onClick={onSelect}
        aria-current={selected ? "true" : undefined}
        className={cn(
          "flex w-full items-start gap-2 px-3 py-2 text-left transition-colors duration-100 hover:bg-accent/60 focus-visible:ring-2 focus-visible:ring-ring/60 focus-visible:outline-none focus-visible:ring-inset",
          selected && "bg-accent hover:bg-accent",
        )}
      >
        <span className="pt-0.5">
          <StatusIcon status={op.status} />
        </span>
        <span className="min-w-0 flex-1">
          <span className={cn("block truncate text-[13px]", !past && "font-medium")} title={op.title}>
            {op.title}
          </span>
          <span className="mt-0.5 flex items-center gap-1.5 text-xs text-muted-foreground">
            <span className="truncate">{nameOf(op.connectionId)}</span>
            <span aria-hidden>·</span>
            <span className="shrink-0">
              {past ? (op.finishedAt ? new Date(op.finishedAt).toLocaleDateString() : "—") : <Elapsed op={op} />}
            </span>
          </span>
          {op.status === OperationStatus.OpRunning && (
            <span className="mt-1.5 block">
              <ProgressBar op={op} />
            </span>
          )}
        </span>
      </button>
    </li>
  );
}

function Group({ label, children }: { label: string; children: ReactNode }) {
  return (
    <section>
      <h3 className="px-3 pt-3 pb-1 text-xs font-medium text-muted-foreground">{label}</h3>
      {children}
    </section>
  );
}

/** Left column of the Operations panel: running, finished in this session, and earlier operations. */
export function OperationList({ live, earlier, earlierLoading, earlierError, selectedId, onSelect, nameOf }: Props) {
  const running = live.filter((o) => o.status === OperationStatus.OpRunning);
  const finished = live.filter((o) => o.status !== OperationStatus.OpRunning);
  const rows = (ops: Operation[], past?: boolean) => (
    <ul>
      {ops.map((op) => (
        <Row key={op.id} op={op} past={past} selected={op.id === selectedId} onSelect={() => onSelect(op.id)} nameOf={nameOf} />
      ))}
    </ul>
  );
  return (
    <nav aria-label="Operations" className="h-full overflow-auto pb-3">
      {running.length > 0 && <Group label="Running">{rows(running)}</Group>}
      {finished.length > 0 && <Group label="Finished">{rows(finished)}</Group>}
      {(earlier.length > 0 || earlierLoading || earlierError) && (
        <Group label="Earlier">
          {earlierError ? (
            <p className="px-3 py-1 text-xs text-destructive">Could not load earlier operations: {earlierError}</p>
          ) : earlier.length === 0 ? (
            <p className="px-3 py-1 text-xs text-muted-foreground">Loading…</p>
          ) : (
            rows(earlier, true)
          )}
        </Group>
      )}
    </nav>
  );
}
