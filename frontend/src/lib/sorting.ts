// Pure table sorting: stable, numbers numeric, unknown values always last (either direction).

export interface SortState {
  id: string;
  desc: boolean;
}

/** null / undefined / NaN / empty string mean "unknown" (rendered as "—") and sort last. */
export type SortValue = string | number | null | undefined;

export type Accessors<T> = Record<string, (item: T) => SortValue>;

/** The order each table has today when the user has not chosen one (undefined = backend order). */
export const DEFAULT_SORT: Record<string, SortState | undefined> = {
  containers: { id: "name", desc: false },
  images: undefined,
  volumes: undefined,
  networks: undefined,
  compose: { id: "name", desc: false },
};

const collator = new Intl.Collator(undefined, { numeric: true, sensitivity: "base" });

function isUnknown(v: SortValue): v is null | undefined {
  return v == null || v === "" || (typeof v === "number" && Number.isNaN(v));
}

/** Compare two values ascending; unknown values order after known ones. */
export function compareValues(a: SortValue, b: SortValue): number {
  const ua = isUnknown(a);
  const ub = isUnknown(b);
  if (ua || ub) return ua === ub ? 0 : ua ? 1 : -1;
  if (typeof a === "number" && typeof b === "number") return a - b;
  return collator.compare(String(a), String(b));
}

/** Stable sort by one accessor. Unknown values stay last even when descending. */
export function sortItems<T>(items: readonly T[], sort: SortState | undefined, accessors: Accessors<T>): T[] {
  const get = sort ? accessors[sort.id] : undefined;
  if (!sort || !get) return [...items];
  const dir = sort.desc ? -1 : 1;
  return items
    .map((item, index) => ({ item, index, value: get(item) }))
    .sort((x, y) => {
      const ux = isUnknown(x.value);
      const uy = isUnknown(y.value);
      if (ux || uy) return ux === uy ? x.index - y.index : ux ? 1 : -1;
      return dir * compareValues(x.value, y.value) || x.index - y.index;
    })
    .map((x) => x.item);
}

/** Clicking the active column flips direction; a new column starts ascending. */
export function nextSort(current: SortState | undefined, id: string): SortState {
  return current?.id === id ? { id, desc: !current.desc } : { id, desc: false };
}

export function ariaSort(current: SortState | undefined, id: string): "ascending" | "descending" | "none" {
  if (current?.id !== id) return "none";
  return current.desc ? "descending" : "ascending";
}
