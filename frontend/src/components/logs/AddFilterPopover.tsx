import { useRef, useState } from "react";
import { Plus } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import type { Rule } from "@/lib/logFilter/rules";
import { RuleForm } from "./RuleForm";
import { KIND_LABEL, TARGET_LABEL } from "./ruleLabels";

interface Props {
  rules: readonly Rule[];
  addRule: (r: Omit<Rule, "id" | "enabled">) => void;
  /** Containers whose lines are in the view: offered as suggestions for the Container target. Empty: no such target. */
  containerNames?: readonly string[];
}

/** "+ Filter": builds one explicit filter rule (kind, what it applies to, pattern) and adds it as a chip. Stays open for the next. */
export function AddFilterPopover({ rules, addRule, containerNames = [] }: Props) {
  const [open, setOpen] = useState(false);
  const [note, setNote] = useState<string | null>(null);
  const input = useRef<HTMLInputElement>(null);

  return (
    <Popover
      open={open}
      onOpenChange={(o) => {
        setOpen(o);
        if (!o) setNote(null);
      }}
    >
      <PopoverTrigger asChild>
        <Button variant="outline" size="sm" className="shrink-0 gap-1.5" title="Add a filter that hides or highlights lines">
          <Plus className="size-3.5" /> Filter
        </Button>
      </PopoverTrigger>
      <PopoverContent
        align="end"
        className="w-[340px] p-3"
        onOpenAutoFocus={(e) => {
          e.preventDefault();
          input.current?.focus();
        }}
      >
        <RuleForm
          keepOpen
          submitLabel="Add filter"
          rules={rules}
          containerNames={containerNames}
          note={note}
          inputRef={input}
          onSubmit={(r) => {
            addRule(r);
            setNote(`Added: ${KIND_LABEL[r.kind]} ${TARGET_LABEL[r.target].toLowerCase()} ${r.pattern}`);
          }}
        />
      </PopoverContent>
    </Popover>
  );
}
