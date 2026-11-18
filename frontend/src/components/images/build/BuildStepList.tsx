import { useEffect, useLayoutEffect, useRef, useState } from "react";
import { Ban, Check, ChevronDown, ChevronRight, Circle, Loader2, X } from "lucide-react";
import type { BuildStep } from "@bindings/model";
import { cn } from "@/lib/utils";
import { formatDuration } from "./buildFormat";

function StepIcon({ state }: { state: string }) {
  switch (state) {
    case "running":
      return <Loader2 className="size-3.5 animate-spin text-info" aria-label="Running" />;
    case "done":
      return <Check className="size-3.5 text-success" aria-label="Done" />;
    case "cached":
      return <Check className="size-3.5 text-muted-foreground" aria-label="Cached" />;
    case "error":
      return <X className="size-3.5 text-destructive" aria-label="Failed" />;
    case "cancelled":
      return <Ban className="size-3.5 text-muted-foreground" aria-label="Cancelled" />;
    default:
      return <Circle className="size-3 text-muted-foreground/60" aria-label="Waiting" />;
  }
}

function useNow(active: boolean): number {
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    if (!active) return;
    const t = setInterval(() => setNow(Date.now()), 1000);
    return () => clearInterval(t);
  }, [active]);
  return now;
}

function StepLog({ lines, error, follow }: { lines: string[]; error?: string; follow: boolean }) {
  const ref = useRef<HTMLDivElement>(null);
  useLayoutEffect(() => {
    const el = ref.current;
    if (el && follow) el.scrollTop = el.scrollHeight;
  }, [lines, follow]);
  return (
    <div
      ref={ref}
      tabIndex={0}
      aria-label="Step output"
      className="ml-[22px] mt-0.5 mb-1.5 max-h-44 overflow-auto rounded-md border border-border bg-muted/40 p-2 font-mono text-[11px] leading-snug"
    >
      {lines.length === 0 && !error && <span className="text-muted-foreground">No output.</span>}
      {lines.map((l, i) => (
        <div key={i} className="whitespace-pre-wrap break-all">
          {l}
        </div>
      ))}
      {error && <div className="mt-1 whitespace-pre-wrap break-words text-destructive">{error}</div>}
    </div>
  );
}

function StepRow({ step, now, open, onToggle }: { step: BuildStep; now: number; open: boolean; onToggle: () => void }) {
  const running = step.state === "running";
  const hasDetail = (step.log?.length ?? 0) > 0 || !!step.error;
  const right =
    step.state === "cached" ? (
      <span className="text-[11px] font-medium tracking-wide text-muted-foreground">CACHED</span>
    ) : running ? (
      <span className="tabular-nums text-muted-foreground">{step.startedAt ? formatDuration(now - step.startedAt) : ""}</span>
    ) : step.state === "done" || step.state === "error" ? (
      <span className="tabular-nums text-muted-foreground">{step.durationMs > 0 ? formatDuration(step.durationMs) : ""}</span>
    ) : step.state === "cancelled" ? (
      <span className="text-xs text-muted-foreground">Cancelled</span>
    ) : null;
  return (
    <li>
      <button
        type="button"
        disabled={!hasDetail}
        aria-expanded={hasDetail ? open : undefined}
        onClick={onToggle}
        className={cn(
          "flex h-7 w-full items-center gap-2 rounded px-1 text-left text-[13px] enabled:hover:bg-muted/50",
          step.internal && "text-muted-foreground",
          step.state === "error" && "text-destructive",
        )}
      >
        <span className="flex w-3.5 shrink-0 justify-center">
          <StepIcon state={step.state} />
        </span>
        <span className="min-w-0 flex-1 truncate font-mono text-xs" title={step.name}>
          {step.name}
        </span>
        {hasDetail && (open ? <ChevronDown className="size-3.5 shrink-0 text-muted-foreground" /> : <ChevronRight className="size-3.5 shrink-0 text-muted-foreground" />)}
        <span className="w-16 shrink-0 text-right text-xs">{right}</span>
      </button>
      {open && hasDetail && <StepLog lines={step.log ?? []} error={step.state === "error" ? step.error : undefined} follow={running} />}
    </li>
  );
}

/**
 * Step list of a build. The running step and a failed step are expanded by default (with their output);
 * every other step opens on click. Setup and export steps sit in a group below the Dockerfile steps.
 */
export function BuildStepList({ steps, live }: { steps: BuildStep[]; live: boolean }) {
  const now = useNow(live);
  const [toggled, setToggled] = useState<Record<string, boolean>>({});
  const [showInternal, setShowInternal] = useState(false);
  const main = steps.filter((s) => !s.internal);
  const internal = steps.filter((s) => s.internal);
  const defaultOpen = (s: BuildStep) => s.state === "running" || s.state === "error";
  const isOpen = (s: BuildStep) => toggled[s.id] ?? defaultOpen(s);
  const toggle = (s: BuildStep) => setToggled((t) => ({ ...t, [s.id]: !isOpen(s) }));
  const internalFailed = internal.some((s) => s.state === "error");
  const internalOpen = showInternal || internalFailed;
  return (
    <div>
      {main.length === 0 && internal.length === 0 ? (
        <p className="py-2 text-xs text-muted-foreground">{live ? "Waiting for the first step…" : "No steps were recorded."}</p>
      ) : (
        <ul>
          {main.map((s) => (
            <StepRow key={s.id} step={s} now={now} open={isOpen(s)} onToggle={() => toggle(s)} />
          ))}
        </ul>
      )}
      {internal.length > 0 && (
        <div className="mt-1 border-t border-border/60 pt-1">
          <button
            type="button"
            aria-expanded={internalOpen}
            onClick={() => setShowInternal((v) => !v)}
            className="inline-flex h-6 items-center gap-1 text-xs text-muted-foreground hover:text-foreground"
          >
            {internalOpen ? <ChevronDown className="size-3.5" /> : <ChevronRight className="size-3.5" />}
            Setup and export steps ({internal.length})
          </button>
          {internalOpen && (
            <ul>
              {internal.map((s) => (
                <StepRow key={s.id} step={s} now={now} open={isOpen(s)} onToggle={() => toggle(s)} />
              ))}
            </ul>
          )}
        </div>
      )}
    </div>
  );
}
