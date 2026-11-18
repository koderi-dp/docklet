import { useEffect } from "react";
import { useUI } from "@/stores/uiStore";

/** Resolves themePreference "system" into the dark/light `theme` that App.tsx applies. */
export function useSystemTheme() {
  const pref = useUI((s) => s.themePreference);
  useEffect(() => {
    if (pref !== "system") {
      if (useUI.getState().theme !== pref) useUI.setState({ theme: pref });
      return;
    }
    const mq = window.matchMedia("(prefers-color-scheme: dark)");
    const apply = () => useUI.setState({ theme: mq.matches ? "dark" : "light" });
    apply();
    mq.addEventListener("change", apply);
    return () => mq.removeEventListener("change", apply);
  }, [pref]);
  const palette = useUI((s) => s.themePalette);
  useEffect(() => {
    if (palette === "shadcn") document.documentElement.dataset.palette = "shadcn";
    else delete document.documentElement.dataset.palette;
  }, [palette]);
}
