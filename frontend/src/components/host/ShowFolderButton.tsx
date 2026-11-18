import { FolderOpen } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { useHostActions, useLocalPath } from "@/hooks/useHost";

/** Small icon button that shows a bind mount's host folder in the file manager; absent unless the path is on this computer. */
export function ShowFolderButton({ connectionId, path }: { connectionId: string; path: string }) {
  const { exists } = useLocalPath(connectionId, path);
  const { showFolder } = useHostActions(connectionId);
  if (!exists) return null;
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <Button variant="ghost" size="icon" className="size-5 shrink-0 text-muted-foreground" aria-label="Show folder" onClick={() => void showFolder(path)}>
          <FolderOpen className="size-3.5" />
        </Button>
      </TooltipTrigger>
      <TooltipContent>Show folder</TooltipContent>
    </Tooltip>
  );
}
