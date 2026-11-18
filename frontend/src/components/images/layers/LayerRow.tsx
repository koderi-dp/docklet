import { Copy } from "lucide-react";
import type { ImageLayerStep } from "@bindings/model";
import { cn } from "@/lib/utils";
import { copyToClipboard } from "@/lib/clipboard";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { KIND_CLASS, formatAge, formatShare, kindLabel, stepSize } from "./layerFormat";

/** Shared column template so header and rows align. */
export const LAYER_GRID = "grid grid-cols-[32px_60px_minmax(0,1fr)_76px_132px_72px] items-center gap-3 px-3";

interface Props {
  step: ImageLayerStep;
  expanded: boolean;
  isLargest: boolean;
  onToggle: () => void;
}

export function LayerRow({ step, expanded, isLargest, onToggle }: Props) {
  const empty = step.size <= 0;
  const text = (step.kind === "OTHER" ? step.instruction : step.command) || "—";
  return (
    <div className={cn("border-b border-border/60 last:border-b-0", expanded && "bg-muted/30")}>
      <div
        role="button"
        tabIndex={0}
        aria-expanded={expanded}
        onClick={onToggle}
        onKeyDown={(e) => {
          if (e.key === "Enter" || e.key === " ") {
            e.preventDefault();
            onToggle();
          }
        }}
        className={cn(LAYER_GRID, "h-[42px] cursor-pointer text-[13px] hover:bg-accent/50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring", empty && "text-muted-foreground")}
      >
        <span className="text-xs tabular-nums text-muted-foreground">{step.index}</span>
        <span className={cn("inline-flex h-5 w-fit items-center rounded px-1.5 font-mono text-[11px] font-medium", KIND_CLASS[step.kind] ?? KIND_CLASS.OTHER)}>{kindLabel(step.kind)}</span>
        <span className="flex min-w-0 items-center gap-1.5">
          <Tooltip>
            <TooltipTrigger asChild>
              <span className="min-w-0 flex-1 truncate font-mono text-xs">{text}</span>
            </TooltipTrigger>
            {step.instruction && (
              <TooltipContent side="top" align="start" className="max-w-[520px] break-all font-mono">
                {step.instruction}
              </TooltipContent>
            )}
          </Tooltip>
          {step.instruction && (
            <button
              type="button"
              aria-label={`Copy instruction of step ${step.index}`}
              title="Copy instruction"
              onClick={(e) => {
                e.stopPropagation();
                copyToClipboard(step.instruction, "Instruction");
              }}
              className="shrink-0 text-muted-foreground hover:text-foreground"
            >
              <Copy className="size-3.5" />
            </button>
          )}
        </span>
        <span className={cn("text-right tabular-nums", isLargest && "font-semibold text-foreground")}>{stepSize(step)}</span>
        <span className="flex items-center gap-2" title={empty ? "No size reported for this step" : undefined}>
          <span className="h-1.5 flex-1 overflow-hidden rounded-full bg-muted">
            {!empty && <span className={cn("block h-full rounded-full", isLargest ? "bg-warning" : "bg-primary")} style={{ width: `${Math.max(2, step.share * 100)}%` }} />}
          </span>
          <span className="w-8 text-right text-xs tabular-nums text-muted-foreground">{formatShare(step.share)}</span>
        </span>
        <span className="text-right text-xs text-muted-foreground">{formatAge(step.created)}</span>
      </div>
      {expanded && (
        <div className="px-3 pb-3 pl-[56px]">
          <pre className="whitespace-pre-wrap break-all rounded-md border border-border bg-background p-2 font-mono text-xs">{step.instruction || "No instruction recorded for this step."}</pre>
          {step.id && step.id !== "<missing>" && <p className="mt-1 font-mono text-[11px] text-muted-foreground">Layer image ID {step.id.replace("sha256:", "").slice(0, 12)}</p>}
        </div>
      )}
    </div>
  );
}
