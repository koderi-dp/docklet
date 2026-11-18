import { toast } from "sonner";

/** The human-readable text of anything thrown (Wails rejects with plain strings or Errors). */
export function errorMessage(e: unknown): string {
  return e instanceof Error ? e.message : String(e);
}

/** A failure toast that stays until dismissed, with the Engine/backend message as detail. */
export function toastError(title: string, e: unknown) {
  toast.error(title, { description: errorMessage(e), duration: Infinity, closeButton: true });
}
