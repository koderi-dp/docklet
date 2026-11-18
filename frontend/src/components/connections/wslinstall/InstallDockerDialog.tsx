import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { InstallDockerFlow, type SuccessAction } from "./InstallDockerFlow";

export type { SuccessAction };

interface Props {
  distro: string;
  /** Attach to an install that is already running (the dialog was closed and reopened). */
  opId?: string;
  onClose: () => void;
  /** The primary button once Docker is running, for example "Test & Save". */
  successAction?: SuccessAction;
}

/** The Docker Engine install flow in a dialog. Closing it never cancels the install. */
export function InstallDockerDialog({ distro, opId, onClose, successAction }: Props) {
  return (
    <Dialog open onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="max-h-[90vh] gap-4 overflow-y-auto sm:max-w-[600px]">
        <InstallDockerFlow
          distro={distro}
          opId={opId}
          onClose={onClose}
          successAction={successAction}
          className="gap-4"
          header={({ title, description }) => (
            <DialogHeader>
              <DialogTitle>{title}</DialogTitle>
              <DialogDescription>{description}</DialogDescription>
            </DialogHeader>
          )}
        />
      </DialogContent>
    </Dialog>
  );
}
