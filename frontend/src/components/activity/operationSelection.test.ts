import { beforeEach, describe, expect, it } from "vitest";
import { OperationStatus, type Operation } from "@bindings/model";
import { useOpsStore } from "@/hooks/useOperations";
import { pickSelected } from "./operationSelection";

const op = (id: string, status: OperationStatus, startedAt = 0) => ({ id, status, startedAt }) as Operation;
const running = (id: string) => op(id, OperationStatus.OpRunning);
const done = (id: string) => op(id, OperationStatus.OpSucceeded);

describe("pickSelected", () => {
  it("keeps the wanted operation, live or earlier, over a running one", () => {
    const live = [running("r"), done("d")];
    const earlier = [done("old")];
    expect(pickSelected("d", live, earlier)?.id).toBe("d");
    expect(pickSelected("old", live, earlier)?.id).toBe("old");
  });

  it("falls back to the newest running operation when nothing is wanted or the wanted one is gone", () => {
    const live = [done("newest"), running("r1"), running("r2")];
    expect(pickSelected(null, live, [])?.id).toBe("r1");
    expect(pickSelected("cleared", live, [])?.id).toBe("r1");
  });

  it("falls back to the newest of this session, then the newest earlier one, then nothing", () => {
    expect(pickSelected(null, [done("a"), done("b")], [done("old")])?.id).toBe("a");
    expect(pickSelected(null, [], [done("old"), done("older")])?.id).toBe("old");
    expect(pickSelected(null, [], [])).toBeNull();
  });
});

describe("operations panel state", () => {
  beforeEach(() => useOpsStore.setState({ ops: {}, logs: {}, panelOpen: false, selectedId: null }));

  it("opens on the given operation and keeps the selection when opened without one", () => {
    useOpsStore.getState().openPanel("a");
    expect(useOpsStore.getState()).toMatchObject({ panelOpen: true, selectedId: "a" });
    useOpsStore.getState().closePanel();
    useOpsStore.getState().openPanel();
    expect(useOpsStore.getState()).toMatchObject({ panelOpen: true, selectedId: "a" });
  });

  it("drops the selection when the selected operation is cleared, and keeps it otherwise", () => {
    useOpsStore.getState().setAll([done("a"), done("b")]);
    useOpsStore.getState().select("a");
    useOpsStore.getState().remove(["b"]);
    expect(useOpsStore.getState().selectedId).toBe("a");
    useOpsStore.getState().remove(["a"]);
    expect(useOpsStore.getState().selectedId).toBeNull();
  });
});
