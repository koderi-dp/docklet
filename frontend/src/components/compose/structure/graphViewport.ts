// Pure pan/zoom maths for the service graph canvas.

export interface View { scale: number; x: number; y: number }
interface Size { width: number; height: number }

export const MIN_SCALE = 0.3;
export const MAX_SCALE = 2;

const clamp = (n: number, lo: number, hi: number) => Math.min(hi, Math.max(lo, n));

/** Scale that fits the content into the box (never above 1), content centred. */
export function fitView(content: Size, box: Size, padding = 32): View {
  const w = Math.max(content.width, 1);
  const h = Math.max(content.height, 1);
  const fit = Math.min((box.width - 2 * padding) / w, (box.height - 2 * padding) / h);
  const scale = clamp(Number.isFinite(fit) ? fit : 1, MIN_SCALE, 1);
  return { scale, x: (box.width - content.width * scale) / 2, y: (box.height - content.height * scale) / 2 };
}

/** Zoom by a factor around a point given in box coordinates; the content under it stays put. */
export function zoomAt(view: View, factor: number, point: { x: number; y: number }): View {
  const scale = clamp(view.scale * factor, MIN_SCALE, MAX_SCALE);
  if (scale === view.scale) return view;
  const k = scale / view.scale;
  return { scale, x: point.x - (point.x - view.x) * k, y: point.y - (point.y - view.y) * k };
}

interface PanEvent {
  button: number;
  clientX: number;
  clientY: number;
  pointerId: number;
  target: unknown;
  currentTarget: { setPointerCapture(id: number): void; hasPointerCapture(id: number): boolean; releasePointerCapture(id: number): void };
}
type DragRef = { current: { x: number; y: number } | null };

/** Pointer handlers that pan the view; a cancelled or lost pointer ends the drag like a release. */
export function createPan(drag: DragRef, setView: (update: (v: View) => View) => void) {
  const end = (e: Pick<PanEvent, "pointerId" | "currentTarget">) => {
    drag.current = null;
    if (e.currentTarget.hasPointerCapture(e.pointerId)) e.currentTarget.releasePointerCapture(e.pointerId);
  };
  return {
    onPointerDown: (e: PanEvent) => {
      if (e.button !== 0 || (e.target as HTMLElement).closest("[data-graph-card]")) return;
      e.currentTarget.setPointerCapture(e.pointerId);
      drag.current = { x: e.clientX, y: e.clientY };
    },
    onPointerMove: (e: Pick<PanEvent, "clientX" | "clientY">) => {
      const d = drag.current;
      if (!d) return;
      const dx = e.clientX - d.x;
      const dy = e.clientY - d.y;
      drag.current = { x: e.clientX, y: e.clientY };
      setView((v) => ({ ...v, x: v.x + dx, y: v.y + dy }));
    },
    onPointerUp: end,
    onPointerCancel: end,
    onLostPointerCapture: () => {
      drag.current = null;
    },
  };
}
