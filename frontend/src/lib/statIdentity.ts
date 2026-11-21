// Keeps ContainerStat objects identical across stats ticks when their values did not change,
// so memoised rows can skip re-rendering. The Map itself is still new on every tick.
import type { ContainerStat } from "@bindings/model";

/** Same id and same CPU / memory values (null and undefined both mean "no sample"). */
export function sameStat(a: ContainerStat, b: ContainerStat): boolean {
  return (
    a.id === b.id &&
    (a.cpuPercent ?? null) === (b.cpuPercent ?? null) &&
    (a.memoryBytes ?? null) === (b.memoryBytes ?? null) &&
    (a.memoryLimit ?? null) === (b.memoryLimit ?? null)
  );
}

/** Previous wanted entries plus the incoming wanted samples; an unchanged sample keeps its previous object. */
export function mergeStats(prev: ReadonlyMap<string, ContainerStat>, incoming: readonly ContainerStat[], wanted: ReadonlySet<string>): Map<string, ContainerStat> {
  const map = new Map<string, ContainerStat>();
  for (const [k, v] of prev) if (wanted.has(k)) map.set(k, v);
  for (const st of incoming) {
    if (!wanted.has(st.id)) continue;
    const old = map.get(st.id);
    map.set(st.id, old && sameStat(old, st) ? old : st);
  }
  return map;
}
