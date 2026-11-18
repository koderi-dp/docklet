import { useCallback, useEffect, useRef, useState } from "react";
import { Events } from "@wailsio/runtime";
import { ContainerService } from "@bindings/services";
import type { LogLine } from "@bindings/model";
import { errorMessage } from "@/lib/errors";
import { makeEntry, type LogEntry } from "@/lib/logEntry";
import { capEntries, mergeEntries } from "@/lib/logMerge";
import type { StreamStatus } from "@/components/containers/useLogStream";

export interface LogTarget {
  id: string;
  name: string;
}

export interface StreamInfo {
  status: StreamStatus;
  error?: string;
}

const FLUSH_MS = 150;
/** Events whose stream id is not known yet are held briefly; anything beyond this many stray streams is ignored. */
const MAX_UNKNOWN_STREAMS = 64;

interface DataPayload {
  streamId: string;
  lines?: LogLine[] | null;
}
interface EndPayload {
  streamId: string;
  error?: string;
}
const unwrap = <T,>(ev: { data?: unknown }): T | undefined => (Array.isArray(ev.data) ? ev.data[0] : ev.data) as T | undefined;

interface Live {
  name: string;
  streamId: string | null;
  cancelled: boolean;
}

/**
 * One merged log buffer fed by one backend stream per target container. Incoming lines are batched and merged by
 * Engine timestamp every FLUSH_MS, so a noisy container never renders per line. Streams of containers that are
 * removed from `targets` are stopped (and their lines dropped); ALL streams are stopped on unmount.
 */
export function useMergedLogStreams(connectionId: string, targets: readonly LogTarget[], tail: number) {
  const [lines, setLines] = useState<LogEntry[]>([]);
  const [dropped, setDropped] = useState(0);
  const [infos, setInfos] = useState<Record<string, StreamInfo>>({});
  const [paused, setPausedState] = useState(false);
  const [unseen, setUnseen] = useState(0);

  const buffer = useRef<LogEntry[]>([]);
  const pending = useRef<LogEntry[]>([]);
  const seq = useRef(0);
  const lastKey = useRef(new Map<string, number>());
  const live = useRef(new Map<string, Live>());
  const byStream = useRef(new Map<string, string>());
  const unknown = useRef(new Map<string, LogLine[]>());
  const pausedRef = useRef(false);
  const unseenRef = useRef(0);
  const tailRef = useRef(tail);
  tailRef.current = tail;
  const targetsRef = useRef(targets);
  targetsRef.current = targets;

  const setInfo = useCallback((id: string, info: StreamInfo) => setInfos((p) => ({ ...p, [id]: info })), []);

  const ingest = useCallback((name: string, raw: readonly LogLine[]) => {
    if (raw.length === 0) return;
    for (const l of raw) {
      const fallback = lastKey.current.get(name) ?? Date.now();
      const e = makeEntry(l, name, seq.current++, fallback);
      lastKey.current.set(name, e.sortKey);
      pending.current.push(e);
    }
  }, []);

  // Batched publishing.
  useEffect(() => {
    const t = setInterval(() => {
      if (pending.current.length === 0) return;
      const batch = pending.current;
      pending.current = [];
      const capped = capEntries(mergeEntries(buffer.current, batch));
      buffer.current = capped.lines;
      if (capped.dropped) setDropped((d) => d + capped.dropped);
      if (pausedRef.current) {
        unseenRef.current += batch.length;
        setUnseen(unseenRef.current);
      } else setLines(buffer.current);
    }, FLUSH_MS);
    return () => clearInterval(t);
  }, []);

  // One pair of event listeners for all streams.
  useEffect(() => {
    const offData = Events.On("containerlogs:data", (ev: { data?: unknown }) => {
      const p = unwrap<DataPayload>(ev);
      if (!p?.streamId) return;
      const id = byStream.current.get(p.streamId);
      const l = id ? live.current.get(id) : undefined;
      if (l) {
        ingest(l.name, p.lines ?? []);
      } else if (unknown.current.has(p.streamId) || unknown.current.size < MAX_UNKNOWN_STREAMS) {
        unknown.current.set(p.streamId, (unknown.current.get(p.streamId) ?? []).concat(p.lines ?? []));
      }
    });
    const offEnd = Events.On("containerlogs:end", (ev: { data?: unknown }) => {
      const p = unwrap<EndPayload>(ev);
      const id = p?.streamId ? byStream.current.get(p.streamId) : undefined;
      if (!p || !id) return;
      setInfo(id, p.error ? { status: "error", error: p.error } : { status: "ended" });
    });
    return () => {
      offData();
      offEnd();
    };
  }, [ingest, setInfo]);

  // Start / stop streams to match the targets.
  const key = targets.map((t) => t.id).join(",");
  useEffect(() => {
    const wanted = new Map(targetsRef.current.map((t) => [t.id, t]));
    const removed: string[] = [];
    for (const [id, l] of live.current) {
      if (wanted.has(id)) continue;
      l.cancelled = true;
      if (l.streamId) {
        void ContainerService.StopLogStream(l.streamId).catch(() => undefined);
        byStream.current.delete(l.streamId);
      }
      live.current.delete(id);
      removed.push(l.name);
    }
    if (removed.length) {
      const gone = new Set(removed);
      for (const n of removed) lastKey.current.delete(n);
      buffer.current = buffer.current.filter((e) => !gone.has(e.container));
      pending.current = pending.current.filter((e) => !gone.has(e.container));
      setLines(buffer.current);
      setInfos((p) => Object.fromEntries(Object.entries(p).filter(([id]) => wanted.has(id))));
    }
    for (const t of wanted.values()) {
      if (live.current.has(t.id)) continue;
      const l: Live = { name: t.name, streamId: null, cancelled: false };
      live.current.set(t.id, l);
      setInfo(t.id, { status: "connecting" });
      ContainerService.StartLogStream(connectionId, t.id, tailRef.current, true)
        .then((sid) => {
          if (l.cancelled) {
            void ContainerService.StopLogStream(sid).catch(() => undefined);
            return;
          }
          l.streamId = sid;
          byStream.current.set(sid, t.id);
          const early = unknown.current.get(sid);
          unknown.current.delete(sid);
          if (early) ingest(l.name, early);
          setInfos((p) => (p[t.id]?.status === "connecting" ? { ...p, [t.id]: { status: "live" } } : p));
        })
        .catch((e: unknown) => {
          if (!l.cancelled) setInfo(t.id, { status: "error", error: errorMessage(e) });
        });
    }
  }, [connectionId, key, ingest, setInfo]);

  // Leaving the screen stops everything.
  useEffect(() => {
    const liveStreams = live.current;
    const streamMap = byStream.current;
    return () => {
      for (const l of liveStreams.values()) {
        l.cancelled = true;
        if (l.streamId) void ContainerService.StopLogStream(l.streamId).catch(() => undefined);
      }
      liveStreams.clear();
      streamMap.clear();
    };
  }, []);

  const setPaused = useCallback((p: boolean) => {
    pausedRef.current = p;
    setPausedState(p);
    if (!p) {
      unseenRef.current = 0;
      setUnseen(0);
      setLines(buffer.current);
    }
  }, []);

  /** Clears the VIEW only; the containers' logs are untouched. */
  const clear = useCallback(() => {
    buffer.current = [];
    pending.current = [];
    unseenRef.current = 0;
    setUnseen(0);
    setDropped(0);
    setLines([]);
  }, []);

  return { lines, dropped, infos, paused, setPaused, unseen, clear };
}
