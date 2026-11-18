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

interface Props {
  open: boolean;
  /** Names of the variables whose real values would be copied. */
  names: string[];
  what: string;
  onCancel: () => void;
  onConfirm: () => void;
}

/** Confirmation before real secret values leave the app through the clipboard. */
export function SecretConfirm({ open, names, what, onCancel, onConfirm }: Props) {
  return (
    <AlertDialog open={open} onOpenChange={(o) => !o && onCancel()}>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>Copy with secret values?</AlertDialogTitle>
          <AlertDialogDescription asChild>
            <div className="space-y-2">
              <p>The copied {what} will contain the real value of:</p>
              <ul className="flex flex-wrap gap-1.5 font-mono text-xs text-foreground">
                {names.map((n) => (
                  <li key={n} className="rounded bg-muted px-1.5 py-0.5">{n}</li>
                ))}
              </ul>
              <p>Anyone who can read your clipboard, or the place you paste it into, will see them.</p>
            </div>
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>Cancel</AlertDialogCancel>
          <AlertDialogAction variant="destructive" onClick={onConfirm}>Copy with secret values</AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}
