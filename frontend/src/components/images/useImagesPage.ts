import { useCallback, useEffect, useMemo, useState } from "react";
import { toast } from "sonner";
import type { ImageRow } from "@bindings/model";
import { useImagesQuery, usePruneDangling, useRemoveImages } from "@/hooks/useImages";
import { useOperations } from "@/hooks/useOperations";
import { useUI } from "@/stores/uiStore";
import { matchesImage, summarizeImages, type ImageFilter } from "./imageFilters";
import { useImageTransfers } from "./useImageTransfers";
import { errorMessage } from "@/lib/errors";

/** All state and handlers of the Images page (selection, inspector, dialogs, bulk actions). */
export function useImagesPage(connectionId: string) {
  const inspectorWidth = useUI((s) => s.inspectorWidth);
  const setInspectorWidth = useUI((s) => s.setInspectorWidth);
  const { setCenterOpen } = useOperations();

  const [query, setQuery] = useState("");
  const [filter, setFilter] = useState<ImageFilter>("all");
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [inspectedKey, setInspectedKey] = useState<string | null>(null);
  const [pullOpen, setPullOpen] = useState(false);
  const [buildOpen, setBuildOpen] = useState(false);
  const [pushTarget, setPushTarget] = useState<ImageRow | null>(null);
  const [tagTarget, setTagTarget] = useState<ImageRow | null>(null);
  const [createFor, setCreateFor] = useState<ImageRow | null>(null);
  const [confirmRemove, setConfirmRemove] = useState<ImageRow[] | null>(null);
  const [force, setForce] = useState(false);
  const [confirmPrune, setConfirmPrune] = useState(false);

  const list = useImagesQuery(connectionId, true);
  const rows = useMemo(() => (list.data ?? []).filter((r) => r.connectionId === connectionId), [list.data, connectionId]);
  const remove = useRemoveImages(connectionId);
  const prune = usePruneDangling(connectionId);
  const transfers = useImageTransfers(connectionId);

  const summary = useMemo(() => summarizeImages(rows), [rows]);
  const visible = useMemo(() => rows.filter((r) => matchesImage(r, filter, query)), [rows, filter, query]);
  const counts = useMemo(
    () => ({
      all: rows.length,
      inuse: rows.filter((r) => r.inUse).length,
      unused: rows.filter((r) => !r.inUse).length,
      dangling: rows.filter((r) => r.dangling).length,
    }),
    [rows],
  );

  // Selection / inspector only reference rows that still exist.
  useEffect(() => {
    if (!list.data) return;
    const keys = new Set(rows.map((r) => r.key));
    setSelected((s) => {
      const n = new Set([...s].filter((k) => keys.has(k)));
      return n.size === s.size ? s : n;
    });
    setInspectedKey((k) => (k && !keys.has(k) ? null : k));
  }, [list.data, rows]);

  useEffect(() => {
    if (!inspectedKey) return;
    const onKey = (e: KeyboardEvent) => {
      const t = e.target as HTMLElement | null;
      if (e.key === "Escape" && !e.defaultPrevented && !(t && /input|textarea/i.test(t.tagName))) setInspectedKey(null);
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [inspectedKey]);

  const inspected = rows.find((r) => r.key === inspectedKey) ?? null;
  const selectedRows = rows.filter((r) => selected.has(r.key));

  const onToggleSelect = useCallback(
    (key: string) =>
      setSelected((s) => {
        const n = new Set(s);
        if (n.has(key)) n.delete(key);
        else n.add(key);
        return n;
      }),
    [],
  );
  const onToggleSelectAll = useCallback(() => {
    setSelected((s) => {
      const allOn = visible.length > 0 && visible.every((r) => s.has(r.key));
      const n = new Set(s);
      for (const r of visible) {
        if (allOn) n.delete(r.key);
        else n.add(r.key);
      }
      return n;
    });
  }, [visible]);
  const onInspect = useCallback((r: ImageRow) => setInspectedKey(r.key), []);
  const onTag = useCallback((r: ImageRow) => setTagTarget(r), []);
  const onPush = useCallback((r: ImageRow) => setPushTarget(r), []);
  const onSaveRow = useCallback((r: ImageRow) => void transfers.saveImages([r]), [transfers.saveImages]);
  const onCreate = useCallback((r: ImageRow) => setCreateFor(r), []);
  const onRemoveRow = useCallback((r: ImageRow) => {
    setForce(false);
    setConfirmRemove([r]);
  }, []);

  const doRemove = async () => {
    const targets = confirmRemove;
    setConfirmRemove(null);
    if (!targets?.length) return;
    try {
      const results = await remove.mutateAsync({ refs: targets.map((t) => t.ref), force });
      const ok = results.filter((r) => r.ok).length;
      const failed = results.filter((r) => !r.ok);
      if (failed.length) {
        toast.error(`Remove: ${failed.length} failed${ok ? `, ${ok} removed` : ""}`, {
          description: failed.map((f) => `${f.name}: ${f.error}`).join("\n"),
          duration: Infinity,
          closeButton: true,
        });
      } else {
        toast.success(`Removed ${ok} image reference${ok === 1 ? "" : "s"}`);
      }
      setSelected(new Set());
    } catch (e) {
      toast.error("Remove failed", { description: errorMessage(e), duration: Infinity, closeButton: true });
    }
  };

  const doPrune = async () => {
    setConfirmPrune(false);
    try {
      const op = await prune.mutateAsync();
      toast.success("Removing dangling images", {
        description: op.title,
        action: { label: "View", onClick: () => setCenterOpen(true) },
      });
    } catch (e) {
      toast.error("Could not start cleanup", { description: errorMessage(e), duration: Infinity, closeButton: true });
    }
  };

  const danglingRows = rows.filter((r) => r.dangling && !r.inUse);

  return {
    // data
    list, rows, visible, counts, summary, danglingRows, inspected, selected, selectedRows, query, filter, transfers,
    // pending flags
    removing: remove.isPending, pruning: prune.isPending,
    // filters / selection
    setQuery, setFilter, setSelected, onToggleSelect, onToggleSelectAll,
    // row actions
    onInspect, onTag, onPush, onSaveRow, onCreate, onRemoveRow, closeInspector: () => setInspectedKey(null),
    inspectedKey, inspectorWidth, setInspectorWidth,
    // dialogs
    pullOpen, setPullOpen, buildOpen, setBuildOpen, pushTarget, setPushTarget, tagTarget, setTagTarget, createFor, setCreateFor,
    confirmRemove, setConfirmRemove, force, setForce, confirmPrune, setConfirmPrune, doRemove, doPrune,
    requestRemoveSelected: () => { setForce(false); setConfirmRemove(selectedRows); },
  };
}
