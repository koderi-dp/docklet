import { useEffect, useState } from "react";
import { OperationStatus, type Operation } from "@bindings/model";

export function formatElapsed(ms: number): string {
  const s = Math.max(0, Math.floor(ms / 1000));
  if (s < 60) return `${s}s`;
  const m = Math.floor(s / 60);
  if (m < 60) return `${m}m ${s % 60}s`;
  return `${Math.floor(m / 60)}h ${m % 60}m`;
}

/** Time the operation has run (ticking while it runs) or took. */
export function Elapsed({ op }: { op: Operation }) {
  const running = op.status === OperationStatus.OpRunning;
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    if (!running) return;
    const t = setInterval(() => setNow(Date.now()), 1000);
    return () => clearInterval(t);
  }, [running]);
  const end = running ? now : op.finishedAt || now;
  return <span className="tabular-nums">{formatElapsed(end - op.startedAt)}</span>;
}

/** Bar of a running operation; pulses when the progress is unknown. Renders nothing once it finished. */
export function ProgressBar({ op }: { op: Operation }) {
  if (op.status !== OperationStatus.OpRunning) return null;
  const known = op.progress != null;
  const pct = known ? Math.round((op.progress as number) * 100) : undefined;
  return (
    <div
      className="h-1 w-full overflow-hidden rounded-full bg-muted"
      role="progressbar"
      aria-label={known ? "Progress" : "Running, progress unknown"}
      aria-valuemin={known ? 0 : undefined}
      aria-valuemax={known ? 100 : undefined}
      aria-valuenow={pct}
    >
      {known ? (
        <div className="h-full rounded-full bg-primary transition-[width]" style={{ width: `${pct}%` }} />
      ) : (
        <div className="h-full w-full animate-pulse rounded-full bg-primary/40" />
      )}
    </div>
  );
}
