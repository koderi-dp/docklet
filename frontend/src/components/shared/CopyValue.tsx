import { Fragment, type ReactNode } from "react";
import { Copy } from "lucide-react";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { cn } from "@/lib/utils";
import { copyToClipboard } from "@/lib/clipboard";

/** Path text that wraps after "/" (and only mid-segment as a last resort). */
export function PathText({ value }: { value: string }) {
  const parts = value.split("/");
  return (
    <>
      {parts.map((p, i) => (
        <Fragment key={i}>
          {p}
          {i < parts.length - 1 && <>/<wbr /></>}
        </Fragment>
      ))}
    </>
  );
}

/** Single-line truncated value with full-text tooltip and a copy button. */
export function CopyValue({
  value,
  what,
  mono = true,
  className,
  children,
}: {
  value: string;
  what: string;
  mono?: boolean;
  className?: string;
  children?: ReactNode;
}) {
  return (
    <span className={cn("group/copy inline-flex max-w-full min-w-0 items-center gap-1", className)}>
      <Tooltip>
        <TooltipTrigger asChild>
          <span className={cn("min-w-0 truncate", mono && "font-mono text-xs")}>{children ?? value}</span>
        </TooltipTrigger>
        <TooltipContent className="max-w-sm break-all font-mono text-xs">{value}</TooltipContent>
      </Tooltip>
      <Tooltip>
        <TooltipTrigger asChild>
          <button
            type="button"
            aria-label={`Copy ${what.toLowerCase()}`}
            onClick={() => copyToClipboard(value, what)}
            className="shrink-0 rounded text-muted-foreground outline-none transition-colors duration-100 hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring/60"
          >
            <Copy className="size-3.5" />
          </button>
        </TooltipTrigger>
        <TooltipContent>Copy {what.toLowerCase()}</TooltipContent>
      </Tooltip>
    </span>
  );
}
