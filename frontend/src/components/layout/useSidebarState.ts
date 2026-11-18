import { useEffect, useState } from "react";
import { useUI } from "@/stores/uiStore";

/** Below this window width the sidebar collapses automatically unless the user pinned a state. */
export const NARROW_BREAKPOINT = 1200;

export function useViewportWidth(): number {
  const [w, setW] = useState(() => window.innerWidth);
  useEffect(() => {
    const on = () => setW(window.innerWidth);
    window.addEventListener("resize", on);
    return () => window.removeEventListener("resize", on);
  }, []);
  return w;
}

/** Effective collapsed state: the user's explicit choice when pinned, otherwise width-driven. */
export function useSidebarCollapsed(): { collapsed: boolean; setCollapsed: (v: boolean) => void } {
  const stored = useUI((s) => s.sidebarCollapsed);
  const pinned = useUI((s) => s.sidebarPinned);
  const set = useUI((s) => s.setSidebarCollapsed);
  const narrow = useViewportWidth() < NARROW_BREAKPOINT;
  return { collapsed: pinned ? stored : stored || narrow, setCollapsed: set };
}
