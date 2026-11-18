import { toast } from "sonner";
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
import { useForgetComposeProject } from "@/hooks/useCompose";
import { errorMessage } from "@/lib/errors";

interface Props {
  connectionId: string;
  project: string;
  files: readonly string[];
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onForgotten?: () => void;
}

/** Forgetting a managed project only removes Docklet's saved record. */
export function ForgetProjectDialog({ connectionId, project, files, open, onOpenChange, onForgotten }: Props) {
  const forget = useForgetComposeProject(connectionId);
  return (
    <AlertDialog open={open} onOpenChange={onOpenChange}>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>Forget project {project}?</AlertDialogTitle>
          <AlertDialogDescription>
            Docklet will forget the saved Compose file location. Nothing is stopped or removed: containers, networks, volumes and your files stay as they are. If containers of this project exist, it remains listed as a project discovered from labels.
          </AlertDialogDescription>
        </AlertDialogHeader>
        {files.length > 0 && (
          <ul className="max-h-32 space-y-0.5 overflow-auto font-mono text-xs text-muted-foreground">
            {files.map((f) => (
              <li key={f}>{f}</li>
            ))}
          </ul>
        )}
        <AlertDialogFooter>
          <AlertDialogCancel>Cancel</AlertDialogCancel>
          <AlertDialogAction
            onClick={() => {
              forget.mutate(project, {
                onSuccess: () => {
                  toast.success(`Forgot project ${project}`, { description: "Nothing was stopped or removed." });
                  onForgotten?.();
                },
                onError: (e) =>
                  toast.error(`Could not forget ${project}`, { description: errorMessage(e), duration: Infinity, closeButton: true }),
              });
            }}
          >
            Forget project
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}
