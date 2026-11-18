import { useState } from "react";
import { ChevronRight, Copy } from "lucide-react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from "@/components/ui/collapsible";
import { cn } from "@/lib/utils";

/** Friendly error text with a collapsible, copyable diagnostics section (already redacted by the backend). */
export function ErrorWithDiagnostics({
  message,
  diagnostics,
  className,
}: {
  message?: string;
  diagnostics?: string;
  className?: string;
}) {
  const [open, setOpen] = useState(false);
  const copy = async () => {
    try {
      await navigator.clipboard.writeText([message, diagnostics].filter(Boolean).join("\n\n"));
      toast.success("Diagnostics copied");
    } catch {
      toast.error("Could not copy to the clipboard");
    }
  };
  return (
    <div className={cn("rounded-md border border-destructive/30 bg-destructive/10 p-3 text-sm", className)} role="alert">
      <p className="text-destructive">{message || "The connection failed."}</p>
      {diagnostics && (
        <Collapsible open={open} onOpenChange={setOpen} className="mt-2">
          <div className="flex items-center justify-between gap-2">
            <CollapsibleTrigger asChild>
              <Button variant="ghost" size="xs" className="-ml-1.5 text-muted-foreground">
                <ChevronRight className={cn("transition-transform", open && "rotate-90")} />
                {open ? "Hide diagnostics" : "Show diagnostics"}
              </Button>
            </CollapsibleTrigger>
            <Button variant="ghost" size="xs" onClick={copy} className="text-muted-foreground">
              <Copy />
              Copy
            </Button>
          </div>
          <CollapsibleContent>
            <pre className="mt-1 max-h-40 overflow-auto rounded-md bg-background/60 p-2 font-mono text-[11px] break-words whitespace-pre-wrap text-muted-foreground">
              {diagnostics}
            </pre>
          </CollapsibleContent>
        </Collapsible>
      )}
    </div>
  );
}
