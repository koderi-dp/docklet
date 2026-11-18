import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { QueryClient } from "@tanstack/react-query";
import { keepForConnection, queueInvalidate } from "@/hooks/queryUtils";

describe("keepForConnection", () => {
  const keep = keepForConnection("c1");
  it("keeps previous rows within the same connection", () => {
    expect(keep(["row"], { queryKey: ["logs", "c1", "id", 100] })).toEqual(["row"]);
  });
  it("never shows another engine's rows after a connection switch", () => {
    expect(keep(["row"], { queryKey: ["logs", "c2", "id", 100] })).toBeUndefined();
  });
});

describe("queueInvalidate", () => {
  beforeEach(() => vi.useFakeTimers());
  afterEach(() => vi.useRealTimers());
  it("collapses an event burst with the same id into a single invalidation", () => {
    const qc = new QueryClient();
    const spy = vi.spyOn(qc, "invalidateQueries").mockResolvedValue();
    for (let i = 0; i < 20; i++) queueInvalidate(qc, "containers:c1", { queryKey: ["containers", "c1"] });
    queueInvalidate(qc, "images:c1", { queryKey: ["images", "c1"] });
    expect(spy).not.toHaveBeenCalled();
    vi.advanceTimersByTime(500);
    expect(spy).toHaveBeenCalledTimes(2);
  });
});
