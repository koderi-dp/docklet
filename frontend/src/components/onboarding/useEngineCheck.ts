import { useCallback, useEffect, useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { ConnectionService } from "@bindings/services";
import { ConnectionState, type ConnectionStatus } from "@bindings/model";
import { connectionsKey, statusKey, useSelectConnection } from "@/hooks/useActiveConnection";
import { errorMessage } from "@/lib/errors";
import type { Candidate } from "@/lib/onboardingCandidates";
import { runHelloWorld, type HelloResult, type HelloStep } from "./helloTest";

export type TestState = { kind: "testing" } | { kind: "ok"; status: ConnectionStatus } | { kind: "failed"; message: string; diagnostics?: string };
export type ConnectState =
  | { kind: "idle" }
  | { kind: "saving" }
  | { kind: "connecting" }
  | { kind: "connected"; status: ConnectionStatus; connectionId: string }
  | { kind: "failed"; message: string; diagnostics?: string };
export type HelloState = { kind: "idle" } | { kind: "running"; step: HelloStep } | { kind: "done"; result: HelloResult };

/** Tests the chosen engine, then saves and connects it, and optionally runs the hello-world container. */
export function useEngineCheck(candidate: Candidate, onSaved: (savedId: string) => void) {
  const qc = useQueryClient();
  const select = useSelectConnection();
  const [attempt, setAttempt] = useState(0);
  const [test, setTest] = useState<TestState>({ kind: "testing" });
  const [connect, setConnect] = useState<ConnectState>({ kind: "idle" });
  const [hello, setHello] = useState<HelloState>({ kind: "idle" });

  useEffect(() => {
    let live = true;
    setTest({ kind: "testing" });
    setConnect({ kind: "idle" });
    setHello({ kind: "idle" });
    ConnectionService.Test({ ...candidate.profile, id: candidate.savedId ?? "" })
      .then((st) => {
        if (!live) return;
        setTest(st.state === ConnectionState.StateConnected ? { kind: "ok", status: st } : { kind: "failed", message: st.error || "The connection test failed.", diagnostics: st.diagnostics });
      })
      .catch((e) => live && setTest({ kind: "failed", message: errorMessage(e) }));
    return () => {
      live = false;
    };
    // The candidate is fixed while this step is shown.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [candidate.id, attempt]);

  /** Saves (when not saved yet) and connects. Resolves with the connection and its status on success. */
  const connectNow = useCallback(async (): Promise<{ id: string; status: ConnectionStatus } | null> => {
    try {
      let id = candidate.savedId;
      if (!id) {
        setConnect({ kind: "saving" });
        id = (await ConnectionService.Save({ ...candidate.profile, id: "" })).id;
        onSaved(id);
        await qc.invalidateQueries({ queryKey: connectionsKey });
      }
      setConnect({ kind: "connecting" });
      await select(id);
      const st = qc.getQueryData<ConnectionStatus>(statusKey(id));
      if (st?.state !== ConnectionState.StateConnected) {
        setConnect({ kind: "failed", message: st?.error || "Could not connect.", diagnostics: st?.diagnostics });
        return null;
      }
      setConnect({ kind: "connected", status: st, connectionId: id });
      return { id, status: st };
    } catch (e) {
      setConnect({ kind: "failed", message: errorMessage(e) });
      return null;
    }
  }, [candidate, onSaved, qc, select]);

  const runHello = useCallback(async (connectionId: string) => {
    setHello({ kind: "running", step: "pulling" });
    try {
      const result = await runHelloWorld(connectionId, (step) => setHello({ kind: "running", step }));
      setHello({ kind: "done", result });
    } catch (e) {
      setHello({ kind: "done", result: { ok: false, error: errorMessage(e) } });
    }
  }, []);

  return { test, connect, hello, connectNow, runHello, retest: () => setAttempt((n) => n + 1) };
}
