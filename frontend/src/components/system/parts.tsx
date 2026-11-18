import type { ReactNode } from "react";
import { formatBytes } from "@/components/containers/format";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";

/** "Not available" with an explanatory tooltip; never renders a missing figure as 0. */
export function NotAvailable({ reason }: { reason: string }) {
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <span className="cursor-default text-muted-foreground">Not available</span>
      </TooltipTrigger>
      <TooltipContent>{reason}</TooltipContent>
    </Tooltip>
  );
}

/** Bytes figure from a nullable Engine value. */
export function Bytes({ value, reason = "The Engine did not report this figure" }: { value?: number | null; reason?: string }) {
  if (value == null) return <NotAvailable reason={reason} />;
  return <span className="tabular-nums">{formatBytes(value)}</span>;
}

export function InfoGrid({ items }: { items: { label: string; value: ReactNode }[] }) {
  return (
    <dl className="grid grid-cols-1 gap-x-8 md:grid-cols-2">
      {items.map((it) => (
        <div key={it.label} className="grid grid-cols-[150px_1fr] gap-3 border-b border-border/50 py-2 text-[13px]">
          <dt className="text-muted-foreground">{it.label}</dt>
          <dd className="min-w-0 break-words">{it.value === "" || it.value == null ? "—" : it.value}</dd>
        </div>
      ))}
    </dl>
  );
}

export function Card({ title, subtitle, actions, children, className }: { title: ReactNode; subtitle?: ReactNode; actions?: ReactNode; children: ReactNode; className?: string }) {
  return (
    <section className={`rounded-[10px] border border-border bg-card ${className ?? ""}`}>
      <header className="flex items-center justify-between gap-2 border-b border-border px-4 py-3">
        <div className="min-w-0">
          <h2 className="text-sm font-semibold">{title}</h2>
          {subtitle && <p className="text-xs text-muted-foreground">{subtitle}</p>}
        </div>
        {actions}
      </header>
      <div className="p-4">{children}</div>
    </section>
  );
}

export function formatEventTime(ms: number): string {
  return new Date(ms).toLocaleTimeString(undefined, { hour: "2-digit", minute: "2-digit", second: "2-digit" });
}
