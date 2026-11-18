import { AlertTriangle, Check, Minus } from "lucide-react";
import { Logo } from "@/components/layout/Logo";
import { Button } from "@/components/ui/button";
import { canGoto, STEP_ORDER, stepStatuses, type OnboardingState, type StepId, type StepStatus } from "@/lib/onboarding";
import { cn } from "@/lib/utils";

const LABELS: Record<StepId, string> = {
  detect: "Look around",
  choose: "Choose an engine",
  setup: "Set up",
  check: "Check connection",
  ready: "Ready",
};

function Marker({ status, n }: { status: StepStatus; n: number }) {
  const base = "flex size-[22px] shrink-0 items-center justify-center rounded-full border text-[11px] font-semibold tabular-nums";
  switch (status) {
    case "active":
      return <span className={cn(base, "border-primary bg-primary text-primary-foreground")}>{n}</span>;
    case "done":
      return (
        <span className={cn(base, "border-success/40 bg-success/15 text-success")}>
          <Check className="size-3" strokeWidth={3} />
        </span>
      );
    case "attention":
      return (
        <span className={cn(base, "border-warning/40 bg-warning/15 text-warning")}>
          <AlertTriangle className="size-3" />
        </span>
      );
    case "skipped":
      return (
        <span className={cn(base, "border-dashed text-muted-foreground")}>
          <Minus className="size-3" />
        </span>
      );
    default:
      return <span className={cn(base, "text-muted-foreground")}>{n}</span>;
  }
}

/** The left rail: where the user is, what each finished step found, and a way back to it. */
export function StepRail({ state, onGoto, onSkip }: { state: OnboardingState; onGoto: (s: StepId) => void; onSkip: () => void }) {
  const status = stepStatuses(state);
  return (
    <aside className="flex w-[248px] shrink-0 flex-col border-r bg-card/50 px-5 py-6" aria-label="Setup steps">
      <div className="flex items-center gap-2.5">
        <Logo className="size-8" />
        <span className="text-base font-semibold tracking-tight">Docklet</span>
      </div>
      <p className="mt-8 mb-3 text-[11px] font-semibold tracking-wider text-muted-foreground uppercase">First-time setup</p>
      <ol className="flex flex-col">
        {STEP_ORDER.map((step, i) => {
          const st = status[step];
          const note = state.results[step]?.text ?? (st === "skipped" ? "Nothing to set up" : undefined);
          const clickable = canGoto(state, step);
          const body = (
            <>
              <Marker status={st} n={i + 1} />
              <span className="min-w-0 text-left">
                <span className={cn("block text-sm", st === "active" ? "font-semibold" : "font-medium", st === "pending" && "text-muted-foreground")}>{LABELS[step]}</span>
                {note && st !== "active" && (
                  <span className={cn("mt-0.5 line-clamp-2 block text-xs break-words", st === "attention" ? "text-warning" : "text-muted-foreground")}>{note}</span>
                )}
              </span>
            </>
          );
          return (
            <li key={step} className="relative pb-5 last:pb-0" aria-current={st === "active" ? "step" : undefined}>
              {i < STEP_ORDER.length - 1 && <span aria-hidden className="absolute top-7 bottom-1 left-[11px] w-px bg-border" />}
              {clickable ? (
                <button
                  type="button"
                  onClick={() => onGoto(step)}
                  className="-m-1 flex w-[calc(100%+0.5rem)] items-start gap-3 rounded-md p-1 outline-none hover:bg-accent/50 focus-visible:ring-[3px] focus-visible:ring-ring/50"
                >
                  {body}
                </button>
              ) : (
                <div className="flex items-start gap-3">{body}</div>
              )}
            </li>
          );
        })}
      </ol>
      <div className="mt-auto pt-6">
        <Button variant="ghost" size="sm" className="-ml-2 text-muted-foreground" onClick={onSkip}>
          Skip setup
        </Button>
      </div>
    </aside>
  );
}
