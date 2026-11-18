import { useEffect, useState } from "react";
import { OperationStatus, type Operation } from "@bindings/model";
import { errorMessage } from "@/lib/errors";
import { BuildStepList } from "./BuildStepList";
import { summaryLine } from "./buildFormat";
import { useBuildProgress } from "./useBuildApi";

/** "5 of 9 steps · 2 cached · 41 s" for a running or finished build operation. */
export function BuildSummary({ op, done, total, cached }: { op: Operation; done: number; total: number; cached: number }) {
  const running = op.status === OperationStatus.OpRunning;
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    if (!running) return;
    const t = setInterval(() => setNow(Date.now()), 1000);
    return () => clearInterval(t);
  }, [running]);
  const end = running ? now : op.finishedAt || now;
  return <span className="tabular-nums">{summaryLine(done, total, cached, Math.max(0, end - op.startedAt))}</span>;
}

/** Step list of one build operation started in this session (polls while it runs). */
export function BuildProgressView({ op }: { op: Operation }) {
  const running = op.status === OperationStatus.OpRunning;
  const q = useBuildProgress(op.id, true);
  if (q.isError) {
    return (
      <p className="py-1 text-xs text-muted-foreground">
        No step list for this build: it used the Classic builder (Engine API) or ran before this session. Its output is in the log.
        <span className="sr-only">{errorMessage(q.error)}</span>
      </p>
    );
  }
  if (!q.data) return <p className="py-1 text-xs text-muted-foreground">Loading steps…</p>;
  const p = q.data;
  return (
    <div>
      <div className="mb-1 flex items-center justify-between text-xs text-muted-foreground">
        <BuildSummary op={op} done={p.done} total={p.total} cached={p.cached} />
        {p.plain && <span title="The CLI printed plain text instead of structured progress; steps are grouped by their # number.">Plain progress (best effort)</span>}
      </div>
      <BuildStepList steps={p.steps ?? []} live={running} />
    </div>
  );
}
