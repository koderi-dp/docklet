import { useEffect, useRef } from "react";
import { Copy } from "lucide-react";
import { toast } from "sonner";
import { OperationStatus, type Operation } from "@bindings/model";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { currentStep } from "./useWslInstall";

/** The last lines of the operation log, following the end like a terminal. */
export function LogTail({ lines, className }: { lines: string[]; className?: string }) {
  const ref = useRef<HTMLPreElement>(null);
  useEffect(() => {
    if (ref.current) ref.current.scrollTop = ref.current.scrollHeight;
  }, [lines]);
  return (
    <pre
      ref={ref}
      aria-label="Install log"
      className={cn("h-44 overflow-auto rounded-md border bg-muted/40 p-2 font-mono text-[11px] leading-relaxed break-words whitespace-pre-wrap text-muted-foreground", className)}
    >
      {lines.length ? lines.slice(-200).join("\n") : "Waiting for output…"}
    </pre>
  );
}

export function CopyLogButton({ lines }: { lines: string[] }) {
  const copy = async () => {
    try {
      await navigator.clipboard.writeText(lines.join("\n"));
      toast.success("Log copied");
    } catch {
      toast.error("Could not copy to the clipboard");
    }
  };
  return (
    <Button variant="ghost" size="sm" onClick={copy}>
      <Copy />
      Copy log
    </Button>
  );
}

/** Current step, real progress (finished steps of all steps) and the live log of a running install. */
export function InstallProgress({ op, lines }: { op: Operation; lines: string[] }) {
  const step = currentStep(lines);
  const pct = op.progress != null ? Math.round(op.progress * 100) : null;
  const running = op.status === OperationStatus.OpRunning;
  return (
    <div className="space-y-2">
      <div className="flex items-baseline justify-between gap-3 text-sm">
        <span className="font-medium">{step ? step.title : "Starting…"}</span>
        {step && (
          <span className="shrink-0 text-xs text-muted-foreground tabular-nums">
            Step {step.index} of {step.total}
          </span>
        )}
      </div>
      <div
        className="h-1.5 w-full overflow-hidden rounded-full bg-muted"
        role="progressbar"
        aria-label="Install progress"
        aria-valuemin={0}
        aria-valuemax={100}
        aria-valuenow={pct ?? undefined}
      >
        {pct != null ? (
          <div className="h-full rounded-full bg-primary transition-[width]" style={{ width: `${pct}%` }} />
        ) : (
          running && <div className="h-full w-full animate-pulse rounded-full bg-primary/40" />
        )}
      </div>
      <LogTail lines={lines} />
    </div>
  );
}
