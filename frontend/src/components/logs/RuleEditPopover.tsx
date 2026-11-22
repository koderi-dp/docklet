import { useRef, useState, type ReactNode } from "react";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import type { Rule } from "@/lib/logFilter/rules";
import { RuleForm } from "./RuleForm";

interface Props {
  rule: Rule;
  rules: readonly Rule[];
  /** Containers whose lines are in the view: offered as suggestions for the Container target. */
  containerNames?: readonly string[];
  onSave: (patch: Partial<Omit<Rule, "id">>) => void;
  /** The trigger element. */
  children: ReactNode;
}

/** Edits one rule's kind, target, pattern and colour. Saving applies and closes; Escape or an outside click discards. */
export function RuleEditPopover({ rule, rules, containerNames, onSave, children }: Props) {
  const [open, setOpen] = useState(false);
  const input = useRef<HTMLInputElement>(null);

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>{children}</PopoverTrigger>
      <PopoverContent
        align="start"
        className="w-[340px] p-3"
        onOpenAutoFocus={(e) => {
          if (!input.current) return;
          e.preventDefault();
          input.current.focus();
        }}
      >
        {open && (
          <RuleForm
            selectOnFocus
            submitLabel="Save"
            initial={{ kind: rule.kind, target: rule.target, pattern: rule.pattern, colour: rule.colour }}
            rules={rules}
            ignoreId={rule.id}
            containerNames={containerNames}
            inputRef={input}
            onSubmit={(r) => {
              onSave({ kind: r.kind, target: r.target, pattern: r.pattern, colour: r.colour });
              setOpen(false);
            }}
          />
        )}
      </PopoverContent>
    </Popover>
  );
}
