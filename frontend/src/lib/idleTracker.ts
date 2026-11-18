// In-memory idle detection for Compose projects. Nothing is persisted: it resets when Docklet restarts.

export interface IdleConfig {
  /** Summed CPU (percent of one core) below which a project counts as quiet. */
  cpuThreshold: number;
  /** How long the project must have stayed quiet before it is called idle. */
  minMs: number;
  /** A pause in observations longer than this (page hidden, other page) restarts the window: we were not watching. */
  maxGapMs: number;
}

export const IDLE_CONFIG: IdleConfig = {
  cpuThreshold: 1,
  minMs: 10 * 60_000,
  maxGapMs: 15_000,
};

export interface IdleObservation {
  /** Summed CPU of the running containers; null when no sample exists. */
  cpuPercent: number | null;
  /** True only when EVERY running container delivered a CPU sample (no idle claim on partial data). */
  complete: boolean;
  /** Identity of the running set; a change (container started/stopped) restarts the window. */
  membersKey: string;
}

export interface IdleInfo {
  /** Epoch ms of the start of the quiet window. */
  since: number;
  idleForMs: number;
}

interface State {
  since: number | null;
  last: number;
  membersKey: string;
}

export class IdleTracker {
  private readonly states = new Map<string, State>();

  constructor(private readonly cfg: IdleConfig = IDLE_CONFIG) {}

  observe(key: string, now: number, o: IdleObservation): void {
    const quiet = o.complete && o.cpuPercent != null && o.cpuPercent < this.cfg.cpuThreshold && o.membersKey !== "";
    const prev = this.states.get(key);
    if (!quiet) {
      this.states.set(key, { since: null, last: now, membersKey: o.membersKey });
      return;
    }
    const restart = !prev || prev.since == null || prev.membersKey !== o.membersKey || now - prev.last > this.cfg.maxGapMs;
    this.states.set(key, { since: restart ? now : prev.since, last: now, membersKey: o.membersKey });
  }

  /** Idle info once the quiet window reached the minimum; null otherwise. */
  idle(key: string, now: number): IdleInfo | null {
    const s = this.states.get(key);
    if (!s || s.since == null || now - s.last > this.cfg.maxGapMs) return null;
    const idleForMs = now - s.since;
    return idleForMs >= this.cfg.minMs ? { since: s.since, idleForMs } : null;
  }

  forget(key: string): void {
    this.states.delete(key);
  }
}

/** App-wide tracker shared by every page that shows project totals. */
export const idleTracker = new IdleTracker();

/** "12 min" / "1 h 5 min" for the chip. */
export function formatIdleDuration(ms: number): string {
  const min = Math.max(1, Math.floor(ms / 60_000));
  if (min < 60) return `${min} min`;
  const h = Math.floor(min / 60);
  const m = min % 60;
  return m === 0 ? `${h} h` : `${h} h ${m} min`;
}
