import { Columns2, Grid2x2, Rows2, Square } from "lucide-react";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { cn } from "@/lib/utils";
import type { LayoutId } from "@/lib/logPanes";

const OPTIONS: Array<{ id: LayoutId; label: string; hint: string; Icon: typeof Square }> = [
  { id: "single", label: "Single", hint: "One merged stream", Icon: Square },
  { id: "cols", label: "Two columns", hint: "Two panes side by side", Icon: Columns2 },
  { id: "rows", label: "Two rows", hint: "Two panes one above the other", Icon: Rows2 },
  { id: "grid", label: "Grid of four", hint: "Four panes in a 2 × 2 grid", Icon: Grid2x2 },
];

interface Props {
  layout: LayoutId;
  onChange: (l: LayoutId) => void;
  /** The window is too narrow for side-by-side panes, so columns and the grid are stacked. */
  narrow: boolean;
}

/** The one layout control of the Logs toolbar. */
export function LayoutSwitch({ layout, onChange, narrow }: Props) {
  return (
    <div className="flex shrink-0 items-center overflow-hidden rounded-md border border-border" role="group" aria-label="Log layout">
      {OPTIONS.map(({ id, label, hint, Icon }) => {
        const stacked = narrow && (id === "cols" || id === "grid");
        return (
          <Tooltip key={id}>
            <TooltipTrigger asChild>
              <button
                type="button"
                aria-label={label}
                aria-pressed={layout === id}
                onClick={() => onChange(id)}
                className={cn(
                  "flex h-[30px] w-8 items-center justify-center border-r border-border last:border-r-0 focus-visible:ring-2 focus-visible:ring-ring/60 focus-visible:outline-none focus-visible:ring-inset",
                  layout === id ? "bg-primary/10 text-foreground" : "text-muted-foreground hover:bg-muted/60",
                )}
              >
                <Icon className="size-3.5" />
              </button>
            </TooltipTrigger>
            <TooltipContent>
              {label}
              <span className="block opacity-70">{hint}</span>
              {stacked && <span className="block opacity-70">The window is narrow, so the panes are stacked.</span>}
            </TooltipContent>
          </Tooltip>
        );
      })}
    </div>
  );
}
