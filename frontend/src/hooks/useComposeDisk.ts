import { useQuery } from "@tanstack/react-query";
import { ComposeService } from "@bindings/services";

/** Not in the live-invalidated segments of useCompose: the Engine's disk accounting is slow and only refreshed on demand. */
export const composeDiskKey = (c: string) => ["compose", c, "disk"] as const;

/** Per-project disk footprint of a connection (one Engine call for all projects), cached briefly per connection. */
export function useComposeDisk(connectionId: string | null, enabled: boolean) {
  return useQuery({
    queryKey: composeDiskKey(connectionId ?? "none"),
    queryFn: () => ComposeService.ProjectDiskUsage(connectionId as string),
    enabled: !!connectionId && enabled,
    staleTime: 60_000,
    gcTime: 5 * 60_000,
    refetchOnWindowFocus: false,
    retry: false,
  });
}
