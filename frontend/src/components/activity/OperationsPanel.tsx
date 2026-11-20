import { useMemo } from "react";
import { Button } from "@/components/ui/button";
import { Sheet, SheetClose, SheetContent, SheetDescription, SheetHeader, SheetTitle } from "@/components/ui/sheet";
import { useConnections } from "@/hooks/useActiveConnection";
import { useOperations, useOpsStore, usePastOperations } from "@/hooks/useOperations";
import { OperationDetail } from "./OperationDetail";
import { OperationList } from "./OperationList";
import { pickSelected } from "./operationSelection";

/** The Operations side panel: every background operation on the left, the selected one with its log on the right. */
export function OperationsPanel() {
  const { operations, finishedCount, loaded, loadError, cancel, clearFinished } = useOperations();
  const open = useOpsStore((s) => s.panelOpen);
  const closePanel = useOpsStore((s) => s.closePanel);
  const selectedId = useOpsStore((s) => s.selectedId);
  const select = useOpsStore((s) => s.select);
  const { data: conns } = useConnections();
  const past = usePastOperations(open, 50);
  const earlier = useMemo(() => {
    const live = new Set(operations.map((o) => o.id));
    return (past.data ?? []).filter((o) => !live.has(o.id));
  }, [operations, past.data]);
  const nameOf = (id: string) => conns?.find((c) => c.id === id)?.name ?? id;
  const selected = pickSelected(selectedId, operations, earlier);
  const empty = operations.length === 0 && earlier.length === 0;

  return (
    <Sheet open={open} onOpenChange={(o) => !o && closePanel()}>
      <SheetContent
        className="w-[min(960px,calc(100vw-4rem))]"
        tabIndex={-1}
        onOpenAutoFocus={(e) => {
          // Focus the panel itself, not its first button ("Clear finished").
          e.preventDefault();
          (e.currentTarget as HTMLElement).focus();
        }}
      >
        <SheetHeader>
          <SheetTitle>Operations</SheetTitle>
          <SheetDescription className="sr-only">Background operations and their logs.</SheetDescription>
          <Button size="sm" variant="ghost" disabled={finishedCount === 0} onClick={() => void clearFinished()}>
            Clear finished
          </Button>
          <SheetClose />
        </SheetHeader>
        {loadError ? (
          <p className="p-4 text-sm text-destructive">Could not load operations: {loadError}</p>
        ) : !loaded ? (
          <p className="p-4 text-sm text-muted-foreground">Loading…</p>
        ) : empty && !past.isLoading && !past.error ? (
          <div className="flex flex-1 items-center justify-center p-8">
            <p className="max-w-sm text-center text-sm text-muted-foreground">
              No background operations. Pulls, builds, Compose runs, prunes and other long tasks appear here.
            </p>
          </div>
        ) : (
          <div className="flex min-h-0 flex-1">
            <div className="w-64 shrink-0 border-r border-border">
              <OperationList
                live={operations}
                earlier={earlier}
                earlierLoading={past.isLoading}
                earlierError={past.error ? String(past.error) : null}
                selectedId={selected?.id ?? null}
                onSelect={select}
                nameOf={nameOf}
              />
            </div>
            <div className="min-w-0 flex-1">
              {selected && (
                <OperationDetail
                  op={selected}
                  past={!operations.some((o) => o.id === selected.id)}
                  connectionName={nameOf(selected.connectionId)}
                  onCancel={() => void cancel(selected.id)}
                />
              )}
            </div>
          </div>
        )}
      </SheetContent>
    </Sheet>
  );
}
