import { SlidersHorizontal, Wrench } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { debugShellDisabledReason } from "./debugShell";

/** Terminal toolbar button; disabled with the reason when a debug shell cannot work. */
export function DebugShellButton({
  state,
  engineOS,
  showOptions,
  onOpen,
  onOptions,
}: {
  state: string;
  engineOS?: string;
  /** The confirm panel is skipped, so offer a way back to its options. */
  showOptions: boolean;
  onOpen: () => void;
  onOptions: () => void;
}) {
  const reason = debugShellDisabledReason(state, engineOS);
  const button = (
    <Button size="sm" variant="outline" disabled={!!reason} onClick={onOpen}>
      <Wrench className="size-4" /> Debug shell
    </Button>
  );
  return (
    <span className="inline-flex items-center gap-1">
      {reason ? (
        <Tooltip>
          <TooltipTrigger asChild>
            <span className="inline-flex">{button}</span>
          </TooltipTrigger>
          <TooltipContent>{reason}</TooltipContent>
        </Tooltip>
      ) : (
        <Tooltip>
          <TooltipTrigger asChild>{button}</TooltipTrigger>
          <TooltipContent>Open a temporary toolbox that shares this container&apos;s process and network namespaces</TooltipContent>
        </Tooltip>
      )}
      {showOptions && !reason && (
        <Button size="icon" variant="ghost" className="size-8" aria-label="Debug shell options" title="Debug shell options" onClick={onOptions}>
          <SlidersHorizontal className="size-4" />
        </Button>
      )}
    </span>
  );
}
