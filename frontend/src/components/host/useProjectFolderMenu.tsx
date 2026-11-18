import { FileText, FolderOpen, SquarePen } from "lucide-react";
import { DropdownMenuItem, DropdownMenuSeparator } from "@/components/ui/dropdown-menu";
import { useHostActions, useLocalPaths } from "@/hooks/useHost";
import { baseName } from "@/lib/hostPath";

/**
 * Menu items for a Compose project's folder and files on this computer, or null when none is
 * reachable (remote engine, folder missing). Existence is resolved up front so the menu does not
 * change shape while it is open.
 */
export function useProjectFolderMenu(connectionId: string, workingDir: string | undefined, configFiles: readonly string[]) {
  const files = [...configFiles];
  const found = useLocalPaths(connectionId, [workingDir ?? "", ...files]);
  const { showFolder, openInEditor } = useHostActions(connectionId);
  const dirOk = !!workingDir && found[0]?.exists;
  const reachableFiles = files.filter((_, i) => found[i + 1]?.exists);
  if (!dirOk && reachableFiles.length === 0) return null;
  return (
    <>
      <DropdownMenuSeparator />
      {dirOk && (
        <>
          <DropdownMenuItem onSelect={() => void openInEditor(workingDir)}>
            <SquarePen /> Open in editor
          </DropdownMenuItem>
          <DropdownMenuItem onSelect={() => void showFolder(workingDir)}>
            <FolderOpen /> Show folder
          </DropdownMenuItem>
        </>
      )}
      {reachableFiles.map((f) => (
        <DropdownMenuItem key={f} title={f} onSelect={() => void openInEditor(f)}>
          <FileText /> {reachableFiles.length === 1 ? "Open compose file" : `Open ${baseName(f)}`}
        </DropdownMenuItem>
      ))}
    </>
  );
}
