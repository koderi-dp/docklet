import { useRef } from "react";
import { Minus } from "lucide-react";
import { Checkbox } from "@/components/ui/checkbox";
import type { CheckState } from "@/lib/composeGrouping";
import { cn } from "@/lib/utils";

interface Props {
  state: CheckState;
  /** `shift` is true when the click had Shift held (range selection). */
  onToggle: (shift: boolean) => void;
  label: string;
  className?: string;
  disabled?: boolean;
}

/** Checkbox with an indeterminate state. Clicks never bubble to the row (selection != inspector). */
export function TriCheckbox({ state, onToggle, label, className, disabled }: Props) {
  const shiftRef = useRef(false);
  return (
    <span
      className={cn("relative inline-flex", className)}
      onClickCapture={(e) => {
        shiftRef.current = e.shiftKey;
      }}
      onClick={(e) => e.stopPropagation()}
      onDoubleClick={(e) => e.stopPropagation()}
    >
      <Checkbox
        aria-label={label}
        disabled={disabled}
        checked={state === "indeterminate" ? "indeterminate" : state === "checked"}
        onCheckedChange={() => onToggle(shiftRef.current)}
        className="data-[state=indeterminate]:border-primary data-[state=indeterminate]:bg-primary [&[data-state=indeterminate]_svg]:opacity-0"
      />
      {state === "indeterminate" && (
        <Minus aria-hidden className="pointer-events-none absolute inset-0 m-auto size-3 text-primary-foreground" />
      )}
    </span>
  );
}
