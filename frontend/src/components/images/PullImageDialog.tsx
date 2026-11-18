import { useEffect, useState } from "react";
import { Loader2 } from "lucide-react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { useOperations } from "@/hooks/useOperations";
import { useSignInDialog } from "@/hooks/useRegistries";
import { usePullImage, useRegistryAuthStatus } from "@/hooks/useImages";
import { quickRefProblem } from "./imageFilters";
import { describeSource } from "./PushImageDialog";
import { errorMessage } from "@/lib/errors";

interface Props {
  connectionId: string;
  engineName: string;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

export function PullImageDialog({ connectionId, engineName, open, onOpenChange }: Props) {
  const [ref, setRef] = useState("");
  const [touched, setTouched] = useState(false);
  const [serverError, setServerError] = useState<string | null>(null);
  const pull = usePullImage(connectionId);
  const { setCenterOpen } = useOperations();
  const openSignIn = useSignInDialog((s) => s.open);

  useEffect(() => {
    if (open) {
      setRef("");
      setTouched(false);
      setServerError(null);
    }
  }, [open]);

  const problem = quickRefProblem(ref);
  const [debounced, setDebounced] = useState("");
  useEffect(() => {
    const t = setTimeout(() => setDebounced(ref.trim()), 350);
    return () => clearTimeout(t);
  }, [ref]);
  const auth = useRegistryAuthStatus(connectionId, open && !quickRefProblem(debounced) ? debounced : null);
  const shown = serverError ?? (touched ? problem : null);

  const submit = async () => {
    setTouched(true);
    if (problem) return;
    setServerError(null);
    try {
      const op = await pull.mutateAsync(ref.trim());
      onOpenChange(false);
      toast.success(`Pull started: ${op.resource}`, {
        description: "Progress is shown in the Activity center.",
        action: { label: "View", onClick: () => setCenterOpen(true) },
      });
    } catch (e) {
      setServerError(errorMessage(e));
    }
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>Pull image</DialogTitle>
          <DialogDescription>
            Pull onto <span className="font-medium text-foreground">{engineName}</span>. Closing this dialog does not cancel the pull.
          </DialogDescription>
        </DialogHeader>
        <form
          onSubmit={(e) => {
            e.preventDefault();
            void submit();
          }}
          className="space-y-2"
        >
          <label htmlFor="pull-ref" className="text-xs font-medium text-muted-foreground">
            Image reference
          </label>
          <Input
            id="pull-ref"
            autoFocus
            value={ref}
            onChange={(e) => {
              setRef(e.target.value);
              setServerError(null);
            }}
            onBlur={() => setTouched(true)}
            placeholder="nginx:1.27  or  ghcr.io/org/app:tag"
            className="font-mono"
            aria-invalid={!!shown}
            aria-describedby="pull-ref-help"
            spellCheck={false}
            autoComplete="off"
          />
          <p id="pull-ref-help" className={shown ? "text-xs text-destructive" : "text-xs text-muted-foreground"}>
            {shown ?? "[registry/]repository[:tag]. Without a tag, latest is pulled."}
          </p>
          {!shown && auth.data && (
            <p className="text-xs text-muted-foreground">
              Registry <span className="font-mono">{auth.data.registry}</span>:{" "}
              {auth.data.hasCredentials ? (
                <span className="text-success">Docker credentials found ({describeSource(auth.data.source)}); they will be used.</span>
              ) : (
                <span>
                  no credentials found; pulling anonymously. For a private registry{" "}
                  <button type="button" className="text-primary underline-offset-2 hover:underline" onClick={() => openSignIn({ registry: auth.data?.registry })}>
                    sign in first
                  </button>
                  .
                </span>
              )}
            </p>
          )}
          <DialogFooter className="pt-2">
            <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
              Cancel
            </Button>
            <Button type="submit" disabled={pull.isPending}>
              {pull.isPending && <Loader2 className="size-4 animate-spin" />}
              Pull
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
