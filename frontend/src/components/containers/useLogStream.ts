import { useCallback, useEffect, useRef, useState } from "react";
import { Events } from "@wailsio/runtime";
import { ContainerService } from "@bindings/services";
import type { LogLine } from "@bindings/model";
import { errorMessage } from "@/lib/errors";

/** Hard cap on lines kept in React state (and in the paused queue). */
export const MAX_LOG_LINES = 5000;

export interface ViewLine extends LogLine {
  /** Monotonic id, stable across trimming (React key). */
  seq: number;
}

export type StreamStatus = "connecting" | "live" | "ended" | "error";

interface DataPayload {
  streamId: string;
  lines?: LogLine[] | null;
}
interface EndPayload {
  streamId: string;
  error?: string;
}

function unwrap<T>(ev: { data?: unknown }): T | undefined {
  const raw = Array.isArray(ev.data) ? ev.data[0] : ev.data;
  return raw as T | undefined;
}

interface Options {
  connectionId: string;
  containerId: string;
  tail: number;
  timestamps: boolean;
  /** When false no stream is opened. */
  enabled?: boolean;
}

/**
 * Live container log stream. Opens a backend stream, appends batched lines to a capped
 * buffer, and ALWAYS stops the backend stream on unmount / option change.
 * `paused` freezes the visible buffer while the stream keeps running (lines queue, capped).
 */
export function useLogStream({ connectionId, containerId, tail, timestamps, enabled = true }: Options) {
  const [lines, setLines] = useState<ViewLine[]>([]);
  const [status, setStatus] = useState<StreamStatus>("connecting");
  const [error, setError] = useState<string | null>(null);
  const [paused, setPausedState] = useState(false);
  const [queued, setQueued] = useState(0);
  const [attempt, setAttempt] = useState(0);

  const pausedRef = useRef(false);
  const queueRef = useRef<ViewLine[]>([]);
  const seqRef = useRef(0);

  const append = useCallback((incoming: LogLine[]) => {
    if (incoming.length === 0) return;
    const withSeq = incoming.map((l) => ({ ...l, seq: seqRef.current++ }));
    if (pausedRef.current) {
      const q = queueRef.current.concat(withSeq);
      queueRef.current = q.length > MAX_LOG_LINES ? q.slice(-MAX_LOG_LINES) : q;
      setQueued(queueRef.current.length);
      return;
    }
    setLines((prev) => {
      const next = prev.length + withSeq.length > MAX_LOG_LINES ? prev.concat(withSeq).slice(-MAX_LOG_LINES) : prev.concat(withSeq);
      return next;
    });
  }, []);

  useEffect(() => {
    if (!enabled) return;
    let cancelled = false;
    let streamId: string | null = null;
    // Events can arrive before StartLogStream resolves; hold them until the id is known.
    const early: Array<{ kind: "data"; p: DataPayload } | { kind: "end"; p: EndPayload }> = [];
    setLines([]);
    queueRef.current = [];
    setQueued(0);
    setStatus("connecting");
    setError(null);

    const handleData = (p: DataPayload) => {
      setStatus((s) => (s === "connecting" ? "live" : s));
      append(p.lines ?? []);
    };
    const handleEnd = (p: EndPayload) => {
      if (p.error) {
        setError(p.error);
        setStatus("error");
      } else {
        setStatus("ended");
      }
    };

    const offData = Events.On("containerlogs:data", (ev: { data?: unknown }) => {
      const p = unwrap<DataPayload>(ev);
      if (!p || cancelled) return;
      if (streamId === null) early.push({ kind: "data", p });
      else if (p.streamId === streamId) handleData(p);
    });
    const offEnd = Events.On("containerlogs:end", (ev: { data?: unknown }) => {
      const p = unwrap<EndPayload>(ev);
      if (!p || cancelled) return;
      if (streamId === null) early.push({ kind: "end", p });
      else if (p.streamId === streamId) handleEnd(p);
    });

    ContainerService.StartLogStream(connectionId, containerId, tail, timestamps)
      .then((id) => {
        if (cancelled) {
          void ContainerService.StopLogStream(id).catch(() => undefined);
          return;
        }
        streamId = id;
        setStatus((s) => (s === "connecting" ? "live" : s));
        for (const e of early) {
          if (e.p.streamId !== id) continue;
          if (e.kind === "data") handleData(e.p);
          else handleEnd(e.p);
        }
        early.length = 0;
      })
      .catch((e: unknown) => {
        if (cancelled) return;
        setError(errorMessage(e));
        setStatus("error");
      });

    return () => {
      cancelled = true;
      offData();
      offEnd();
      if (streamId) void ContainerService.StopLogStream(streamId).catch(() => undefined);
    };
  }, [connectionId, containerId, tail, timestamps, enabled, attempt, append]);

  const setPaused = useCallback((p: boolean) => {
    pausedRef.current = p;
    setPausedState(p);
    if (!p && queueRef.current.length) {
      const q = queueRef.current;
      queueRef.current = [];
      setQueued(0);
      setLines((prev) => prev.concat(q).slice(-MAX_LOG_LINES));
    }
  }, []);

  /** Clears the VIEW only; the container's logs are untouched. */
  const clear = useCallback(() => {
    setLines([]);
    queueRef.current = [];
    setQueued(0);
  }, []);

  const reconnect = useCallback(() => setAttempt((n) => n + 1), []);

  return { lines, status, error, paused, setPaused, queued, clear, reconnect };
}
