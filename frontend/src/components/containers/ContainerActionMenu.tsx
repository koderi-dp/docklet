import { Copy, ExternalLink, Eye, FolderOpen, MoreHorizontal, Pause, Pencil, Play, RotateCw, Skull, Square, SquareTerminal, Trash2, Wrench } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { type ContainerAction, ineligibleReason } from "@/lib/actionEligibility";
import { externalTerminalReason, useOpenExternalTerminal } from "@/components/containers/externalTerminal";
import { useActiveConnection } from "@/hooks/useActiveConnection";
import { useHostActions, useLocalPath } from "@/hooks/useHost";
import { useNavigate } from "react-router-dom";
import { debugShellDisabledReason } from "@/components/containers/debugShell/debugShell";
import { openRecreate } from "@/components/containers/recreate/recreateStore";

export interface ContainerActionMenuProps {
  name: string;
  state: string;
  id: string;
  busy?: boolean;
  onAction: (action: ContainerAction) => void;
  onInspect?: () => void;
  onOpenDetails?: () => void;
  onCopyId: () => void;
  /** Compose working directory of the container's project (engine path), when it has one. */
  projectDir?: string;
}

/** Eligibility-aware per-container overflow menu. Disabled items explain why via title. */
export function ContainerActionMenu({ id, name, state, busy, onAction, onInspect, onOpenDetails, onCopyId, projectDir }: ContainerActionMenuProps) {
  const openExternal = useOpenExternalTerminal();
  const { connectionId, status } = useActiveConnection();
  const navigate = useNavigate();
  const debugReason = debugShellDisabledReason(state, status?.os);
  const { exists: projectDirHere } = useLocalPath(connectionId, projectDir);
  const { showFolder } = useHostActions(connectionId);
  const externalReason = externalTerminalReason(state);
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
        <DropdownMenuItem disabled={!!externalReason} title={externalReason} onSelect={() => void openExternal(id)}>
          <SquareTerminal />
          Open in external terminal
        </DropdownMenuItem>
        <DropdownMenuItem disabled={!!debugReason} title={debugReason} onSelect={() => navigate(`/containers/${id}?tab=terminal&debug=1`)}>
          <Wrench />
          Debug shell…
        </DropdownMenuItem>
        {projectDir && projectDirHere && (
          <DropdownMenuItem onSelect={() => void showFolder(projectDir)}>
            <FolderOpen />
            Open project folder
          </DropdownMenuItem>
        )}
        <DropdownMenuItem onSelect={onCopyId}>
          <Copy />
          Copy ID
        </DropdownMenuItem>
        <DropdownMenuItem onSelect={() => openRecreate(id)} disabled={!!busy} title={busy ? "Another action is in progress" : "Change settings and replace the container"}>
          <Pencil />
          Edit & recreate…
        </DropdownMenuItem>
        <DropdownMenuSeparator />
        {item("remove", "Delete…", <Trash2 />, true)}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
