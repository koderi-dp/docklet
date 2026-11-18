import { useCallback, useEffect, useMemo, useState } from "react";
import type { LogLevel } from "@/lib/logLevel";
import type { TimeRange } from "@/lib/logRange";
import { mergeImported, type MergeResult } from "@/lib/logFilter/savedFiltersFile";
import { newRuleId, type IncludeMode, type Rule } from "@/lib/logFilter/rules";
import { useLogFilterStore, type SavedFilter } from "@/stores/logFilterStore";

const sameRules = (a: readonly Rule[], b: readonly Rule[]) =>
  a.length === b.length && a.every((r, i) => r.kind === b[i].kind && r.target === b[i].target && r.pattern === b[i].pattern && r.enabled === b[i].enabled && r.colour === b[i].colour);

export interface LogFilterState {
  rules: Rule[];
  includeMode: IncludeMode;
  setIncludeMode: (m: IncludeMode) => void;
  addRule: (r: Omit<Rule, "id" | "enabled"> & { enabled?: boolean }) => void;
  updateRule: (id: string, patch: Partial<Omit<Rule, "id">>) => void;
  removeRule: (id: string) => void;
  levels: ReadonlySet<LogLevel>;
  toggleLevel: (l: LogLevel) => void;
  range: TimeRange;
  setRange: (r: TimeRange) => void;
  /** True when anything narrows the view: rules, levels or a time range. */
  active: boolean;
  clearAll: () => void;
  saved: SavedFilter[];
  activeSaved: SavedFilter | null;
  /** The open rule set differs from the saved one it came from. */
  modified: boolean;
  saveAs: (name: string) => void;
  applySaved: (id: string) => void;
  updateSaved: () => void;
  deleteSaved: (id: string) => void;
  /** Adds filters read from a file (names that are taken get " (imported)"). */
  importSaved: (incoming: SavedFilter[]) => MergeResult;
}

/**
 * State of the filter bar. With `restore` the last used rule set is loaded on open and kept up to date
 * (the Logs screen); without it the rules start empty and stay local (the single-container viewer).
 * Saved filters are global in both cases.
 */
export function useLogFilter({ restore }: { restore: boolean }): LogFilterState {
  const store = useLogFilterStore;
  const saved = useLogFilterStore((s) => s.saved);
  const [initial] = useState(() => (restore ? store.getState().last : { rules: [], includeMode: "any" as IncludeMode, activeSavedId: null }));
  const [rules, setRules] = useState<Rule[]>(initial.rules);
  const [includeMode, setIncludeMode] = useState<IncludeMode>(initial.includeMode);
  const [activeSavedId, setActiveSavedId] = useState<string | null>(initial.activeSavedId);
  const [levels, setLevels] = useState<ReadonlySet<LogLevel>>(new Set());
  const [range, setRange] = useState<TimeRange>("all");

  useEffect(() => {
    if (restore) store.getState().setLast({ rules, includeMode, activeSavedId });
  }, [restore, store, rules, includeMode, activeSavedId]);

  const activeSaved = useMemo(() => saved.find((s) => s.id === activeSavedId) ?? null, [saved, activeSavedId]);
  const modified = !!activeSaved && (!sameRules(activeSaved.rules, rules) || activeSaved.includeMode !== includeMode);

  const addRule = useCallback((r: Omit<Rule, "id" | "enabled"> & { enabled?: boolean }) => {
    setRules((prev) => {
      if (prev.some((p) => p.kind === r.kind && p.target === r.target && p.pattern === r.pattern)) return prev;
      return [...prev, { enabled: true, ...r, id: newRuleId(), colour: r.kind === "highlight" ? (r.colour ?? "warning") : undefined }];
    });
  }, []);

  const updateRule = useCallback((id: string, patch: Partial<Omit<Rule, "id">>) => {
    setRules((prev) =>
      prev.map((r) => {
        if (r.id !== id) return r;
        const next = { ...r, ...patch };
        if (next.kind === "highlight") next.colour ??= "warning";
        else delete next.colour;
        return next;
      }),
    );
  }, []);

  const removeRule = useCallback((id: string) => setRules((prev) => prev.filter((r) => r.id !== id)), []);

  const toggleLevel = useCallback((l: LogLevel) => {
    setLevels((prev) => {
      const next = new Set(prev);
      if (next.has(l)) next.delete(l);
      else next.add(l);
      return next;
    });
  }, []);

  const clearAll = useCallback(() => {
    setRules([]);
    setLevels(new Set());
    setRange("all");
    setActiveSavedId(null);
  }, []);

  const saveAs = useCallback(
    (name: string) => {
      const id = newRuleId();
      store.getState().setSaved([...store.getState().saved, { id, name: name.trim(), rules: rules.map((r) => ({ ...r })), includeMode }]);
      setActiveSavedId(id);
    },
    [store, rules, includeMode],
  );

  const applySaved = useCallback(
    (id: string) => {
      const s = store.getState().saved.find((x) => x.id === id);
      if (!s) return;
      setRules(s.rules.map((r) => ({ ...r, id: newRuleId() })));
      setIncludeMode(s.includeMode);
      setActiveSavedId(id);
    },
    [store],
  );

  const updateSaved = useCallback(() => {
    if (!activeSavedId) return;
    store.getState().setSaved(store.getState().saved.map((s) => (s.id === activeSavedId ? { ...s, rules: rules.map((r) => ({ ...r })), includeMode } : s)));
  }, [store, activeSavedId, rules, includeMode]);

  const deleteSaved = useCallback(
    (id: string) => {
      store.getState().setSaved(store.getState().saved.filter((s) => s.id !== id));
      setActiveSavedId((cur) => (cur === id ? null : cur));
    },
    [store],
  );

  const importSaved = useCallback(
    (incoming: SavedFilter[]) => {
      const r = mergeImported(store.getState().saved, incoming);
      store.getState().setSaved(r.merged);
      return r;
    },
    [store],
  );

  return {
    rules, includeMode, setIncludeMode,
    addRule, updateRule, removeRule,
    levels, toggleLevel, range, setRange,
    active: rules.length > 0 || levels.size > 0 || range !== "all",
    clearAll, saved, activeSaved, modified, saveAs, applySaved, updateSaved, deleteSaved, importSaved,
  };
}
