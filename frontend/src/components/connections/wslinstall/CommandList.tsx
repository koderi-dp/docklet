import { useState } from "react";
import { ChevronRight } from "lucide-react";
import type { WSLInstallStep } from "@bindings/model";
import { Button } from "@/components/ui/button";
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from "@/components/ui/collapsible";
import { cn } from "@/lib/utils";

/** Every command the install will run, in order, with why it is needed. Collapsed by default. */
export function CommandList({ steps }: { steps: WSLInstallStep[] }) {
  const [open, setOpen] = useState(false);
  return (
    <Collapsible open={open} onOpenChange={setOpen}>
      <CollapsibleTrigger asChild>
        <Button variant="ghost" size="xs" className="-ml-1.5 text-muted-foreground">
          <ChevronRight className={cn("transition-transform", open && "rotate-90")} />
          {open ? "Hide commands" : "Show commands"} ({steps.length})
        </Button>
      </CollapsibleTrigger>
      <CollapsibleContent>
        <ol className="mt-1 max-h-64 space-y-2 overflow-auto rounded-md border bg-muted/40 p-3">
          {steps.map((s, i) => (
            <li key={`${i}-${s.title}`} className="text-xs">
              <p className="font-medium">
                {i + 1}. {s.title}
              </p>
              <code className="mt-0.5 block font-mono text-[11px] break-all whitespace-pre-wrap text-muted-foreground">{s.command}</code>
              {s.input && (
                <pre className="mt-1 rounded bg-background/60 p-1.5 font-mono text-[11px] whitespace-pre-wrap text-muted-foreground">{s.input}</pre>
              )}
              <p className="mt-0.5 text-muted-foreground">{s.why}</p>
            </li>
          ))}
        </ol>
      </CollapsibleContent>
    </Collapsible>
  );
}
