import { Copy, ExternalLink, Eye, MoreHorizontal, Pause, Play, RotateCw, Skull, Square, Trash2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { type ContainerAction, ineligibleReason } from "@/lib/actionEligibility";

export interface ContainerActionMenuProps {
  name: string;
  state: string;
  id: string;
  busy?: boolean;
  onAction: (action: ContainerAction) => void;
  onInspect?: () => void;
  onOpenDetails?: () => void;
  onCopyId: () => void;
}

/** Eligibility-aware per-container overflow menu. Disabled items explain why via title. */
export function ContainerActionMenu({ name, state, busy, onAction, onInspect, onOpenDetails, onCopyId }: ContainerActionMenuProps) {
  const item = (action: ContainerAction, label: string, icon: React.ReactNode, destructive = false) => {
    const reason = busy ? "Another action is in progress" : ineligibleReason(action, state);
    return (
      <DropdownMenuItem
        disabled={!!reason}
        title={reason}
        variant={destructive ? "destructive" : "default"}
        onSelect={() => onAction(action)}
      >
        {icon}
        {label}
      </DropdownMenuItem>
    );
  };
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button
          variant="ghost"
          size="icon"
          className="size-7"
          aria-label={`Actions for ${name}`}
          onClick={(e) => e.stopPropagation()}
          onDoubleClick={(e) => e.stopPropagation()}
        >
          <MoreHorizontal className="size-4" />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" onClick={(e) => e.stopPropagation()}>
        {item("start", "Start", <Play />)}
        {item("stop", "Stop", <Square />)}
        {item("restart", "Restart", <RotateCw />)}
        {state === "paused" ? item("unpause", "Unpause", <Play />) : item("pause", "Pause", <Pause />)}
        {item("kill", "Kill…", <Skull />, true)}
        <DropdownMenuSeparator />
        {onOpenDetails && (
          <DropdownMenuItem onSelect={onOpenDetails}>
            <ExternalLink />
            Open details
          </DropdownMenuItem>
        )}
        {onInspect && (
          <DropdownMenuItem onSelect={onInspect}>
            <Eye />
            Inspect
          </DropdownMenuItem>
        )}
        <DropdownMenuItem onSelect={onCopyId}>
          <Copy />
          Copy ID
        </DropdownMenuItem>
        <DropdownMenuSeparator />
        {item("remove", "Delete…", <Trash2 />, true)}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
