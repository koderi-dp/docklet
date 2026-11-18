import { useMemo } from "react";
import { useQuery } from "@tanstack/react-query";
import { FileService } from "@bindings/services";
import type { FileCapabilities, FileListing, FileText, Operation } from "@bindings/model";

/**
 * A file source for the FileBrowser: a container (archive API) or a volume (helper container).
 * Optional members are omitted when the source does not support the action.
 */
export interface FileBackend {
  /** Cache key part; together with connectionId it identifies the source. */
  id: string;
  connectionId: string;
  list(path: string): Promise<FileListing>;
  readText(path: string): Promise<FileText>;
  download(path: string, dest: string): Promise<Operation>;
  upload?: (destDir: string, src: string) => Promise<Operation>;
  mkdir?: (parent: string, name: string) => Promise<void>;
  remove?: (path: string) => Promise<void>;
  /** Rebuilds the cached index that serves stopped containers. */
  refreshIndex?: () => Promise<unknown>;
}

const PREVIEW_BYTES = 256 * 1024;

export function useContainerFileBackend(connectionId: string, containerId: string): FileBackend {
  return useMemo(
    () => ({
      id: `container:${containerId}`,
      connectionId,
      list: (p) => FileService.List(connectionId, containerId, p),
      readText: (p) => FileService.ReadText(connectionId, containerId, p, PREVIEW_BYTES),
      download: (p, dest) => FileService.Download(connectionId, containerId, p, dest),
      upload: (dir, src) => FileService.Upload(connectionId, containerId, dir, src),
      mkdir: (parent, name) => FileService.Mkdir(connectionId, containerId, parent, name),
      remove: (p) => FileService.Delete(connectionId, containerId, p),
      refreshIndex: () => FileService.RefreshIndex(connectionId, containerId),
    }),
    [connectionId, containerId],
  );
}

/** Read-only volume source (browse + download only). */
export function useVolumeFileBackend(connectionId: string, volume: string): FileBackend {
  return useMemo(
    () => ({
      id: `volume:${volume}`,
      connectionId,
      list: (p) => FileService.VolumeBrowse(connectionId, volume, p),
      readText: (p) => FileService.VolumeReadText(connectionId, volume, p, PREVIEW_BYTES),
      download: (p, dest) => FileService.VolumeDownload(connectionId, volume, p, dest),
    }),
    [connectionId, volume],
  );
}

export const filesKey = (b: Pick<FileBackend, "connectionId" | "id">) => ["files", b.connectionId, b.id] as const;

export function useFileListing(backend: FileBackend, path: string) {
  return useQuery({
    queryKey: [...filesKey(backend), "list", path],
    queryFn: () => backend.list(path),
    retry: false,
    gcTime: 30_000,
  });
}

export function useFilePreview(backend: FileBackend, path: string | null) {
  return useQuery({
    queryKey: [...filesKey(backend), "text", path],
    queryFn: () => backend.readText(path as string),
    enabled: !!path,
    retry: false,
    staleTime: 10_000,
  });
}

/** Capabilities depend on the container state, so it is part of the key. */
export function useFileCaps(connectionId: string, containerId: string, state: string) {
  return useQuery<FileCapabilities>({
    queryKey: ["files", connectionId, `container:${containerId}`, "caps", state],
    queryFn: () => FileService.Capabilities(connectionId, containerId),
    retry: false,
  });
}
