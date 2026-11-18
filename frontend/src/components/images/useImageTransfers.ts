import { useCallback } from "react";
import { toast } from "sonner";
import { DialogService } from "@bindings/services";
import type { ImageRow } from "@bindings/model";
import { useLoadImage, useSaveImages } from "@/hooks/useImages";
import { useOperations } from "@/hooks/useOperations";
import { toastError } from "@/lib/errors";


/** Native-dialog driven Save / Load. Both run as background operations. */
export function useImageTransfers(connectionId: string) {
  const { setCenterOpen } = useOperations();
  const save = useSaveImages(connectionId);
  const load = useLoadImage(connectionId);

  const saveImages = useCallback(
    async (rows: ImageRow[]) => {
      if (rows.length === 0) return;
      const refs = [...new Set(rows.map((r) => r.ref))];
      const base = rows.length === 1 && !rows[0].dangling ? `${rows[0].repository.replace(/[^\w.-]+/g, "_")}-${rows[0].tag || "latest"}` : "images";
      try {
        const dest = await DialogService.PickSavePath(`Save ${refs.length === 1 ? refs[0] : `${refs.length} images`} as tar archive`, `${base}.tar`);
        if (!dest) return;
        const op = await save.mutateAsync({ refs, destPath: dest });
        toast.success(`Saving ${op.title.replace(/^Save /, "")}`, {
          description: "Progress is shown in the Activity center.",
          action: { label: "View", onClick: () => setCenterOpen(true) },
        });
      } catch (e) {
        toastError("Could not start save", e);
      }
    },
    [save, setCenterOpen],
  );

  const loadImage = useCallback(async () => {
    try {
      const src = await DialogService.PickFile("Load image archive", "Image archives (*.tar, *.tar.gz)", "*.tar;*.tar.gz;*.tgz");
      if (!src) return;
      const op = await load.mutateAsync(src);
      toast.success(`Loading ${op.resource}`, {
        description: "Progress is shown in the Activity center.",
        action: { label: "View", onClick: () => setCenterOpen(true) },
      });
    } catch (e) {
      toastError("Could not start load", e);
    }
  }, [load, setCenterOpen]);

  return { saveImages, loadImage, busy: save.isPending || load.isPending };
}
