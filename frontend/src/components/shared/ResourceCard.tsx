import type { ReactNode } from "react";
import { TriangleAlert } from "lucide-react";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { cn } from "@/lib/utils";

/**
 * Card laid out as a flex column: header, description, optional one-line note, and a footer pinned to the
 * bottom so footers (value + button) line up across a grid row.
 */
export function ResourceCard({
  icon,
  title,
  description,
  note,
  danger,
  footer,
  className,
}: {
  icon?: ReactNode;
  title: ReactNode;
  description?: ReactNode;
  /** Single muted line with a small warning icon (full text in tooltip). */
  note?: string;
  danger?: boolean;
  footer?: ReactNode;
  className?: string;
}) {
  return (
    <section className={cn("flex h-full flex-col rounded-[10px] border border-border bg-card p-4", className)}>
      <h3 className="flex items-center gap-2 text-sm font-semibold [&_svg]:size-4 [&_svg]:shrink-0">{icon}{title}</h3>
      {description && <p className="mt-1 line-clamp-2 text-[13px] text-muted-foreground">{description}</p>}
      {note && (
        <Tooltip>
          <TooltipTrigger asChild>
            <p className="mt-2 flex min-w-0 items-center gap-1.5 text-xs text-muted-foreground">
              <TriangleAlert aria-hidden className={cn("size-3.5 shrink-0", danger ? "text-destructive" : "text-warning")} />
              <span className="truncate">{note}</span>
            </p>
          </TooltipTrigger>
          <TooltipContent className="max-w-sm">{note}</TooltipContent>
        </Tooltip>
      )}
      {footer && <div className="mt-auto flex items-center justify-between gap-2 pt-4 text-[13px]">{footer}</div>}
    </section>
  );
}
