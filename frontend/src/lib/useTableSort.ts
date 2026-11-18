import { useCallback } from "react";
import { useUI } from "@/stores/uiStore";
import { DEFAULT_SORT, nextSort, type SortState } from "./sorting";

/** Persisted sort of one table (UI store); falls back to the table's default order. */
export function useTableSort(table: string): { sort: SortState | undefined; toggle: (id: string) => void } {
  const stored = useUI((s) => s.tableSort[table]);
  const setTableSort = useUI((s) => s.setTableSort);
  const sort = stored ?? DEFAULT_SORT[table];
  const toggle = useCallback((id: string) => setTableSort(table, nextSort(sort, id)), [setTableSort, table, sort]);
  return { sort, toggle };
}
