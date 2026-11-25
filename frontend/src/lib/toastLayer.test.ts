import { describe, expect, it, vi } from "vitest";
import { failureToastWanted, keepOpenForToasts, opToastId } from "./toastLayer";

/** A stand-in for an element: closest() answers for the toaster selector only when inside it. */
const el = (insideToaster: boolean) => ({ closest: (sel: string) => (insideToaster && sel === "[data-sonner-toaster]" ? {} : null) });

describe("keepOpenForToasts", () => {
  it("stops a dialog from closing when the press landed on a toast", () => {
    const ev = { target: el(true), preventDefault: vi.fn() };
    keepOpenForToasts(ev);
    expect(ev.preventDefault).toHaveBeenCalledOnce();
  });

  it("reads the original target of a Radix outside event", () => {
    const ev = { target: el(false), detail: { originalEvent: { target: el(true) } }, preventDefault: vi.fn() };
    keepOpenForToasts(ev);
    expect(ev.preventDefault).toHaveBeenCalledOnce();
  });

  it("lets a press anywhere else close the dialog", () => {
    const ev = { target: el(false), preventDefault: vi.fn() };
    keepOpenForToasts(ev);
    expect(ev.preventDefault).not.toHaveBeenCalled();
  });

  it("ignores a target that is not an element", () => {
    const ev = { target: null, preventDefault: vi.fn() };
    keepOpenForToasts(ev);
    expect(ev.preventDefault).not.toHaveBeenCalled();
  });
});

describe("failureToastWanted", () => {
  it("shows the toast while the Operations panel is closed", () => expect(failureToastWanted({ panelOpen: false, selectedId: "a" }, "a")).toBe(true));
  it("shows it when the panel shows another operation", () => expect(failureToastWanted({ panelOpen: true, selectedId: "b" }, "a")).toBe(true));
  it("skips it when the panel already shows this operation", () => expect(failureToastWanted({ panelOpen: true, selectedId: "a" }, "a")).toBe(false));
});

describe("opToastId", () => {
  it("is stable per operation", () => expect(opToastId("op-1")).toBe(opToastId("op-1")));
  it("differs between operations", () => expect(opToastId("op-1")).not.toBe(opToastId("op-2")));
});
