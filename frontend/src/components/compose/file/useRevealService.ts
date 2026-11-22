import { useEffect, useRef } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { ComposeService } from "@bindings/services";
import type { ComposeFileRef, ComposeOutlineItem } from "@bindings/model";
import { composeFileKey } from "@/hooks/useComposeFiles";

export interface RevealRequest {
  service: string;
  nonce: number;
}

interface Options {
  reveal: RevealRequest | undefined;
  /** The services of the open file, only while that outline belongs to the text in the editor. */
  outlineServices: readonly ComposeOutlineItem[] | null;
  path: string;
  files: readonly ComposeFileRef[];
  connectionId: string;
  name: string;
  /** Opens another file without asking; false when it did not (unsaved changes). */
  switchFile: (path: string) => boolean;
  jumpTo: (line: number) => void;
}

/**
 * Brings the line of a service into view once per request: in the open file when it is there,
 * else in the first other file that defines it.
 */
export function useRevealService({ reveal, outlineServices, path, files, connectionId, name, switchFile, jumpTo }: Options) {
  const qc = useQueryClient();
  const handled = useRef(0);
  const pending = useRef<{ service: string; path: string } | null>(null);
  const search = useRef(0);
  const latest = useRef({ switchFile, jumpTo, files });
  latest.current = { switchFile, jumpTo, files };

  // A new request or a new outline: take the request when it is not handled yet, or finish a switch.
  useEffect(() => {
    if (!outlineServices) return;
    const wait = pending.current;
    if (wait && wait.path === path) {
      pending.current = null;
      const hit = outlineServices.find((s) => s.name === wait.service);
      if (hit) latest.current.jumpTo(hit.line);
      return;
    }
    if (!reveal || reveal.nonce === handled.current) return;
    handled.current = reveal.nonce;
    const hit = outlineServices.find((s) => s.name === reveal.service);
    if (hit) {
      latest.current.jumpTo(hit.line);
      return;
    }
    const token = ++search.current;
    const others = latest.current.files.map((f) => f.path).filter((p) => p !== path);
    void (async () => {
      for (const other of others) {
        try {
          const file = await qc.fetchQuery({
            queryKey: composeFileKey(connectionId, name, other),
            queryFn: () => ComposeService.ReadProjectFile(connectionId, name, other),
            staleTime: 0,
          });
          if (token !== search.current) return;
          const outline = await qc.fetchQuery({
            queryKey: ["compose", "outline", file.text],
            queryFn: () => ComposeService.FileOutline(file.text),
            staleTime: 60_000,
          });
          if (token !== search.current) return;
          if ((outline.services ?? []).some((s) => s.name === reveal.service)) {
            if (latest.current.switchFile(other)) pending.current = { service: reveal.service, path: other };
            return;
          }
        } catch {
          // A file that cannot be read or parsed has no line to show; try the next.
        }
      }
    })();
  }, [reveal, outlineServices, path, connectionId, name, qc]);

  // A newer request cancels a search still running.
  useEffect(() => () => void search.current++, [reveal]);
}
