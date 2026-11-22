import { createElement, useEffect, type ReactNode } from "react";
import { useBlocker } from "react-router-dom";
import { DiscardChangesDialog } from "./DiscardChangesDialog";

/**
 * Holds back route changes and window closing while there are unsaved edits.
 * Returns the dialog to render for a held navigation.
 */
export function useUnsavedGuard(dirty: boolean): ReactNode {
  const blocker = useBlocker(dirty);
  useEffect(() => {
    if (!dirty) return;
    const warn = (e: BeforeUnloadEvent) => {
      e.preventDefault();
      e.returnValue = "";
    };
    window.addEventListener("beforeunload", warn);
    return () => window.removeEventListener("beforeunload", warn);
  }, [dirty]);
  return createElement(DiscardChangesDialog, {
    open: blocker.state === "blocked",
    onDiscard: () => blocker.proceed?.(),
    onKeep: () => blocker.reset?.(),
  });
}
