import { useCallback } from "react";
import { toast } from "sonner";
import { ExternalLink } from "lucide-react";
import { ExecService } from "@bindings/services";
import { Button } from "@/components/ui/button";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { useActiveConnection } from "@/hooks/useActiveConnection";
import { toastError } from "@/lib/errors";
import { useUI } from "@/stores/uiStore";

/** Why "Open in external terminal" is unavailable for a container state; undefined when it works. */
export function externalTerminalReason(state: string): string | undefined {
  return state === "running" ? undefined : "The container is not running. Start it first.";
}

/** Opens the user's terminal with `docker exec -it` for the container on the ACTIVE connection. */
export function useOpenExternalTerminal() {
  const { connectionId } = useActiveConnection();
  const terminal = useUI((s) => s.externalTerminal);
  return useCallback(
    async (containerId: string, shell = "auto") => {
      if (!connectionId) return;
      try {
        await ExecService.OpenExternalTerminal(connectionId, containerId, shell, terminal);
        toast.success("Opened in your terminal");
      } catch (e) {
        toastError("Could not open an external terminal", e);
      }
    },
    [connectionId, terminal],
  );
}

/** Toolbar button; disabled with the reason when the container is not running. */
export function ExternalTerminalButton({ containerId, state, shell }: { containerId: string; state: string; shell?: string }) {
  const open = useOpenExternalTerminal();
  const reason = externalTerminalReason(state);
  const button = (
    <Button size="sm" variant="outline" disabled={!!reason} onClick={() => void open(containerId, shell)}>
      <ExternalLink className="size-4" /> Open in external terminal
    </Button>
  );
  if (!reason) return button;
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <span className="inline-flex">{button}</span>
      </TooltipTrigger>
      <TooltipContent>{reason}</TooltipContent>
    </Tooltip>
  );
}
