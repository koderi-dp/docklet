import { useEffect, useMemo, useState } from "react";
import { toast } from "sonner";
import { OperationStatus, type Volume } from "@bindings/model";
import { useOnOperationDone } from "@/hooks/useOperations";
import { usePruneVolumes, useRemoveVolumes, useVolumesQuery } from "@/hooks/useVolumes";
import type { UsageFilter } from "./parts";
import { errorMessage } from "@/lib/errors";
import { sortItems, type Accessors } from "@/lib/sorting";
import { useTableSort } from "@/lib/useTableSort";

export const volumeUsers = (v: Volume) => (v.inUseBy ?? []).length;

const VOLUME_SORT: Accessors<Volume> = {
  name: (v) => v.name,
  driver: (v) => v.driver,
  inuse: volumeUsers,
  created: (v) => (v.created ? Date.parse(v.created) : undefined),
};

/** All state and handlers of the Volumes page (filters, selection, inspector, remove/prune flows). */
export function useVolumesPage(connectionId: string) {
  const list = useVolumesQuery(connectionId);
  const remove = useRemoveVolumes(connectionId);
  const prune = usePruneVolumes(connectionId);

  const [search, setSearch] = useState("");
  const [filter, setFilter] = useState<UsageFilter>("all");
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [inspected, setInspected] = useState<string | null>(null);
  const [createOpen, setCreateOpen] = useState(false);
  const [confirmRemove, setConfirmRemove] = useState<Volume[] | null>(null);
  const [pruneOpen, setPruneOpen] = useState(false);
  const [understood, setUnderstood] = useState(false);

  const volumes = useMemo(() => (list.data ?? []).filter((v) => v.connectionId === connectionId), [list.data, connectionId]);
  const usedCount = volumeUsers;
  const counts = useMemo(
    () => ({ all: volumes.length, inuse: volumes.filter((v) => usedCount(v) > 0).length, unused: volumes.filter((v) => usedCount(v) === 0).length }),
    [volumes],
  );
  const { sort } = useTableSort("volumes");
  const visible = useMemo(() => {
    const q = search.trim().toLowerCase();
    return sortItems(volumes.filter((v) => {
      if (filter === "inuse" && usedCount(v) === 0) return false;
      if (filter === "unused" && usedCount(v) > 0) return false;
      if (!q) return true;
      return v.name.toLowerCase().includes(q) || v.driver.toLowerCase().includes(q) || Object.entries(v.labels ?? {}).some(([k, val]) => `${k}=${val}`.toLowerCase().includes(q));
    }), sort, VOLUME_SORT);
  }, [volumes, search, filter, sort]);

  // Selection / inspector can only reference volumes that still exist.
  useEffect(() => {
    if (!list.data) return;
    const names = new Set(volumes.map((v) => v.name));
    setSelected((s) => new Set([...s].filter((n) => names.has(n))));
    setInspected((n) => (n && !names.has(n) ? null : n));
  }, [list.data, volumes]);

  useOnOperationDone("volume.prune", connectionId, (op) => {
    if (op.status === OperationStatus.OpSucceeded) toast.success("Unused volumes pruned", { description: op.detail });
    else if (op.status === OperationStatus.OpFailed) toast.error("Volume prune failed", { description: op.error, duration: Infinity, closeButton: true });
  });

  const unusedVolumes = volumes.filter((v) => usedCount(v) === 0);
  const inspectedVolume = volumes.find((v) => v.name === inspected) ?? null;
  const selectedVolumes = volumes.filter((v) => selected.has(v.name));
  const allVisibleSelected = visible.length > 0 && visible.every((v) => selected.has(v.name));

  const doRemove = async (targets: Volume[]) => {
    try {
      const results = await remove.mutateAsync(targets.map((v) => v.name));
      const ok = (results ?? []).filter((r) => r.ok).length;
      const failed = (results ?? []).filter((r) => !r.ok);
      if (failed.length) {
        toast.error(`Remove: ${failed.length} failed${ok ? `, ${ok} removed` : ""}`, {
          description: failed.map((f) => f.error || f.name).join("\n"),
          duration: Infinity,
          closeButton: true,
        });
      } else {
        toast.success(`Removed ${ok} volume${ok === 1 ? "" : "s"}`);
      }
      setSelected((s) => new Set([...s].filter((n) => !(results ?? []).find((r) => r.ok && r.name === n))));
    } catch (e) {
      toast.error("Remove failed", { description: errorMessage(e), duration: Infinity, closeButton: true });
    }
  };

  const startPrune = async () => {
    setPruneOpen(false);
    try {
      await prune.mutateAsync();
      toast.info("Volume prune started", { description: "Progress is shown in the Activity Center." });
    } catch (e) {
      toast.error("Could not start volume prune", { description: errorMessage(e), duration: Infinity, closeButton: true });
    }
  };

  const toggle = (name: string) =>
    setSelected((s) => {
      const n = new Set(s);
      if (n.has(name)) n.delete(name);
      else n.add(name);
      return n;
    });

  const toggleAllVisible = (on: boolean) =>
    setSelected((s) => {
      const n = new Set(s);
      visible.forEach((v) => (on ? n.add(v.name) : n.delete(v.name)));
      return n;
    });

  return {
    list, volumes, visible, counts, search, setSearch, filter, setFilter,
    selected, setSelected, selectedVolumes, allVisibleSelected, toggle, toggleAllVisible,
    inspected, setInspected, inspectedVolume,
    createOpen, setCreateOpen, confirmRemove, setConfirmRemove, pruneOpen, setPruneOpen, understood, setUnderstood,
    unusedVolumes, removing: remove.isPending, pruning: prune.isPending, doRemove, startPrune,
  };
}
