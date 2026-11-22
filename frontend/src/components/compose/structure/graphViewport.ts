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
