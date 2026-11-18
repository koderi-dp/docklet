import { useQuery } from "@tanstack/react-query";
import { ConnectionService } from "@bindings/services";

export const environmentKey = ["environment"] as const;

/**
 * What this machine already has (WSL distributions, local engines, Docker contexts, saved connections).
 * Read-only; refetching is the "Check again" action and never reuses an older answer.
 */
export function useEnvironment(enabled = true) {
  return useQuery({
    queryKey: environmentKey,
    queryFn: () => ConnectionService.DetectEnvironment(),
    enabled,
    retry: false,
    staleTime: 0,
    gcTime: 0,
    refetchOnWindowFocus: false,
  });
}
