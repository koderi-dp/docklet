import { ImageService } from "@bindings/services";
import { OperationStatus, type DebugShellResult } from "@bindings/model";
import { awaitOperation } from "@/lib/awaitOperation";

export const TOOLBOX_IMAGES = [
  { value: "busybox:latest", label: "busybox:latest (small)" },
  { value: "alpine:latest", label: "alpine:latest (apk)" },
  { value: "nicolaka/netshoot:latest", label: "nicolaka/netshoot:latest (network tools)" },
] as const;

export const CUSTOM_IMAGE = "__custom__";

export function isPresetImage(image: string): boolean {
  return TOOLBOX_IMAGES.some((i) => i.value === image);
}

/** Why a debug shell cannot be opened right now; undefined when it can. */
export function debugShellDisabledReason(state: string, engineOS?: string): string | undefined {
  if (engineOS === "windows") return "Debug shells are not supported on Windows container engines.";
  if (state !== "running") return "The container is not running. Start it to open a debug shell.";
  return undefined;
}

/** The Engine reported that the container has no usable shell (see ExecService.Start). */
export function isNoShellError(message: string): boolean {
  return /no usable shell/i.test(message);
}

/** Dim intro printed into the terminal when a debug shell starts. */
export function debugBanner(r: DebugShellResult): string {
  const lines = [
    `Debug shell: toolbox ${r.image} (${r.sidecarName}) shares this container's process and network namespaces.`,
    "Its filesystem is at /proc/1/root (not readable when the container runs as a non-root user).",
    "The toolbox is removed when this terminal closes.",
    ...(r.notices ?? []),
  ];
  return lines.map((l) => `\x1b[2m# ${l}\x1b[0m\r\n`).join("");
}

/** Pulls the toolbox image through the regular pull operation and waits for it. */
export async function pullToolboxImage(connectionId: string, image: string): Promise<void> {
  const started = await ImageService.Pull(connectionId, image);
  const op = await awaitOperation(started.id);
  if (op.status !== OperationStatus.OpSucceeded) {
    throw new Error(op.error || `Pulling ${image} did not complete (${op.status}).`);
  }
}
