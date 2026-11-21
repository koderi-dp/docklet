import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useNavigate } from "react-router-dom";
import { toast } from "sonner";
import { portAwareDescription } from "@/components/ports/PortErrorHint";
import type { Container } from "@bindings/model";
import { useContainerAction, useContainersQuery, useContainerStats } from "@/hooks/useContainers";
import { useUI } from "@/stores/uiStore";
import {
  ACTION_LABEL,
  needsForceRemove,
  partitionByEligibility,
  resolveSelection,
  type ContainerAction,
} from "@/lib/actionEligibility";
import {
  buildGroups,
  applyRange,
  EMPTY_FILTERS,
  flattenFlat,
  flattenGrouped,
  isRunning,
  matchesFilters,
  pruneSelection,
  scopeToConnection,
  summarize,
  toggleIds,
  type ComposeGroup,
  type ContainerFilters,
} from "@/lib/composeGrouping";
import { sortFlatRows, sortGroups } from "@/lib/containerSort";
import { errorMessage } from "@/lib/errors";
import { useTableSort } from "@/lib/useTableSort";

/** All state and handlers of the Containers page: filters, selection, inspector, bulk actions, confirmations. */
export function useContainersPage(connectionId: string) {
  const view = useUI((s) => s.containerView);
  const setView = useUI((s) => s.setContainerView);
  const collapsedGroups = useUI((s) => s.collapsedGroups);
  const toggleGroup = useUI((s) => s.toggleGroup);
  const setAllGroups = useUI((s) => s.setAllGroups);
  const inspectorWidth = useUI((s) => s.inspectorWidth);
  const setInspectorWidth = useUI((s) => s.setInspectorWidth);

  const [filters, setFilters] = useState<ContainerFilters>(EMPTY_FILTERS);
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [inspectedId, setInspectedId] = useState<string | null>(null);
  const [inFlight, setInFlight] = useState<Set<string>>(new Set());
  const [colVis, setColVis] = useState<Record<string, boolean>>({});
  const [confirmRemove, setConfirmRemove] = useState<Container[] | null>(null);
  const [confirmKill, setConfirmKill] = useState<Container[] | null>(null);
  const [createOpen, setCreateOpen] = useState(false);
  const navigate = useNavigate();
  const anchorRef = useRef<string | null>(null);

  const list = useContainersQuery(connectionId, true);
  const containers = useMemo(() => scopeToConnection(list.data ?? [], connectionId), [list.data, connectionId]);
  const mutation = useContainerAction(connectionId);

  // Selection / inspector can only reference containers that still exist.
  useEffect(() => {
    if (!list.data) return;
    const ids = new Set(containers.map((c) => c.id));
    setSelected((s) => pruneSelection(s, ids));
    setInspectedId((id) => (id && !ids.has(id) ? null : id));
  }, [list.data, containers]);

  const isCollapsed = useCallback((g: ComposeGroup) => !!collapsedGroups[g.collapseKey], [collapsedGroups]);

  const visibleContainers = useMemo(() => containers.filter((c) => matchesFilters(c, filters)), [containers, filters]);
  // ALL running containers are watched (not only the visible ones): Compose group rows sum their project's
  // stats, and a filter must not change a project's total. Still one stream per container.
  const runningIds = useMemo(() => containers.filter(isRunning).map((c) => c.id), [containers]);
  const { stats } = useContainerStats(connectionId, runningIds, true);

  const { sort } = useTableSort("containers");
  const built = useMemo(() => buildGroups(connectionId, containers, filters, inFlight), [connectionId, containers, filters, inFlight]);
  const prevGroups = useRef<ComposeGroup[]>([]);
  const groups = useMemo(() => {
    const next = sortGroups(built, sort, stats, prevGroups.current);
    prevGroups.current = next;
    return next;
  }, [built, sort, stats]);
  const rows = useMemo(
    () => (view === "grouped" ? flattenGrouped(groups, isCollapsed) : sortFlatRows(flattenFlat(connectionId, containers, filters), sort, stats)),
    [view, groups, isCollapsed, connectionId, containers, filters, sort, stats],
  );

  const summary = useMemo(() => summarize(containers), [containers]);
  const tabCounts = summary && { all: summary.total, running: summary.running, stopped: summary.stopped };

  const inspected = containers.find((c) => c.id === inspectedId) ?? null;
  const selectedContainers = resolveSelection(containers, selected);

  // Esc closes the inspector (dialogs/menus handle Esc themselves first).
  useEffect(() => {
    if (!inspectedId) return;
    const onKey = (e: KeyboardEvent) => {
      const t = e.target as HTMLElement | null;
      if (e.key === "Escape" && !e.defaultPrevented && !(t && /input|textarea/i.test(t.tagName))) setInspectedId(null);
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [inspectedId]);

  const run = useCallback(
    async (action: ContainerAction, targets: Container[], force = false) => {
      const part = partitionByEligibility(action, targets);
      if (part.eligibleIds.length === 0) {
        toast.info(`Nothing to ${ACTION_LABEL[action].toLowerCase()}`, {
          description: `${part.skippedCount} selected container(s) are not eligible for this action.`,
        });
        return;
      }
      setInFlight((s) => new Set([...s, ...part.eligibleIds]));
      try {
        const out = await mutation.mutateAsync({ action, ids: part.eligibleIds, skippedCount: part.skippedCount, force });
        const ok = out.results.filter((r) => r.ok).length;
        const failed = out.results.filter((r) => !r.ok && !r.skipped);
        const engineSkipped = out.results.filter((r) => r.skipped).length;
        const skipped = out.skippedCount + engineSkipped;
        const verb = ACTION_LABEL[action];
        if (failed.length) {
          toast.error(`${verb}: ${failed.length} failed${ok ? `, ${ok} succeeded` : ""}`, {
            description: portAwareDescription(connectionId, failed.map((f) => `${f.name}: ${f.error}`).join("\n")),
            duration: Infinity,
            closeButton: true,
          });
        } else if (ok) {
          toast.success(`${verb}: ${ok} container${ok === 1 ? "" : "s"}${skipped ? ` (${skipped} skipped)` : ""}`);
        } else {
          toast.info(`${verb}: nothing changed`, { description: `${skipped} skipped` });
        }
      } catch (e) {
        toast.error(`${ACTION_LABEL[action]} failed`, { description: errorMessage(e), duration: Infinity, closeButton: true });
      } finally {
        setInFlight((s) => {
          const n = new Set(s);
          part.eligibleIds.forEach((id) => n.delete(id));
          return n;
        });
      }
    },
    [mutation],
  );

  const requestAction = useCallback(
    (action: ContainerAction, targets: Container[]) => {
      if (action === "remove") setConfirmRemove(targets);
      else if (action === "kill") {
        const part = partitionByEligibility("kill", targets);
        if (part.eligible.length === 0) void run(action, targets);
        else setConfirmKill(part.eligible);
      } else void run(action, targets);
    },
    [run],
  );

  const bulk = (action: ContainerAction) => requestAction(action, selectedContainers);
  const allGroupKeys = groups.map((g) => g.collapseKey);

  const onToggleSelect = useCallback((id: string) => {
    anchorRef.current = id;
    setSelected((s) => toggleIds(s, [id]));
  }, []);
  const onSelectRange = useCallback((ordered: string[], id: string) => {
    const anchor = anchorRef.current;
    anchorRef.current = id;
    setSelected((s) => applyRange(s, ordered, anchor, id, !s.has(id)));
  }, []);
  const onOpenDetails = useCallback((c: Container) => navigate(`/containers/${encodeURIComponent(c.id)}`), [navigate]);
  const onToggleSelectMany = useCallback((ids: string[]) => setSelected((s) => toggleIds(s, ids)), []);
  const onInspect = useCallback((c: Container) => setInspectedId(c.id), []);

  const hiddenSelected = selectedContainers.filter((c) => !visibleContainers.includes(c)).length;
  const forceNeeded = confirmRemove ? needsForceRemove(confirmRemove) : false;

  return {
    view, setView, toggleGroup, setAllGroups, inspectorWidth, setInspectorWidth,
    filters, setFilters, selected, setSelected, inspectedId, setInspectedId, inFlight, colVis, setColVis,
    confirmRemove, setConfirmRemove, confirmKill, setConfirmKill, forceNeeded, createOpen, setCreateOpen,
    list, containers, mutation, isCollapsed, groups, rows, summary, tabCounts, visibleContainers, stats,
    inspected, selectedContainers, hiddenSelected, run, requestAction, bulk, allGroupKeys,
    onToggleSelect, onSelectRange, onOpenDetails, onToggleSelectMany, onInspect,
  };
}
