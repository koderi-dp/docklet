import type { ReactNode } from "react";
import { cn } from "@/lib/utils";

type Tone = "primary" | "success" | "muted" | "info" | "warning";

const TONE: Record<Tone, string> = {
  primary: "bg-primary/15 text-primary",
  success: "bg-success/15 text-success",
  muted: "bg-muted text-muted-foreground",
  info: "bg-info/15 text-info",
  warning: "bg-warning/15 text-warning",
};

export interface MetricCardProps {
  icon: ReactNode;
  label: string;
  value: ReactNode;
  /** Secondary line, e.g. "83% of total". Dropped (not the icon) when the card is narrow. */
  hint?: ReactNode;
  tone?: Tone;
  className?: string;
}

/** Compact KPI card (~80px high): icon chip left, label + value + hint. */
export function MetricCard({ icon, label, value, hint, tone = "primary", className }: MetricCardProps) {
  return (
    <div className={cn("@container flex h-20 min-w-0 items-center gap-3 rounded-[10px] border border-border bg-card px-3.5", className)}>
      <div className={cn("flex size-9 shrink-0 items-center justify-center rounded-lg [&_svg]:size-[18px]", TONE[tone])}>{icon}</div>
      <div className="min-w-0 flex-1">
        <div className="truncate text-xs text-muted-foreground" title={label}>{label}</div>
        <div className="text-2xl font-semibold leading-7 tabular-nums">{value}</div>
        {hint != null && (
          <div className="hidden truncate text-[11px] leading-4 text-muted-foreground @[130px]:block" title={typeof hint === "string" ? hint : undefined}>
            {hint}
          </div>
        )}
      </div>
    </div>
  );
}
