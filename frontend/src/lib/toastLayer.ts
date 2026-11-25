/** The element sonner renders every toast into. */
const TOASTER = "[data-sonner-toaster]";

type MaybeElement = { closest?: (selector: string) => unknown } | null | undefined;

interface OutsideEvent {
  target: unknown;
  detail?: { originalEvent?: { target: unknown } };
  preventDefault: () => void;
}

const inToaster = (t: unknown) => !!(t as MaybeElement)?.closest?.(TOASTER);

/**
 * For a dialog's or panel's "pressed outside" handlers: a press on a toast is not a press
 * outside. Without this, closing a toast would close the dialog that is open under it.
 */
export function keepOpenForToasts(e: OutsideEvent): void {
  if (inToaster(e.detail?.originalEvent?.target) || inToaster(e.target)) e.preventDefault();
}

/** The toast of one operation, so it can be replaced or dismissed by id. */
export const opToastId = (opId: string) => `operation-${opId}`;

/** A failure needs no toast when the Operations panel is open on that very operation. */
export function failureToastWanted(panel: { panelOpen: boolean; selectedId: string | null }, opId: string): boolean {
  return !(panel.panelOpen && panel.selectedId === opId);
}
