import { useCallback, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { ComposeService } from "@bindings/services";
import type { ComposeProject, Container } from "@bindings/model";
import { confirmComposeUp } from "@/components/ports/portGuard";
import { useContainerAction } from "@/hooks/useContainers";
import { useInvalidateOnEvents } from "@/hooks/queryUtils";
import { ACTION_LABEL, partitionByEligibility, type ContainerAction } from "@/lib/actionEligibility";
import { toastError } from "@/lib/errors";

/** Every key starts with ["compose", connectionId] so engines never share cache entries. */
export const composeProjectsKey = (c: string) => ["compose", c, "projects"] as const;
export const composeProjectKey = (c: string, n: string) => ["compose", c, "project", n] as const;
export const composeResourcesKey = (c: string, n: string) => ["compose", c, "resources", n] as const;
export const composeConfigKey = (c: string, n: string) => ["compose", c, "config", n] as const;
export const composeLogsKey = (c: string, n: string, tail: number) => ["compose", c, "logs", n, tail] as const;

const LIVE_SEGMENTS = new Set(["projects", "project", "resources"]);

/** Refetch project data when the Engine reports container changes (this connection only). */
function useComposeEvents(connectionId: string | null) {
  useInvalidateOnEvents(["containers:changed"], connectionId, true, [
    {
      id: `compose-live:${connectionId}`,
      filters: {
        predicate: (q) => q.queryKey[0] === "compose" && q.queryKey[1] === connectionId && LIVE_SEGMENTS.has(q.queryKey[2] as string),
      },
    },
  ]);
}

export function useComposeProjects(connectionId: string | null) {
  useComposeEvents(connectionId);
  return useQuery({
    queryKey: composeProjectsKey(connectionId ?? "none"),
    queryFn: () => ComposeService.ListProjects(connectionId as string),
    enabled: !!connectionId,
    refetchInterval: 30_000,
    retry: false,
  });
}

export function useComposeProject(connectionId: string | null, name: string) {
  useComposeEvents(connectionId);
  return useQuery({
    queryKey: composeProjectKey(connectionId ?? "none", name),
    queryFn: () => ComposeService.GetProject(connectionId as string, name),
    enabled: !!connectionId && !!name,
    refetchInterval: 30_000,
    retry: false,
  });
}

export function useComposeResources(connectionId: string | null, name: string, enabled: boolean) {
  return useQuery({
    queryKey: composeResourcesKey(connectionId ?? "none", name),
    queryFn: () => ComposeService.Resources(connectionId as string, name),
    enabled: !!connectionId && !!name && enabled,
    retry: false,
  });
}

export function useComposeConfig(connectionId: string | null, name: string, enabled: boolean) {
  return useQuery({
    queryKey: composeConfigKey(connectionId ?? "none", name),
    queryFn: () => ComposeService.Config(connectionId as string, name),
    enabled: !!connectionId && !!name && enabled,
    staleTime: 60_000,
    retry: false,
  });
}

/** Snapshot only: fetched when enabled and on explicit refresh, never polled. */
export function useComposeLogs(connectionId: string | null, name: string, tail: number, enabled: boolean) {
  return useQuery({
    queryKey: composeLogsKey(connectionId ?? "none", name, tail),
    queryFn: () => ComposeService.ProjectLogs(connectionId as string, name, tail),
    enabled: !!connectionId && !!name && enabled,
    staleTime: Infinity,
    retry: false,
  });
}

export type ComposeVerb = "up" | "down" | "start" | "stop" | "restart" | "pull" | "build";

export interface UpOptions {
  build: boolean;
  removeOrphans: boolean;
  profiles: string[];
}
export const DEFAULT_UP: UpOptions = { build: false, removeOrphans: false, profiles: [] };


/**
 * Starts a Compose CLI operation in the background. Resolves once the backend accepted it;
 * progress and the final result live in Operations, data refetches on containers:changed.
 */
export function useComposeVerb(connectionId: string) {
  const [busy, setBusy] = useState<Set<string>>(new Set());
  const mutation = useMutation({
    mutationFn: async (v: { verb: ComposeVerb; name: string; volumes?: boolean; up?: UpOptions; service?: string }) => {
      const up = v.up ?? DEFAULT_UP;
      switch (v.verb) {
        case "up":
          return v.service ? ComposeService.UpService(connectionId, v.name, v.service, up) : ComposeService.Up(connectionId, v.name, up);
        case "down":
          return ComposeService.Down(connectionId, v.name, { volumes: !!v.volumes, removeOrphans: false, profiles: [] });
        case "start":
          return ComposeService.Start(connectionId, v.name);
        case "stop":
          return ComposeService.Stop(connectionId, v.name);
        case "restart":
          return v.service ? ComposeService.RestartService(connectionId, v.name, v.service) : ComposeService.Restart(connectionId, v.name);
        case "pull":
          return ComposeService.Pull(connectionId, v.name);
        case "build":
          return ComposeService.Build(connectionId, v.name);
      }
    },
  });
  const run = useCallback(
    async (verb: ComposeVerb, name: string, opts?: { volumes?: boolean; up?: UpOptions; service?: string }) => {
      // Starting a project: say first what already holds its host ports (no dialog when nothing does).
      if (verb === "up" && !(await confirmComposeUp(connectionId, name, opts?.service ?? "", opts?.up?.profiles ?? []))) return;
      setBusy((s) => new Set(s).add(name));
      const target = opts?.service ? `${name} / ${opts.service}` : name;
      try {
        await mutation.mutateAsync({ verb, name, volumes: opts?.volumes, up: opts?.up, service: opts?.service });
        toast.success(`Compose ${verb} started for ${target}`, { description: "Follow progress in Operations." });
      } catch (e) {
        toastError(`Compose ${verb} could not start for ${target}`, e);
      } finally {
        // The operation runs in the background; this only guards against double submits.
        setBusy((s) => {
          const n = new Set(s);
          n.delete(name);
          return n;
        });
      }
    },
    [mutation, connectionId],
  );
  return { run, isBusy: (name: string) => busy.has(name) };
}

export const composeLocationKey = (c: string) => ["compose", c, "location"] as const;

/** Where the Compose CLI runs for this connection and how file paths must be written. */
export function useComposeLocation(connectionId: string | null, enabled = true) {
  return useQuery({
    queryKey: composeLocationKey(connectionId ?? "none"),
    queryFn: () => ComposeService.Location(connectionId as string),
    enabled: !!connectionId && enabled,
    staleTime: 30_000,
    retry: false,
  });
}

/** Validates and saves a managed project. Nothing is started. */
export function useAddComposeProject(connectionId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (v: { configFiles: string[]; envFiles: string[]; name: string }) =>
      ComposeService.AddProject(connectionId, v.configFiles, v.envFiles, v.name),
    onSuccess: () => void qc.invalidateQueries({ queryKey: ["compose", connectionId] }),
  });
}

/** Forgets the saved record only; the Engine and the files are untouched. */
export function useForgetComposeProject(connectionId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (name: string) => ComposeService.RemoveProject(connectionId, name),
    onSuccess: () => void qc.invalidateQueries({ queryKey: ["compose", connectionId] }),
  });
}

/** Engine-level start/stop/restart of a project's containers (works without the Compose CLI). */
export function useProjectEngineBatch(connectionId: string) {
  const mutation = useContainerAction(connectionId);
  const [pending, setPending] = useState(false);
  const run = useCallback(
    async (action: Exclude<ContainerAction, "remove">, targets: Container[]) => {
      const part = partitionByEligibility(action, targets);
      if (part.eligibleIds.length === 0) {
        toast.info(`Nothing to ${ACTION_LABEL[action].toLowerCase()}`, {
          description: `${part.skippedCount} container(s) are not eligible for this action.`,
        });
        return;
      }
      setPending(true);
      try {
        const out = await mutation.mutateAsync({ action, ids: part.eligibleIds, skippedCount: part.skippedCount });
        const ok = out.results.filter((r) => r.ok).length;
        const failed = out.results.filter((r) => !r.ok && !r.skipped);
        const verb = ACTION_LABEL[action];
        if (failed.length) {
          toast.error(`${verb}: ${failed.length} failed${ok ? `, ${ok} succeeded` : ""}`, {
            description: failed.map((f) => `${f.name}: ${f.error}`).join("\n"),
            duration: Infinity,
            closeButton: true,
          });
        } else if (ok) {
          toast.success(`${verb}: ${ok} container${ok === 1 ? "" : "s"}${out.skippedCount ? ` (${out.skippedCount} skipped)` : ""}`);
        } else {
          toast.info(`${verb}: nothing changed`);
        }
      } catch (e) {
        toastError(`${ACTION_LABEL[action]} failed`, e);
      } finally {
        setPending(false);
      }
    },
    [mutation],
  );
  return { run, pending };
}

export function projectContainers(all: readonly Container[] | null | undefined, p: Pick<ComposeProject, "name">): Container[] {
  return (all ?? []).filter((c) => c.composeProject === p.name);
}
