import { useEffect } from "react";
import { toast } from "sonner";
import { useUI } from "@/stores/uiStore";
import { DEFAULT_TEXT_SIZE, stepTextSize, textSizeShortcut } from "@/lib/textSize";

/**
 * Ctrl/Cmd with "+", "-" and "0" change the text size. Keys typed in the terminal never get here
 * (it stops them), which is right: Ctrl+- is a real terminal key (^_).
 */
export function useTextSizeShortcuts() {
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      const action = textSizeShortcut(e);
      if (!action || e.defaultPrevented) return;
      e.preventDefault();
      const { textSize, setTextSize } = useUI.getState();
      const next = action === "reset" ? DEFAULT_TEXT_SIZE : stepTextSize(textSize, action === "larger" ? 1 : -1);
      setTextSize(next);
      toast(`Text size ${next}%`, { id: "text-size", duration: 1200 });
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, []);
}
