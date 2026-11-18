import { useQuery } from "@tanstack/react-query";
import { PortService } from "@bindings/services";

export const portListingKey = (connectionId: string) => ["port-listing", connectionId] as const;

/** Published ports of the Engine's containers plus (for a local Engine) the listeners of this computer. Refreshed on demand. */
export function usePortListing(connectionId: string | null) {
  return useQuery({
    queryKey: portListingKey(connectionId ?? "none"),
    queryFn: () => PortService.Listeners(connectionId as string),
    enabled: !!connectionId,
    staleTime: 5_000,
    retry: false,
  });
}
