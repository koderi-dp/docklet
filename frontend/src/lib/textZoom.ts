import { Window } from "@wailsio/runtime";
import { useUI } from "@/stores/uiStore";
import { normalizeTextSize } from "./textSize";

/** Sets the native webview zoom (WebView2 zoom factor, WKWebView pageZoom, WebKitGTK zoom level). */
function applyTextSize(percent: number) {
  try {
    void Window.SetZoom(normalizeTextSize(percent) / 100).catch(() => undefined);
  } catch {
    /* not running inside Wails (plain browser) */
  }
}

/** Applies the stored size before the first render and keeps the window in step with the setting. */
export function startTextZoom() {
  applyTextSize(useUI.getState().textSize);
  useUI.subscribe((s, prev) => {
    if (s.textSize !== prev.textSize) applyTextSize(s.textSize);
  });
}
