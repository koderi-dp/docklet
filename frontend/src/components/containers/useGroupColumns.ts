import { useLayoutEffect, useRef, useState } from "react";
import type { ColumnVisibilityState } from "@tanstack/react-table";

/**
 * Minimum table width (px) at which a responsive column is shown. Mirrors RESPONSIVE_COL in ContainerTable
 * (CSS container queries); the group row needs the same answer in JS to place its totals in the CPU / Memory columns.
 */
export const RESPONSIVE_MIN_WIDTH: Record<string, number> = { project: 900, cpu: 900, memory: 820, uptime: 1200 };

const BEFORE = ["select", "name", "image", "project", "status", "health", "ports"];

export interface GroupColumns {
  /** Visible columns left of CPU (the group label spans them). */
  before: number;
  cpu: boolean;
  memory: boolean;
  /** Visible columns right of Memory (uptime, actions). */
  after: number;
}

export function groupColumns(width: number, vis: ColumnVisibilityState): GroupColumns {
  const shown = (id: string) => vis[id] !== false && width >= (RESPONSIVE_MIN_WIDTH[id] ?? 0);
  return {
    before: BEFORE.filter(shown).length,
    cpu: shown("cpu"),
    memory: shown("memory"),
    after: ["uptime", "actions"].filter(shown).length,
  };
}

/** Measures the table wrapper and returns the group-row column layout; re-renders only when the layout changes. */
export function useGroupColumns(vis: ColumnVisibilityState) {
  const ref = useRef<HTMLDivElement>(null);
  const [cols, setCols] = useState<GroupColumns>(() => groupColumns(Infinity, vis));
  const visRef = useRef(vis);
  visRef.current = vis;
  const widthRef = useRef(Infinity);

  const recompute = () => {
    const next = groupColumns(widthRef.current, visRef.current);
    setCols((prev) => (prev.before === next.before && prev.cpu === next.cpu && prev.memory === next.memory && prev.after === next.after ? prev : next));
  };

  useLayoutEffect(() => {
    const el = ref.current;
    if (!el) return;
    const ro = new ResizeObserver((entries) => {
      widthRef.current = entries[0].contentRect.width;
      recompute();
    });
    widthRef.current = el.clientWidth;
    recompute();
    ro.observe(el);
    return () => ro.disconnect();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);
  useLayoutEffect(recompute);

  return { ref, cols };
}
