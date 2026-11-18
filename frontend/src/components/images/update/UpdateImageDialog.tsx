import { useState } from "react";
import { Link } from "react-router-dom";
import { useQueries } from "@tanstack/react-query";
import { CheckCircle2, Loader2, XCircle } from "lucide-react";
import { ContainerService } from "@bindings/services";
import type { ContainerRecreateInfo } from "@bindings/model";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Skeleton } from "@/components/ui/skeleton";
import { StatusBadge } from "@/components/shared/StatusBadge";
import { recreateInfoKey } from "@/components/containers/recreate/useRecreate";
import { useImageDetail } from "@/hooks/useImages";
import { errorMessage } from "@/lib/errors";
import { sameImageRef } from "./refMatch";
import { useUpdateRun, type RunItem } from "./useUpdateRun";

interface Props {
  connectionId: string;
  imageRef: string;
  onClose: () => void;
}

type Kind = "recreate" | "compose" | "othertag";

function classify(info: ContainerRecreateInfo, ref: string): Kind {
  if (info.composeManaged) return "compose";
  return sameImageRef(info.spec.image, ref) ? "recreate" : "othertag";
}

/** "Update…": pull the new image, then recreate the standalone containers that use it. */
export function UpdateImageDialog({ connectionId, imageRef, onClose }: Props) {
  const detail = useImageDetail(connectionId, imageRef);
  const refs = detail.data?.containers ?? [];
  const infos = useQueries({
    queries: refs.map((c) => ({
      queryKey: recreateInfoKey(connectionId, c.id),
      queryFn: () => ContainerService.RecreateSpec(connectionId, c.id),
      gcTime: 0,
      staleTime: 0,
      retry: false,
    })),
  });
  const loading = detail.isLoading || infos.some((q) => q.isLoading);
  const [unticked, setUnticked] = useState<Set<string>>(new Set());
  const { phase, pullError, items, run } = useUpdateRun(connectionId, imageRef);

  const rows = infos.flatMap((q, i) => (q.data ? [{ info: q.data, kind: classify(q.data, imageRef), state: refs[i].state }] : []));
  const chosen = rows.filter((r) => r.kind === "recreate" && !unticked.has(r.info.id)).map((r) => r.info);
  const busy = phase === "pulling" || phase === "recreating";
  const toggle = (id: string) =>
    setUnticked((s) => {
      const n = new Set(s);
      if (!n.delete(id)) n.add(id);
      return n;
    });

  return (
    <Dialog open onOpenChange={(o) => !o && !busy && onClose()}>
      <DialogContent className="flex max-h-[88vh] flex-col gap-3 sm:max-w-xl">
        <DialogHeader>
          <DialogTitle>Update {imageRef}</DialogTitle>
          <DialogDescription>
            Pulls the newest image for this tag, then replaces the containers that use it. Their volumes are kept, and each one is restored if it cannot start.
          </DialogDescription>
        </DialogHeader>

        <div className="min-h-0 flex-1 space-y-2 overflow-y-auto text-[13px]">
          {loading && <Skeleton className="h-24 w-full" />}
          {detail.error && <p className="text-destructive">Could not list containers: {errorMessage(detail.error)}</p>}
          {!loading && rows.length === 0 && <p className="text-muted-foreground">No container uses this image. Only the image is pulled.</p>}
          {rows.map(({ info, kind, state }) => (
            <ContainerRow key={info.id} info={info} kind={kind} state={state} ticked={!unticked.has(info.id)} disabled={busy || phase === "done"} onToggle={() => toggle(info.id)} item={items.find((i) => i.id === info.id)} />
          ))}
          {pullError && (
            <p role="alert" className="rounded-md border border-destructive/40 bg-destructive/10 px-3 py-2 text-destructive">
              Pull failed: {pullError}
            </p>
          )}
          {phase === "done" && !pullError && <p className="text-muted-foreground">Finished. The image status is being rechecked.</p>}
        </div>

        <DialogFooter>
          <Button variant="outline" size="sm" onClick={onClose} disabled={busy}>
            {phase === "done" ? "Close" : "Cancel"}
          </Button>
          {phase !== "done" && (
            <Button size="sm" disabled={busy || loading} onClick={() => void run(chosen)}>
              {busy && <Loader2 className="size-4 animate-spin" />}
              {phase === "pulling" ? "Pulling…" : phase === "recreating" ? "Recreating…" : chosen.length > 0 ? `Pull and recreate ${chosen.length}` : "Pull only"}
            </Button>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

function ContainerRow(p: {
  info: ContainerRecreateInfo;
  kind: Kind;
  state: string;
  ticked: boolean;
  disabled: boolean;
  onToggle: () => void;
  item?: RunItem;
}) {
  const { info, kind } = p;
  return (
    <div className="rounded-md border border-border px-3 py-2">
      <div className="flex items-center gap-2">
        {kind === "recreate" && <Checkbox checked={p.ticked} disabled={p.disabled} onCheckedChange={() => p.onToggle()} aria-label={`Recreate ${info.name}`} />}
        <span className="min-w-0 flex-1 truncate font-medium">{info.name}</span>
        <StatusBadge state={p.state} />
        {p.item && <ItemState item={p.item} />}
      </div>
      {kind === "compose" && (
        <p className="mt-1 text-xs text-muted-foreground">
          Managed by Compose project{" "}
          <Link to={`/compose/${encodeURIComponent(info.composeProject ?? "")}`} className="text-primary hover:underline">
            {info.composeProject}
          </Link>
          : not replaced here. After the pull, run Up for service <span className="font-mono">{info.composeService}</span> in that project to use the new image.
        </p>
      )}
      {kind === "othertag" && <p className="mt-1 text-xs text-muted-foreground">Created from <span className="font-mono">{info.spec.image}</span>, a different tag of the same image: not updated.</p>}
      {kind === "recreate" && (info.anonymousVolumes ?? []).length > 0 && (
        <p className="mt-1 text-xs text-warning">Has anonymous volumes that are not carried over ({(info.anonymousVolumes ?? []).map((v) => v.target).join(", ")}).</p>
      )}
      {p.item?.error && <p className="mt-1 break-words text-xs text-destructive">{p.item.error}</p>}
    </div>
  );
}

function ItemState({ item }: { item: RunItem }) {
  if (item.status === "running") return <Loader2 className="size-4 animate-spin text-muted-foreground" aria-label="Recreating" />;
  if (item.status === "done") return <CheckCircle2 className="size-4 text-success" aria-label="Recreated" />;
  if (item.status === "failed") return <XCircle className="size-4 text-destructive" aria-label="Failed" />;
  return <span className="text-xs text-muted-foreground">waiting</span>;
}
