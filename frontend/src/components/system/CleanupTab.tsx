import { useState, type ReactNode } from "react";
import { AlertTriangle, Eraser } from "lucide-react";
import { toast } from "sonner";
import { OperationStatus } from "@bindings/model";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { useOnOperationDone } from "@/hooks/useOperations";
import { useDiskUsage, usePrune, type PruneKind } from "@/hooks/useSystem";
import { ResourceCard } from "@/components/shared/ResourceCard";
import { Bytes, NotAvailable } from "./parts";
import { errorMessage } from "@/lib/errors";

interface CleanupDef {
  id: string;
  title: string;
  description: string;
  warning: string;
  estimate: ReactNode;
  kind: PruneKind;
  /** Optional wider variant enabled by an explicit checkbox in the dialog. */
  variant?: { label: string; kind: PruneKind };
  /** Destructive-to-data: the checkbox must be ticked to run at all. */
  requireAck?: string;
  danger?: boolean;
}

export function CleanupTab({ connectionId }: { connectionId: string }) {
  const du = useDiskUsage(connectionId);
  const prune = usePrune(connectionId);
  const [active, setActive] = useState<CleanupDef | null>(null);
  const [checked, setChecked] = useState(false);

  useOnOperationDone("system.", connectionId, (op) => {
    if (op.status === OperationStatus.OpSucceeded) toast.success(`${op.title} finished`, { description: op.detail });
    else if (op.status === OperationStatus.OpFailed) toast.error(`${op.title} failed`, { description: op.error, duration: Infinity, closeButton: true });
  });

  const cat = (key: string) => du.data?.categories?.find((c) => c.key === key);
  const reclaim = (key: string, reason: string): ReactNode =>
    du.isLoading ? <span className="text-muted-foreground">Loading…</span> : cat(key) ? <Bytes value={cat(key)?.reclaimable} reason="The Engine did not report reclaimable space" /> : <NotAvailable reason={reason} />;

  const defs: CleanupDef[] = [
    {
      id: "containers",
      title: "Stopped containers",
      description: "Removes every stopped container (created, exited or dead).",
      warning: "Their writable layers and logs are deleted. Volumes are kept.",
      estimate: reclaim("containers", "Docker disk usage is not available"),
      kind: "containers",
    },
    {
      id: "images",
      title: "Dangling images",
      description: "Removes untagged images that no other image depends on.",
      warning: "Tagged images are kept unless you also tick the option to remove all unused images.",
      estimate: <NotAvailable reason="The Engine reports reclaimable space for all unused images, not for dangling ones alone. The figure appears on the Disk Usage tab as Images / Reclaimable." />,
      kind: "images",
      variant: { label: "Also remove ALL images not used by any container (tagged images included)", kind: "images-all" },
    },
    {
      id: "networks",
      title: "Unused networks",
      description: "Removes networks no container uses. Predefined networks are never removed.",
      warning: "Containers that are stopped but configured for a removed network will fail to start until it is recreated.",
      estimate: <NotAvailable reason="Networks occupy no measurable disk space; the Engine reports no estimate." />,
      kind: "networks",
    },
    {
      id: "volumes",
      title: "Unused volumes",
      description: "Removes every volume no container references, including named volumes.",
      warning: "Data in these volumes is deleted permanently and cannot be recovered.",
      estimate: reclaim("volumes", "Docker disk usage is not available"),
      kind: "volumes",
      requireAck: "I understand that this permanently deletes volume data",
      danger: true,
    },
    {
      id: "buildcache",
      title: "Build cache",
      description: "Removes build cache records that are not in use.",
      warning: "The next image build may be slower because layers must be rebuilt.",
      estimate: reclaim("buildCache", "Docker disk usage is not available"),
      kind: "buildcache",
    },
    {
      id: "system",
      title: "System prune",
      description: "Stopped containers, unused networks and dangling images in one operation.",
      warning: "Volumes are NOT included unless you tick the option below.",
      estimate: <NotAvailable reason="A combined estimate is not reported by the Engine. See the individual cards." />,
      kind: "system",
      variant: { label: "Also delete ALL unused volumes (permanently deletes their data)", kind: "system-volumes" },
    },
  ];

  const open = (d: CleanupDef) => {
    setChecked(false);
    setActive(d);
  };
  const resolvedKind = (d: CleanupDef): PruneKind => (d.variant && checked ? d.variant.kind : d.kind);
  const canRun = active ? !(active.requireAck && !checked) : false;

  const run = async () => {
    if (!active) return;
    const kind = resolvedKind(active);
    setActive(null);
    try {
      await prune.mutateAsync(kind);
      toast.info(`${active.title}: cleanup started`, { description: "Progress and the result are shown in the Activity Center." });
    } catch (e) {
      toast.error(`Could not start ${active.title.toLowerCase()} cleanup`, {
        description: errorMessage(e),
        duration: Infinity,
        closeButton: true,
      });
    }
  };

  return (
    <div>
      <p className="mb-3 text-[13px] text-muted-foreground">
        Cleanup acts on the Docker Engine of this connection only. Estimates come from Docker disk usage (docker system df), not from the host disk.
      </p>
      <div className="grid grid-cols-1 gap-3 lg:grid-cols-2 xl:grid-cols-3 [&>*]:min-h-[148px]">
        {defs.map((d) => (
          <ResourceCard
            key={d.id}
            icon={d.danger ? <AlertTriangle className="text-destructive" /> : <Eraser className="text-muted-foreground" />}
            title={d.title}
            description={d.description}
            note={d.warning}
            danger={d.danger}
            footer={
              <>
                <span className="min-w-0 truncate">
                  <span className="text-muted-foreground">Reclaimable: </span>
                  {d.estimate}
                </span>
                <Button
                  size="sm"
                  variant="outline"
                  disabled={prune.isPending}
                  onClick={() => open(d)}
                  className={d.danger ? "shrink-0 border-destructive/50 text-destructive hover:bg-destructive/10 hover:text-destructive" : "shrink-0"}
                >
                  Clean up…
                </Button>
              </>
            }
          />
        ))}
      </div>

      <AlertDialog open={!!active} onOpenChange={(o) => !o && setActive(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{active?.title}: run cleanup?</AlertDialogTitle>
            <AlertDialogDescription asChild>
              <div>
                <p>{active?.description}</p>
                <p className="mt-2 font-medium text-foreground">{active?.warning}</p>
                <p className="mt-2">This cannot be undone. The Engine reports what it removed in the Activity Center.</p>
                {(active?.variant || active?.requireAck) && (
                  <label className="mt-3 flex items-start gap-2 text-[13px] text-foreground">
                    <Checkbox className="mt-0.5" checked={checked} onCheckedChange={(c) => setChecked(c === true)} />
                    {active?.requireAck ?? active?.variant?.label}
                  </label>
                )}
              </div>
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction disabled={!canRun} className="bg-destructive! text-destructive-foreground! hover:bg-destructive/90!" onClick={() => void run()}>
              Run cleanup
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}
