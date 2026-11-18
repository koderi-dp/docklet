import { toast } from "sonner";
import { create } from "zustand";
import { PortService } from "@bindings/services";
import type { PortCheck } from "@bindings/ports";
import { errorMessage } from "@/lib/errors";
import { isInUse } from "@/lib/portCheck";
import { PortConflictDialog } from "./PortConflictDialog";

interface Pending {
  project: string;
  conflicts: PortCheck[];
  resolve: (go: boolean) => void;
}

const usePending = create<{ pending: Pending | null }>(() => ({ pending: null }));

/**
 * Before `compose up`: reads the ports the project would publish and checks them. Resolves true to go
 * ahead — at once when nothing conflicts (no dialog, no friction), or after "Start anyway".
 * A check that cannot run never blocks the start.
 */
export async function confirmComposeUp(connectionId: string, project: string, service: string, profiles: string[]): Promise<boolean> {
  let conflicts: PortCheck[];
  try {
    const res = await PortService.CheckProject(connectionId, project, service, profiles);
    conflicts = (res.checks ?? []).filter(isInUse);
  } catch (e) {
    toast.info("Ports were not checked", { description: errorMessage(e) });
    return true;
  }
  if (conflicts.length === 0) return true;
  return new Promise<boolean>((resolve) => usePending.setState({ pending: { project, conflicts, resolve } }));
}

/** Mounted once in the app shell: shows the dialog of a pending compose-up port check. */
export function PortGuardHost() {
  const pending = usePending((s) => s.pending);
  if (!pending) return null;
  const done = (go: boolean) => {
    usePending.setState({ pending: null });
    pending.resolve(go);
  };
  const n = pending.conflicts.length;
  return (
    <PortConflictDialog
      title={n === 1 ? "A port is already in use" : `${n} ports are already in use`}
      intro={
        <>
          Project <span className="font-medium text-foreground">{pending.project}</span> publishes {n === 1 ? "a port" : "ports"} that something else holds. The services using
          {n === 1 ? " it" : " them"} will fail to start until the holder is stopped.
        </>
      }
      conflicts={pending.conflicts}
      confirmLabel="Start anyway"
      onConfirm={() => done(true)}
      onCancel={() => done(false)}
    />
  );
}
