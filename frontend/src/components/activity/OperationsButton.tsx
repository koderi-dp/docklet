import { ListChecks } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { useOperations } from "@/hooks/useOperations";
import { OperationsPanel } from "./OperationsPanel";

/** Top-bar button with the running-operation count; opens the Operations side panel. */
export default function OperationsButton() {
  const { runningCount, openOperations } = useOperations();
  return (
    <>
      <Button
        variant="ghost"
        size="icon"
        className="relative"
        title="Operations"
        aria-label={runningCount > 0 ? `Operations: ${runningCount} running` : "Operations"}
        onClick={() => openOperations()}
      >
        <ListChecks className="size-4" />
        {runningCount > 0 && (
          <Badge className="absolute -right-0.5 -top-0.5 h-4 min-w-4 justify-center rounded-full px-1 text-[10px] leading-none">{runningCount}</Badge>
        )}
      </Button>
      <OperationsPanel />
    </>
  );
}
