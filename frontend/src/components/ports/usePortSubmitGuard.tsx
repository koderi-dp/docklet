import { useCallback, useRef, useState, type ReactNode } from "react";
import { toast } from "sonner";
import { PortService } from "@bindings/services";
import type { PortCheck } from "@bindings/ports";
import type { PortRow } from "@/components/containers/createContainerForm";
import { checkRowsNow } from "@/hooks/usePortChecks";
import { errorMessage } from "@/lib/errors";
import { isInUse, requestsFromRows } from "@/lib/portCheck";
import { PortConflictDialog } from "./PortConflictDialog";

interface Open {
  conflicts: { index: number; check: PortCheck }[];
  resolve: (go: boolean) => void;
}

/**
 * Guards "Create" and "Recreate": a fresh check at submit time; known conflicts ask first. `confirm()` resolves
 * true to go on (nothing conflicts, the check could not run, or "… anyway"), false after Cancel or "Pick another port".
 */
export function usePortSubmitGuard(opts: {
  connectionId: string;
  rows: PortRow[];
  setRows: (rows: PortRow[]) => void;
  ignoreContainerIds?: string[];
  anywayLabel: string;
  what: string; // "container" | "replacement container"
}): { confirm: () => Promise<boolean>; dialog: ReactNode } {
  const { connectionId, rows, setRows, ignoreContainerIds, anywayLabel, what } = opts;
  const [open, setOpen] = useState<Open | null>(null);
  const [picking, setPicking] = useState(false);
  const latest = useRef({ rows, ignoreContainerIds });
  latest.current = { rows, ignoreContainerIds };

  const confirm = useCallback(async () => {
    let found: { index: number; check: PortCheck }[];
    try {
      found = (await checkRowsNow(connectionId, latest.current.rows, latest.current.ignoreContainerIds)).filter((r) => isInUse(r.check));
    } catch (e) {
      toast.info("Ports were not checked", { description: errorMessage(e) }); // never blocks
      return true;
    }
    if (found.length === 0) return true;
    return new Promise<boolean>((resolve) => setOpen({ conflicts: found, resolve }));
  }, [connectionId]);

  const finish = (go: boolean) => {
    open?.resolve(go);
    setOpen(null);
  };

  const pick = async () => {
    if (!open) return;
    setPicking(true);
    try {
      const reqs = requestsFromRows(latest.current.rows);
      const next = latest.current.rows.map((r) => ({ ...r }));
      for (const c of open.conflicts) {
        const req = reqs.find((q) => q.index === c.index)?.request;
        if (!req) continue;
        const s = await PortService.NextFree(connectionId, req, latest.current.ignoreContainerIds ?? []);
        if (s.port > 0) next[c.index].hostPort = String(s.port);
      }
      setRows(next);
      finish(false);
    } catch (e) {
      toast.error("Could not look for a free port", { description: errorMessage(e) });
    } finally {
      setPicking(false);
    }
  };

  const n = open?.conflicts.length ?? 0;
  const dialog = open && (
    <PortConflictDialog
      title={n === 1 ? "This host port is already in use" : `${n} host ports are already in use`}
      intro={`Docker will not be able to start the ${what} while something else holds ${n === 1 ? "this port" : "these ports"}.`}
      conflicts={open.conflicts.map((c) => c.check)}
      confirmLabel={anywayLabel}
      onConfirm={() => finish(true)}
      onCancel={() => finish(false)}
      onPick={() => void pick()}
      picking={picking}
    />
  );
  return { confirm, dialog };
}
