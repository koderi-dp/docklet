import type { ComponentProps } from "react";
import { Button } from "@/components/ui/button";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { cn } from "@/lib/utils";

type Props = Omit<ComponentProps<typeof Button>, "size" | "aria-label"> & {
  /** Used for the tooltip AND aria-label. */
  label: string;
  /** Shown instead of `label` in the tooltip (e.g. why the control is disabled). */
  tooltip?: string;
};

/** Icon-only button that always has a tooltip + aria-label. Works for disabled buttons too. */
export function IconButton({ label, tooltip, className, children, variant = "ghost", disabled, ...rest }: Props) {
  const btn = (
    <Button size="icon-sm" variant={variant} aria-label={label} disabled={disabled} className={cn("size-8", className)} {...rest}>
      {children}
    </Button>
  );
  return (
    <Tooltip>
      <TooltipTrigger asChild>{disabled ? <span className="inline-flex">{btn}</span> : btn}</TooltipTrigger>
      <TooltipContent>{tooltip ?? label}</TooltipContent>
    </Tooltip>
  );
}
