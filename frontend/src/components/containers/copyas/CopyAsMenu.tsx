import { ChevronDown, ClipboardCopy, Database, FileCode2, SquareTerminal } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSub,
  DropdownMenuSubContent,
  DropdownMenuSubTrigger,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { useUI } from "@/stores/uiStore";
import { openCopyAs } from "./copyAsStore";
import { useCopyAsInfo } from "./useCopyAs";

interface Props {
  containerId: string;
  name: string;
}

/** The "Copy as" entries; "Connection string" only appears once the container is recognised as a database. */
function CopyAsItems({ containerId, name }: Props) {
  const connectionId = useUI((s) => s.activeConnectionId);
  const info = useCopyAsInfo(connectionId, containerId);
  return (
    <>
      <DropdownMenuItem onSelect={() => openCopyAs("run", containerId, name)}>
        <SquareTerminal />
        docker run command
      </DropdownMenuItem>
      <DropdownMenuItem onSelect={() => openCopyAs("compose", containerId, name)}>
        <FileCode2 />
        Compose service
      </DropdownMenuItem>
      {info.data?.connectionKind && (
        <DropdownMenuItem onSelect={() => openCopyAs("connection", containerId, name)}>
          <Database />
          Connection string
        </DropdownMenuItem>
      )}
    </>
  );
}

/** Submenu for the container row menu and the details "…" menu. */
export function CopyAsSubmenu(p: Props) {
  return (
    <DropdownMenuSub>
      <DropdownMenuSubTrigger>
        <ClipboardCopy />
        Copy as
      </DropdownMenuSubTrigger>
      <DropdownMenuSubContent>
        <CopyAsItems {...p} />
      </DropdownMenuSubContent>
    </DropdownMenuSub>
  );
}

/** Small "Copy as" button for the Overview panel. */
export function CopyAsButton(p: Props) {
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="outline" size="sm" className="h-7 gap-1 px-2 text-xs">
          <ClipboardCopy className="size-3.5" />
          Copy as
          <ChevronDown className="size-3.5" />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        <CopyAsItems {...p} />
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
