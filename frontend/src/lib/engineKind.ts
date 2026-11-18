import { EngineKind } from "@bindings/model";
import { useConnectionStatus } from "@/hooks/useActiveConnection";

/** Podman answers the Docker API through its compatibility layer; everything unrecognised is Docker. */
export function isPodmanKind(kind: EngineKind | string | undefined | null): boolean {
  return kind === EngineKind.EnginePodman;
}

/** "Podman" or "Docker": the engine's product name. */
export function engineName(kind: EngineKind | string | undefined | null): string {
  return isPodmanKind(kind) ? "Podman" : "Docker";
}

/** "Podman 6.1.4" / "Docker 29.1.0"; just the name when the version is unknown. */
export function engineVersionLabel(kind: EngineKind | string | undefined | null, version?: string): string {
  return [engineName(kind), version].filter(Boolean).join(" ");
}

/** Why registry update checks are off on Podman: its Docker API has no digest lookup. */
export const PODMAN_NO_UPDATE_CHECK = "Podman's Docker API cannot look up registry digests. Pull the image again (Update…) to get the latest.";

/** "Podman" or "Docker" for a connection (Docker until its status is known). */
export function useEngineName(connectionId: string | null | undefined): string {
  const { data } = useConnectionStatus(connectionId ?? null);
  return engineName(data?.kind);
}

/** Whether the connection is a Podman engine (false while its status is not known yet). */
export function useIsPodman(connectionId: string | null | undefined): boolean {
  const { data } = useConnectionStatus(connectionId ?? null);
  return isPodmanKind(data?.kind);
}
