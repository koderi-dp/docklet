import { describe, expect, it, vi } from "vitest";
import type { LogEntry } from "@/lib/logEntry";
import { SearchRegistry } from "./searchRegistry";

const entries = (...seqs: number[]) => seqs.map((seq) => ({ seq }) as LogEntry);

describe("SearchRegistry reveal", () => {
  it("is false when no list is registered or none holds the line", () => {
    const r = new SearchRegistry();
    expect(r.reveal(1)).toBe(false);
    r.addReveal("a", () => false);
    expect(r.reveal(1)).toBe(false);
  });
  it("asks every list (no short-circuit) and is true when any one revealed it", () => {
    const r = new SearchRegistry();
    const first = vi.fn(() => true);
    const second = vi.fn(() => false);
    r.addReveal("a", first);
    r.addReveal("b", second);
    expect(r.reveal(5)).toBe(true);
    expect(first).toHaveBeenCalledWith(5);
    expect(second).toHaveBeenCalledWith(5);
  });
  it("unregistering removes the list", () => {
    const r = new SearchRegistry();
    const fn = vi.fn(() => true);
    const off = r.addReveal("a", fn);
    off();
    expect(r.reveal(1)).toBe(false);
    expect(fn).not.toHaveBeenCalled();
  });
  it("a stale unregister does not remove a list that re-registered under the same id", () => {
    const r = new SearchRegistry();
    const offOld = r.addReveal("a", () => false);
    r.addReveal("a", () => true);
    offOld();
    expect(r.reveal(1)).toBe(true);
  });
});

describe("SearchRegistry panes", () => {
  it("memberSeqs is null with no panes, a union of the panes otherwise", () => {
    const r = new SearchRegistry();
    expect(r.memberSeqs()).toBeNull();
    r.setPane(0, entries(1, 2, 3));
    r.setPane(1, entries(3, 4));
    expect([...r.memberSeqs()!].sort()).toEqual([1, 2, 3, 4]);
  });
  it("a registered but empty pane gives an empty set, not null", () => {
    const r = new SearchRegistry();
    r.setPane(0, []);
    expect(r.memberSeqs()).toEqual(new Set());
  });
  it("replacing a pane drops its old lines and null removes it", () => {
    const r = new SearchRegistry();
    r.setPane(0, entries(1, 2));
    r.setPane(0, entries(9));
    expect([...r.memberSeqs()!]).toEqual([9]);
    r.setPane(0, null);
    expect(r.memberSeqs()).toBeNull();
  });
});

describe("SearchRegistry notifications", () => {
  it("stores pane changes silently until watching is on", () => {
    const r = new SearchRegistry();
    const l = vi.fn();
    r.subscribe(l);
    r.setPane(0, entries(1));
    expect(l).not.toHaveBeenCalled();
    expect(r.getVersion()).toBe(0);
  });
  it("bumps the version and notifies every subscriber while watching, including on removal", () => {
    const r = new SearchRegistry();
    const a = vi.fn();
    const b = vi.fn();
    r.subscribe(a);
    r.subscribe(b);
    r.watching = true;
    r.setPane(0, entries(1));
    r.setPane(0, null);
    expect(r.getVersion()).toBe(2);
    expect(a).toHaveBeenCalledTimes(2);
    expect(b).toHaveBeenCalledTimes(2);
  });
  it("stops notifying after unsubscribe", () => {
    const r = new SearchRegistry();
    const l = vi.fn();
    const off = r.subscribe(l);
    r.watching = true;
    off();
    r.setPane(0, entries(1));
    expect(l).not.toHaveBeenCalled();
    expect(r.getVersion()).toBe(1);
  });
});
