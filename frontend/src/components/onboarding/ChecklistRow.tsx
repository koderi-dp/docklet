import type { ReactNode } from "react";
import { Check, Loader2, Minus, TriangleAlert } from "lucide-react";
import type { DetectItem, RowTone } from "@/lib/onboardingDetect";
import { cn } from "@/lib/utils";

const TONE_TEXT: Record<RowTone, string> = { ok: "text-success", warn: "text-warning", none: "text-muted-foreground" };

function Icon({ tone, pending }: { tone?: RowTone; pending?: boolean }) {
  const box = "mt-px flex size-5 shrink-0 items-center justify-center rounded-full";
  if (pending) return <span className={cn(box, "text-muted-foreground")}><Loader2 className="size-4 animate-spin" aria-hidden /></span>;
  if (tone === "ok") return <span className={cn(box, "bg-success/15 text-success")}><Check className="size-3.5" strokeWidth={3} aria-hidden /></span>;
  if (tone === "warn") return <span className={cn(box, "bg-warning/15 text-warning")}><TriangleAlert className="size-3" aria-hidden /></span>;
  return <span className={cn(box, "bg-muted text-muted-foreground")}><Minus className="size-3.5" aria-hidden /></span>;
}

interface Props {
  title: string;
  detail?: string;
  tone?: RowTone;
  pending?: boolean;
  items?: DetectItem[];
  children?: ReactNode;
}

/** One line of the detection checklist: what was looked at and what was found, nothing more. */
export function ChecklistRow({ title, detail, tone, pending, items }: Props) {
  return (
    <li className="flex gap-3 px-4 py-3">
      <Icon tone={tone} pending={pending} />
      <div className="min-w-0 flex-1">
        <p className="text-sm font-medium">{title}</p>
        <p className="mt-0.5 text-xs break-words text-muted-foreground">{pending ? "Checking…" : detail}</p>
        {!pending && items && items.length > 0 && (
          <ul className="mt-2 space-y-1.5 border-l pl-3">
            {items.map((it) => (
              <li key={it.label} className="text-xs">
                <span className="font-medium text-foreground">{it.label}</span>
                <span className={cn("block break-words", TONE_TEXT[it.tone])}>{it.detail}</span>
              </li>
            ))}
          </ul>
        )}
      </div>
    </li>
  );
}
