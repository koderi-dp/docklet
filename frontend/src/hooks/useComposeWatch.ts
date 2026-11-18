import { useCallback, useEffect, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { ComposeService } from "@bindings/services";
import type { ComposeProject, ComposeWatchOptions } from "@bindings/model";
import { confirmComposeUp } from "@/components/ports/portGuard";
import { useInvalidateOnEvents } from "@/hooks/queryUtils";
import { toastError } from "@/lib/errors";

export const watchInfoKey = (c: string, n: string) => ["compose", c, "watch-info", n] as const;
export const watchStateKey = (c: string, n: string) => ["compose", c, "watch-state", n] as const;

/** The develop.watch rules of the project and whether watch can start. Read-only, reads the Compose config. */
export function useWatchInfo(connectionId: string | null, name: string, enabled: boolean) {
  return useQuery({
    queryKey: watchInfoKey(connectionId ?? "none", name),
    queryFn: () => ComposeService.WatchInfo(connectionId as string, name),
    enabled: !!connectionId && !!name && enabled,
    staleTime: 15_000,
    retry: false,
  });
}

/** Live watch of the project (refetched whenever the backend reports a change). */
export function useWatchState(connectionId: string | null, name: string) {
  useInvalidateOnEvents(["compose:watch"], connectionId, true, [
    { id: `compose-watch:${connectionId}:${name}`, filters: { queryKey: watchStateKey(connectionId ?? "none", name) } },
  ]);
  return useQuery({
    queryKey: watchStateKey(connectionId ?? "none", name),
    queryFn: () => ComposeService.WatchState(connectionId as string, name),
    enabled: !!connectionId && !!name,
    retry: false,
  });
}

/** Re-renders every `ms` while `enabled`; returns the current time (for elapsed counters). */
export function useNow(enabled: boolean, ms = 1000): number {
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    if (!enabled) return;
    setNow(Date.now());
    const t = setInterval(() => setNow(Date.now()), ms);
    return () => clearInterval(t);
  }, [enabled, ms]);
  return now;
}

export function useWatchActions(connectionId: string, name: string) {
  const qc = useQueryClient();
  const [busy, setBusy] = useState(false);
  const refresh = useCallback(() => qc.invalidateQueries({ queryKey: watchStateKey(connectionId, name) }), [qc, connectionId, name]);
  const start = useMutation({ mutationFn: (o: ComposeWatchOptions) => ComposeService.StartWatch(connectionId, name, o) });
  const stop = useMutation({ mutationFn: () => ComposeService.StopWatch(connectionId, name) });

  const startWatch = useCallback(
    async (opts: ComposeWatchOptions) => {
      // Without --no-up the CLI first builds and starts the services: say what already holds their ports.
      if (!opts.noUp && !(await confirmComposeUp(connectionId, name, "", []))) return;
      setBusy(true);
      try {
        await start.mutateAsync(opts);
        toast.success(`Watch started for ${name}`, { description: "It keeps running until you stop it or close Docklet." });
      } catch (e) {
        toastError(`Watch could not start for ${name}`, e);
      } finally {
        setBusy(false);
        void refresh();
      }
    },
    [connectionId, name, start, refresh],
  );

  const stopWatch = useCallback(async () => {
    setBusy(true);
    try {
      await stop.mutateAsync();
      toast.success(`Stopped watching ${name}`, { description: "Containers keep running." });
    } catch (e) {
      toastError(`Watch could not be stopped for ${name}`, e);
    } finally {
      setBusy(false);
      void refresh();
    }
  }, [name, stop, refresh]);

  const dismiss = useCallback(async () => {
    try {
      await ComposeService.DismissWatch(connectionId, name);
    } finally {
      void refresh();
    }
  }, [connectionId, name, refresh]);

  return { startWatch, stopWatch, dismiss, busy };
}

/** Why watch cannot start for a project (undefined = it can), and whether it is running. Shared by the header and the menu. */
export function useWatchAvailability(connectionId: string, project: ComposeProject) {
  const canRun = project.capability.canRun;
  const info = useWatchInfo(connectionId, project.name, canRun);
  const state = useWatchState(connectionId, project.name);
  const reason = !canRun
    ? project.capability.reason || "Compose CLI is not available for this project"
    : info.isLoading
      ? "Reading the watch rules of the Compose file…"
      : info.error
        ? (info.error as Error).message
        : info.data && !info.data.available
          ? info.data.reason
          : undefined;
  const noRules = canRun && !!info.data && !info.data.available && (info.data.ruleCount ?? 0) === 0 && !info.error;
  return { info, state, reason, noRules, running: state.data?.running === true };
}
