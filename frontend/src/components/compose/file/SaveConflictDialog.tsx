import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { Button } from "@/components/ui/button";

interface Props {
  open: boolean;
  onReload: () => void;
  onOverwrite: () => void;
  onCancel: () => void;
}

export function SaveConflictDialog({ open, onReload, onOverwrite, onCancel }: Props) {
  return (
    <AlertDialog open={open} onOpenChange={(o) => !o && onCancel()}>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>The file changed on disk since it was opened.</AlertDialogTitle>
          <AlertDialogDescription>
            Reload to read the new version and lose your edits, or overwrite to replace the file with your text.
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>Cancel</AlertDialogCancel>
          <Button variant="outline" onClick={onReload}>
            Reload
          </Button>
          <AlertDialogAction onClick={onOverwrite}>Overwrite</AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}
