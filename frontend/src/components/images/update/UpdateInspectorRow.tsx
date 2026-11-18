import type { ImageRow } from "@bindings/model";
import { Button } from "@/components/ui/button";
import { PODMAN_NO_UPDATE_CHECK, useIsPodman } from "@/lib/engineKind";
import { formatDateTime } from "@/components/containers/format";
import { checkImageUpdate, openUpdateDialog, useUpdateStatus } from "./updateStore";
import { UpdateChip } from "./UpdateChip";

const shortDigest = (d: string) => d.replace(/^sha256:/, "").slice(0, 12);

/** "Update" row of the image inspector: state, digests, check and update actions. */
export function UpdateInspectorRow({ row }: { row: ImageRow }) {
  const { status, checking } = useUpdateStatus(row.connectionId, row.ref);
  const podman = useIsPodman(row.connectionId);
  if (row.dangling) return null;
  return (
    <div className="grid grid-cols-[96px_1fr] gap-3 border-b border-border/50 py-1.5 text-[13px]">
      <dt className="text-muted-foreground">Update</dt>
      <dd className="min-w-0 space-y-1.5">
        <div className="flex flex-wrap items-center gap-2">
          {status || checking ? <UpdateChip row={row} /> : <span className="text-xs text-muted-foreground">Not checked</span>}
          {status && !status.error && !checking && (
            <span className="text-xs text-muted-foreground">checked {formatDateTime(new Date(status.checkedAt))}</span>
          )}
        </div>
        {status?.error && <p className="text-xs text-muted-foreground">{status.error}</p>}
        {status && !status.error && status.remoteDigest && (
          <p className="font-mono text-xs text-muted-foreground">
            local {shortDigest(status.localDigest ?? "") || "—"} · registry {shortDigest(status.remoteDigest)}
          </p>
        )}
        <div className="flex gap-1.5">
          <Button size="sm" variant="outline" disabled={checking || podman} title={podman ? PODMAN_NO_UPDATE_CHECK : undefined} onClick={() => void checkImageUpdate(row)}>
            Check for update
          </Button>
          {status?.updateAvailable && (
            <Button size="sm" onClick={() => openUpdateDialog(row)}>
              Update…
            </Button>
          )}
        </div>
      </dd>
    </div>
  );
}
