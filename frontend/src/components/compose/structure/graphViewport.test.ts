import { describe, expect, it } from "vitest";
import { createPan, fitView, zoomAt } from "./graphViewport";

describe("createPan", () => {
  const target = { closest: () => null };
  const node = { setPointerCapture: () => {}, hasPointerCapture: () => false, releasePointerCapture: () => {} };
  const down = { button: 0, clientX: 0, clientY: 0, pointerId: 1, target, currentTarget: node };

  function setup() {
    const drag = { current: null as { x: number; y: number } | null };
    const moves: number[] = [];
    const pan = createPan(drag, () => moves.push(1));
    return { drag, moves, pan };
  }

  it("pans while dragging", () => {
    const { drag, moves, pan } = setup();
    pan.onPointerDown(down);
    pan.onPointerMove({ clientX: 5, clientY: 5 });
    expect(drag.current).toEqual({ x: 5, y: 5 });
    expect(moves).toHaveLength(1);
  });

  it("stops panning when the pointer is cancelled or capture is lost", () => {
    for (const end of ["onPointerCancel", "onLostPointerCapture"] as const) {
      const { drag, moves, pan } = setup();
      pan.onPointerDown(down);
      pan[end](down);
      pan.onPointerMove({ clientX: 9, clientY: 9 });
      expect(drag.current).toBeNull();
      expect(moves).toHaveLength(0);
    }
  });
});

describe("fitView", () => {
  it("centres content that is smaller than the box at scale 1", () => {
    const v = fitView({ width: 200, height: 100 }, { width: 600, height: 400 });
    expect(v).toEqual({ scale: 1, x: 200, y: 150 });
  });

  it("shrinks content that does not fit, keeping the padding", () => {
    const v = fitView({ width: 1000, height: 500 }, { width: 564, height: 400 }, 32);
    expect(v.scale).toBeCloseTo(0.5);
    expect(v.x).toBeCloseTo(32);
    expect(v.y).toBeCloseTo((400 - 250) / 2);
  });

  it("never goes below 0.3", () => {
    expect(fitView({ width: 10000, height: 100 }, { width: 500, height: 500 }).scale).toBe(0.3);
  });

  it("handles an empty box without NaN", () => {
    const v = fitView({ width: 100, height: 100 }, { width: 0, height: 0 });
    expect(Number.isFinite(v.scale) && Number.isFinite(v.x) && Number.isFinite(v.y)).toBe(true);
  });
});

describe("zoomAt", () => {
  it("keeps the point under the cursor fixed", () => {
    const view = { scale: 1, x: 40, y: 20 };
    const point = { x: 300, y: 200 };
    const next = zoomAt(view, 1.5, point);
    expect(next.scale).toBeCloseTo(1.5);
    // content coordinate under the point before and after
    expect((point.x - view.x) / view.scale).toBeCloseTo((point.x - next.x) / next.scale);
    expect((point.y - view.y) / view.scale).toBeCloseTo((point.y - next.y) / next.scale);
  });

  it("clamps the scale to [0.3, 2]", () => {
    expect(zoomAt({ scale: 1.9, x: 0, y: 0 }, 2, { x: 0, y: 0 }).scale).toBe(2);
    expect(zoomAt({ scale: 0.35, x: 0, y: 0 }, 0.5, { x: 0, y: 0 }).scale).toBe(0.3);
  });

  it("does not move when the scale is already at the limit", () => {
    const view = { scale: 2, x: 10, y: 10 };
    expect(zoomAt(view, 1.2, { x: 50, y: 50 })).toEqual(view);
  });
});
