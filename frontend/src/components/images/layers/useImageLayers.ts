import { useQuery } from "@tanstack/react-query";
import { ImageService } from "@bindings/services";

export const imageLayersKey = (connectionId: string, ref: string) => ["image-layers", connectionId, ref] as const;

/** Layer breakdown for one image; only fetched while `enabled` (dialog open). */
export function useImageLayers(connectionId: string, ref: string, enabled: boolean) {
  return useQuery({
    queryKey: imageLayersKey(connectionId, ref),
    queryFn: () => ImageService.Layers(connectionId, ref),
    enabled: enabled && !!ref,
    retry: false,
    staleTime: 0,
  });
}
