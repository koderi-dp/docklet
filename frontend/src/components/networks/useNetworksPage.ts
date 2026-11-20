import { useEffect, useMemo, useState } from "react";
import { toast } from "sonner";
import { OperationStatus, type Network } from "@bindings/model";
import { useOnOperationDone } from "@/hooks/useOperations";
import { useNetworksQuery, usePruneNetworks, useRemoveNetworks } from "@/hooks/useNetworks";
import type { UsageFilter } from "@/components/volumes/parts";
import { errorMessage } from "@/lib/errors";
import { sortItems, type Accessors } from "@/lib/sorting";
import { useTableSort } from "@/lib/useTableSort";

export const networkAttached = (n: Network) => (n.attached ?? []).length;

const NETWORK_SORT: Accessors<Network> = {
  name: (n) => n.name,
  driver: (n) => n.driver,
  attached: networkAttached,
};

/** All state and handlers of the Networks page (filters, selection, inspector, remove/prune flows). */
export function useNetworksPage(connectionId: string) {
  const list = useNetworksQuery(connectionId);
  const remove = useRemoveNetworks(connectionId);
  const prune = usePruneNetworks(connectionId);

  const [search, setSearch] = useState("");
  const [filter, setFilter] = useState<UsageFilter>("all");
  const [selected, setSelected] = useState<Set<string>>(new Set()); // network ids
  const [inspected, setInspected] = useState<string | null>(null);
  const [createOpen, setCreateOpen] = useState(false);
  const [confirmRemove, setConfirmRemove] = useState<Network[] | null>(null);
  const [pruneOpen, setPruneOpen] = useState(false);

  const networks = useMemo(() => (list.data ?? []).filter((n) => n.connectionId === connectionId), [list.data, connectionId]);
  const attachedCount = networkAttached;
  const counts = useMemo(
    () => ({ all: networks.length, inuse: networks.filter((n) => attachedCount(n) > 0).length, unused: networks.filter((n) => attachedCount(n) === 0).length }),
    [networks],
  );
  const { sort } = useTableSort("networks");
  const visible = useMemo(() => {
    const q = search.trim().toLowerCase();
    return sortItems(networks.filter((n) => {
      if (filter === "inuse" && attachedCount(n) === 0) return false;
      if (filter === "unused" && attachedCount(n) > 0) return false;
      if (!q) return true;
      return (
        n.name.toLowerCase().includes(q) ||
        n.driver.toLowerCase().includes(q) ||
        (n.subnets ?? []).some((s) => s.includes(q)) ||
        Object.entries(n.labels ?? {}).some(([k, v]) => `${k}=${v}`.toLowerCase().includes(q))
      );
    }), sort, NETWORK_SORT);
  }, [networks, search, filter, sort]);

  useEffect(() => {
    if (!list.data) return;
    const ids = new Set(networks.map((n) => n.id));
    setSelected((s) => new Set([...s].filter((id) => ids.has(id))));
    setInspected((id) => (id && !ids.has(id) ? null : id));
  }, [list.data, networks]);

  useOnOperationDone("network.prune", connectionId, (op) => {
    if (op.status === OperationStatus.OpSucceeded) toast.success("Unused networks pruned", { description: op.detail });
    else if (op.status === OperationStatus.OpFailed) toast.error("Network prune failed", { description: op.error, duration: Infinity, closeButton: true });
  });

  const inspectedNetwork = networks.find((n) => n.id === inspected) ?? null;
  const selectedNetworks = networks.filter((n) => selected.has(n.id));
  const removable = selectedNetworks.filter((n) => !n.predefined);
  const unused = networks.filter((n) => attachedCount(n) === 0 && !n.predefined);
  const allVisibleSelected = visible.length > 0 && visible.every((n) => selected.has(n.id));

  const doRemove = async (targets: Network[]) => {
    try {
      const results = await remove.mutateAsync(targets.map((n) => n.id));
      const ok = (results ?? []).filter((r) => r.ok).length;
      const failed = (results ?? []).filter((r) => !r.ok);
      if (failed.length) {
        toast.error(`Remove: ${failed.length} failed${ok ? `, ${ok} removed` : ""}`, {
          description: failed.map((f) => f.error || f.name).join("\n"),
          duration: Infinity,
          closeButton: true,
        });
      } else {
        toast.success(`Removed ${ok} network${ok === 1 ? "" : "s"}`);
      }
      const gone = new Set((results ?? []).filter((r) => r.ok).map((r) => r.id));
      setSelected((s) => new Set([...s].filter((id) => !gone.has(id))));
    } catch (e) {
      toast.error("Remove failed", { description: errorMessage(e), duration: Infinity, closeButton: true });
    }
  };

  const startPrune = async () => {
    setPruneOpen(false);
    try {
      await prune.mutateAsync();
      toast.info("Network prune started", { description: "Progress is shown in Operations." });
    } catch (e) {
      toast.error("Could not start network prune", { description: errorMessage(e), duration: Infinity, closeButton: true });
    }
  };

  const toggle = (id: string) =>
    setSelected((s) => {
      const n = new Set(s);
      if (n.has(id)) n.delete(id);
      else n.add(id);
      return n;
    });

  const toggleAllVisible = (on: boolean) =>
    setSelected((s) => {
      const n = new Set(s);
      visible.forEach((v) => (on ? n.add(v.id) : n.delete(v.id)));
      return n;
    });

  return {
    list, networks, visible, counts, search, setSearch, filter, setFilter,
    selected, setSelected, removable, allVisibleSelected, toggle, toggleAllVisible,
    inspected, setInspected, inspectedNetwork,
    createOpen, setCreateOpen, confirmRemove, setConfirmRemove, pruneOpen, setPruneOpen,
    unused, removing: remove.isPending, pruning: prune.isPending, doRemove, startPrune,
  };
}
