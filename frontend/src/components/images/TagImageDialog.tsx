import { useEffect, useState } from "react";
import { Loader2 } from "lucide-react";
import { toast } from "sonner";
import type { ImageRow } from "@bindings/model";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { useTagImage } from "@/hooks/useImages";
import { quickRefProblem } from "./imageFilters";
import { errorMessage } from "@/lib/errors";

interface Props {
  connectionId: string;
  image: ImageRow | null;
  onClose: () => void;
}

export function TagImageDialog({ connectionId, image, onClose }: Props) {
  const [target, setTarget] = useState("");
  const [touched, setTouched] = useState(false);
  const [serverError, setServerError] = useState<string | null>(null);
  const tag = useTagImage(connectionId);

  useEffect(() => {
    setTarget("");
    setTouched(false);
    setServerError(null);
  }, [image?.key]);

  const problem = quickRefProblem(target);
  const shown = serverError ?? (touched ? problem : null);

  const submit = async () => {
    setTouched(true);
    if (!image || problem) return;
    try {
      await tag.mutateAsync({ source: image.ref, target: target.trim() });
      toast.success(`Tagged ${image.ref} as ${target.trim()}`);
      onClose();
    } catch (e) {
      setServerError(errorMessage(e));
    }
  };

  return (
    <Dialog open={!!image} onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>Tag image</DialogTitle>
          <DialogDescription>
            Add a new tag to <span className="font-mono text-foreground">{image?.ref}</span>. The original tag is kept.
          </DialogDescription>
        </DialogHeader>
        <form
          onSubmit={(e) => {
            e.preventDefault();
            void submit();
          }}
          className="space-y-2"
        >
          <label htmlFor="tag-target" className="text-xs font-medium text-muted-foreground">
            New reference
          </label>
          <Input
            id="tag-target"
            autoFocus
            value={target}
            onChange={(e) => {
              setTarget(e.target.value);
              setServerError(null);
            }}
            onBlur={() => setTouched(true)}
            placeholder="myrepo/app:v2"
            className="font-mono"
            aria-invalid={!!shown}
            spellCheck={false}
            autoComplete="off"
          />
          {shown && <p className="text-xs text-destructive">{shown}</p>}
          <DialogFooter className="pt-2">
            <Button type="button" variant="outline" onClick={onClose}>
              Cancel
            </Button>
            <Button type="submit" disabled={tag.isPending}>
              {tag.isPending && <Loader2 className="size-4 animate-spin" />}
              Add tag
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
