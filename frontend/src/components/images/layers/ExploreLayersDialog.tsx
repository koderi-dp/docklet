import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Skeleton } from "@/components/ui/skeleton";
import { useImageLayers } from "./useImageLayers";
import { LayersSummary } from "./LayersSummary";
import { LayersTable } from "./LayersTable";

interface Props {
  connectionId: string;
  imageRef: string;
  title: string;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

/** Wide dialog showing where an image's size comes from. */
export function ExploreLayersDialog({ connectionId, imageRef, title, open, onOpenChange }: Props) {
  const q = useImageLayers(connectionId, imageRef, open);
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="overflow-x-hidden sm:max-w-[920px] [&>*]:min-w-0">
        <DialogHeader>
          <DialogTitle className="truncate pr-6">Layers of {title}</DialogTitle>
          <DialogDescription>Where the size of this image comes from, as reported by the Docker Engine.</DialogDescription>
        </DialogHeader>
        {q.isLoading && <Skeleton className="h-64 w-full" />}
        {q.error && <p className="text-[13px] text-destructive">Could not load layers: {String(q.error)}</p>}
        {q.data && (
          <div className="space-y-4">
            <LayersSummary report={q.data} />
            <LayersTable report={q.data} />
          </div>
        )}
      </DialogContent>
    </Dialog>
  );
}
