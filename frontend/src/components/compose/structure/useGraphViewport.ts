import { useCallback, useEffect, useLayoutEffect, useRef, useState } from "react";
import { createPan, fitView, zoomAt, type View } from "./graphViewport";

const STEP = 1.2;
const EMPTY: View = { scale: 1, x: 0, y: 0 };

/**
 * Pan and zoom state of the graph canvas. Fits once when the layout first arrives and again only when
 * Fit is pressed; resizing the window after that leaves the user's view alone.
 */
export function useGraphViewport(content: { width: number; height: number } | null) {
  const [el, boxRef] = useState<HTMLDivElement | null>(null);
  const [view, setView] = useState<View>(EMPTY);
  const [ready, setReady] = useState(false);
  const fitted = useRef(false);
  const contentRef = useRef(content);
  contentRef.current = content;

  const box = useCallback(() => {
    const r = el?.getBoundingClientRect();
    return r ? { width: r.width, height: r.height } : null;
  }, [el]);

  const fit = useCallback(() => {
    const b = box();
    const c = contentRef.current;
    if (!b || !c || b.width === 0 || b.height === 0) return false;
    setView(fitView(c, b));
    return true;
  }, [box]);

  const hasContent = content !== null && content.width > 0;
  useLayoutEffect(() => {
    if (fitted.current || !hasContent) return;
    if (fit()) {
      fitted.current = true;
      setReady(true);
    }
  }, [hasContent, fit, el]);

  // The box can have no size on the first frame; fit as soon as it gets one.
  useEffect(() => {
    if (!el || typeof ResizeObserver === "undefined") return;
    const ro = new ResizeObserver(() => {
      if (fitted.current || !contentRef.current || contentRef.current.width === 0) return;
      if (fit()) {
        fitted.current = true;
        setReady(true);
      }
    });
    ro.observe(el);
    return () => ro.disconnect();
  }, [fit, el]);

  const zoomCentre = useCallback(
    (factor: number) => {
      const b = box();
      if (b) setView((v) => zoomAt(v, factor, { x: b.width / 2, y: b.height / 2 }));
    },
    [box],
  );

  // Wheel needs a non-passive listener to keep the page from scrolling.
  useEffect(() => {
    if (!el) return;
    const onWheel = (e: WheelEvent) => {
      e.preventDefault();
      const r = el.getBoundingClientRect();
      const factor = Math.exp(-e.deltaY * 0.0015);
      setView((v) => zoomAt(v, factor, { x: e.clientX - r.left, y: e.clientY - r.top }));
    };
    el.addEventListener("wheel", onWheel, { passive: false });
    return () => el.removeEventListener("wheel", onWheel);
  }, [el]);

  const drag = useRef<{ x: number; y: number } | null>(null);
  const pan = createPan(drag, setView);

  return {
    boxRef,
    view,
    ready,
    pan,
    fit,
    zoomIn: () => zoomCentre(STEP),
    zoomOut: () => zoomCentre(1 / STEP),
  };
}
