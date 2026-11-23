import { describe, expect, it, vi } from "vitest";
import { QueryClient } from "@tanstack/react-query";
import { ConnectionState, type ConnectionStatus } from "@bindings/model";

vi.mock("@bindings/services", () => ({ ConnectionService: {} }));
vi.mock("@wailsio/runtime", () => ({ Events: { On: () => () => undefined } }));

import { connectAndTrack, statusKey } from "@/hooks/useActiveConnection";

const status = (state: ConnectionState) => ({ connectionId: "c1", state }) as ConnectionStatus;
const stateOf = (qc: QueryClient) => qc.getQueryData<ConnectionStatus>(statusKey("c1"))?.state;

describe("connectAndTrack", () => {
  // After a page reload the engine is still connected from before, so a status read that started
  // before the reconnect answers "connected" while the backend is already reconnecting.
  it("ignores a status read that was already under way when the reconnect started", async () => {
    const qc = new QueryClient();
    let answerRead!: (s: ConnectionStatus) => void;
    const read = qc.fetchQuery({ queryKey: statusKey("c1"), queryFn: () => new Promise<ConnectionStatus>((r) => (answerRead = r)) }).catch(() => undefined);
    await Promise.resolve();
    let answerConnect!: (s: ConnectionStatus) => void;
    const done = connectAndTrack(qc, "c1", () => new Promise<ConnectionStatus>((r) => (answerConnect = r)));
    await vi.waitFor(() => expect(answerConnect).toBeDefined());

    answerRead(status(ConnectionState.StateConnected));
    await read;
    expect(stateOf(qc)).toBe(ConnectionState.StateConnecting);

    answerConnect(status(ConnectionState.StateConnected));
    await done;
    expect(stateOf(qc)).toBe(ConnectionState.StateConnected);
  });

  it("records a failed attempt as an error status", async () => {
    const qc = new QueryClient();
    await connectAndTrack(qc, "c1", () => Promise.reject(new Error("no engine")));
    const st = qc.getQueryData<ConnectionStatus>(statusKey("c1"));
    expect(st?.state).toBe(ConnectionState.StateError);
    expect(st?.error).toContain("no engine");
  });
});
