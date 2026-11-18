import { useEffect } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { useUI } from "@/stores/uiStore";

/** True when the keyboard event belongs to an xterm terminal, which must keep Ctrl+K/Ctrl+R. */
export function isTerminalEvent(e: KeyboardEvent): boolean {
  const t = e.target;
  return t instanceof Element && !!t.closest(".xterm");
}

/**
 * Ctrl/Cmd+K toggles the command palette; Ctrl/Cmd+R refreshes the current page's
 * queries without reloading the window. Both are ignored inside terminals and when
 * another handler already consumed the event.
 */
export function useGlobalShortcuts() {
  const qc = useQueryClient();
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.defaultPrevented || e.altKey || e.shiftKey || !(e.ctrlKey || e.metaKey)) return;
      if (isTerminalEvent(e)) return;
      const k = e.key.toLowerCase();
      if (k === "k") {
        e.preventDefault();
        const { paletteOpen, setPaletteOpen } = useUI.getState();
        setPaletteOpen(!paletteOpen);
      } else if (k === "r") {
        e.preventDefault();
        void qc.invalidateQueries();
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [qc]);
}
