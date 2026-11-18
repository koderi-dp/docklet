import type { LogEntry } from "@/lib/logEntry";

type Reveal = (seq: number) => boolean;

/**
 * What the search needs to know about the lists that are drawn: how to scroll a line into view (every list),
 * and, in the split view, which lines the panes show together. Plain object so lists can register without
 * re-rendering the owner of the search.
 */
export class SearchRegistry {
  private reveals = new Map<string, Reveal>();
  private panes = new Map<number, readonly LogEntry[]>();
  private listeners = new Set<() => void>();
  private version = 0;
  /** Set while a search is active: pane changes then notify, otherwise they are stored silently. */
  watching = false;

  addReveal(id: string, fn: Reveal): () => void {
    this.reveals.set(id, fn);
    return () => {
      if (this.reveals.get(id) === fn) this.reveals.delete(id);
    };
  }

  /** Scrolls the line into view in every list that holds it; true when at least one did. */
  reveal(seq: number): boolean {
    let any = false;
    for (const fn of this.reveals.values()) any = fn(seq) || any;
    return any;
  }

  setPane(index: number, entries: readonly LogEntry[] | null): void {
    if (entries === null) this.panes.delete(index);
    else this.panes.set(index, entries);
    if (this.watching) this.bump();
  }

  /** Sequence numbers of every line some pane shows; null when no pane is registered (single list). */
  memberSeqs(): Set<number> | null {
    if (this.panes.size === 0) return null;
    const out = new Set<number>();
    for (const list of this.panes.values()) for (const e of list) out.add(e.seq);
    return out;
  }

  private bump() {
    this.version++;
    for (const l of this.listeners) l();
  }

  subscribe = (l: () => void): (() => void) => {
    this.listeners.add(l);
    return () => this.listeners.delete(l);
  };

  getVersion = (): number => this.version;
}
