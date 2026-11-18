import { useUI } from "@/stores/uiStore";
import { useCallback, useEffect, useRef, useState } from "react";
import { Loader2, Plus, RotateCw } from "lucide-react";
import { Terminal } from "@xterm/xterm";
import { FitAddon } from "@xterm/addon-fit";
import "@xterm/xterm/css/xterm.css";
import { Events } from "@wailsio/runtime";
import { ExecService } from "@bindings/services";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { fromBase64, readMonoFont, readTerminalTheme, toBase64 } from "@/components/terminal/util";
import { errorMessage } from "@/lib/errors";

export interface TerminalPanelProps {
  connectionId: string;
  containerId: string;
  running: boolean;
}

type Phase =
  | { kind: "idle" }
  | { kind: "connecting" }
  | { kind: "active"; shell: string }
  | { kind: "exited"; code: number }
  | { kind: "disconnected"; error: string };

const SHELLS = ["auto", "bash", "sh", "ash", "zsh"] as const;

interface ExecData {
  sessionId: string;
  dataBase64: string;
}
interface ExecExit {
  sessionId: string;
  exitCode?: number | null;
  error?: string;
}


export default function TerminalPanel({ connectionId, containerId, running }: TerminalPanelProps) {
  const hostRef = useRef<HTMLDivElement>(null);
  const startRef = useRef<(() => void) | null>(null);
  const shellRef = useRef<string>("auto");
  const [shell, setShell] = useState<string>("auto");
  const [phase, setPhase] = useState<Phase>({ kind: "idle" });

  useEffect(() => {
    shellRef.current = shell;
  }, [shell]);

  useEffect(() => {
    const host = hostRef.current;
    if (!running || !host) {
      setPhase({ kind: "idle" });
      return;
    }
    let disposed = false;
    let sessionId: string | null = null;
    let generation = 0;
    let sendChain: Promise<unknown> = Promise.resolve();
    const early: ExecData[] = [];

    const term = new Terminal({
      fontFamily: readMonoFont(),
      fontSize: useUI.getState().terminalFontSize,
      cursorBlink: true,
      scrollback: 5000,
      theme: readTerminalTheme(),
    });
    const fit = new FitAddon();
    term.loadAddon(fit);
    term.open(host);

    const doFit = () => {
      try {
        fit.fit();
      } catch {
        /* not laid out yet */
      }
    };
    doFit();

    // Keys typed in the terminal must never reach app shortcuts.
    const stop = (e: KeyboardEvent) => e.stopPropagation();
    host.addEventListener("keydown", stop);
    host.addEventListener("keyup", stop);
    host.addEventListener("keypress", stop);

    const copySel = () => {
      const sel = term.getSelection();
      if (sel) void navigator.clipboard.writeText(sel).catch(() => undefined);
    };
    term.attachCustomKeyEventHandler((e) => {
      if (e.type !== "keydown") return true;
      const k = e.key.toLowerCase();
      if (e.ctrlKey && e.shiftKey && k === "c") {
        copySel();
        return false;
      }
      if (e.ctrlKey && e.shiftKey && k === "v") {
        void navigator.clipboard
          .readText()
          .then((t) => t && term.paste(t))
          .catch(() => undefined);
        return false;
      }
      // Ctrl+C copies when text is selected; otherwise it is the interrupt (^C).
      if (e.ctrlKey && !e.shiftKey && !e.altKey && k === "c" && term.hasSelection()) {
        copySel();
        return false;
      }
      return true;
    });

    const dataSub = term.onData((data) => {
      const id = sessionId;
      if (!id) return;
      sendChain = sendChain.then(() => ExecService.Write(id, toBase64(data))).catch(() => undefined);
    });
    const resizeSub = term.onResize(({ cols, rows }) => {
      if (sessionId) void ExecService.Resize(sessionId, cols, rows).catch(() => undefined);
    });

    const offData = Events.On("exec:data", (ev: { data: ExecData }) => {
      const d = ev.data;
      if (!d) return;
      if (sessionId === null) {
        early.push(d); // may arrive before Start() resolves
        return;
      }
      if (d.sessionId === sessionId) term.write(fromBase64(d.dataBase64));
    });
    const offExit = Events.On("exec:exit", (ev: { data: ExecExit }) => {
      const d = ev.data;
      if (!d || d.sessionId !== sessionId) return;
      sessionId = null;
      if (d.exitCode == null || d.error) {
        const error = d.error || "The terminal session ended unexpectedly.";
        term.write(`\r\n\x1b[2m[${error}]\x1b[0m\r\n`);
        setPhase({ kind: "disconnected", error });
      } else {
        term.write(`\r\n\x1b[2m[Process exited with code ${d.exitCode}]\x1b[0m\r\n`);
        setPhase({ kind: "exited", code: d.exitCode });
      }
    });

    const closeCurrent = () => {
      const id = sessionId;
      sessionId = null;
      if (id) void ExecService.Close(id).catch(() => undefined);
    };

    const start = () => {
      closeCurrent();
      const gen = ++generation;
      early.length = 0;
      term.reset();
      doFit();
      setPhase({ kind: "connecting" });
      ExecService.Start(connectionId, containerId, {
        shell: shellRef.current === "auto" ? "" : shellRef.current,
        cols: term.cols,
        rows: term.rows,
        user: "",
        workDir: "",
      })
        .then((s) => {
          if (disposed || gen !== generation) {
            void ExecService.Close(s.id).catch(() => undefined);
            return;
          }
          sessionId = s.id;
          for (const d of early.splice(0)) if (d.sessionId === s.id) term.write(fromBase64(d.dataBase64));
          setPhase({ kind: "active", shell: s.shell });
          term.focus();
          // the size may have changed while connecting
          if (term.cols && term.rows) void ExecService.Resize(s.id, term.cols, term.rows).catch(() => undefined);
        })
        .catch((e) => {
          if (disposed || gen !== generation) return;
          setPhase({ kind: "disconnected", error: errorMessage(e) });
        });
    };
    startRef.current = start;

    let raf = 0;
    const ro = new ResizeObserver(() => {
      cancelAnimationFrame(raf);
      raf = requestAnimationFrame(doFit);
    });
    ro.observe(host);

    const themeObs = new MutationObserver(() => {
      term.options.theme = readTerminalTheme();
    });
    themeObs.observe(document.documentElement, { attributes: true, attributeFilter: ["class"] });

    start();

    return () => {
      disposed = true;
      generation++;
      startRef.current = null;
      cancelAnimationFrame(raf);
      ro.disconnect();
      themeObs.disconnect();
      offData();
      offExit();
      dataSub.dispose();
      resizeSub.dispose();
      host.removeEventListener("keydown", stop);
      host.removeEventListener("keyup", stop);
      host.removeEventListener("keypress", stop);
      closeCurrent();
      term.dispose();
    };
  }, [connectionId, containerId, running]);

  const restart = useCallback(() => startRef.current?.(), []);

  if (!running) {
    return <p className="p-4 text-sm text-muted-foreground">The container is not running. Start it to open a terminal.</p>;
  }

  const ended = phase.kind === "exited" || phase.kind === "disconnected";
  return (
    <div className="flex h-full min-h-[240px] flex-col gap-2">
      <div className="flex flex-wrap items-center gap-2 text-[13px]">
        <span className="inline-flex items-center gap-1.5" role="status">
          {phase.kind === "connecting" ? (
            <Loader2 className="size-3.5 animate-spin text-muted-foreground" />
          ) : (
            <span
              className={cn(
                "size-2 rounded-full",
                phase.kind === "active" && "bg-success",
                phase.kind === "disconnected" && "bg-destructive",
                (phase.kind === "exited" || phase.kind === "idle") && "bg-muted-foreground",
              )}
            />
          )}
          {phase.kind === "connecting" && "Connecting…"}
          {phase.kind === "active" && (
            <>
              Connected <span className="font-mono text-xs text-muted-foreground">{phase.shell}</span>
            </>
          )}
          {phase.kind === "exited" && `Process exited with code ${phase.code}`}
          {phase.kind === "disconnected" && <span className="text-destructive">Disconnected: {phase.error}</span>}
        </span>
        <div className="ml-auto flex items-center gap-2">
          <label className="flex items-center gap-1.5 text-muted-foreground">
            Shell
            <select
              value={shell}
              onChange={(e) => setShell(e.target.value)}
              className="h-8 rounded-md border border-border bg-card px-2 text-[13px] text-foreground"
              aria-label="Shell (applies to the next session)"
              title="Applies to the next session"
            >
              {SHELLS.map((s) => (
                <option key={s} value={s}>
                  {s}
                </option>
              ))}
            </select>
          </label>
          <Button size="sm" variant="outline" onClick={restart} disabled={phase.kind === "connecting"}>
            {ended ? <RotateCw className="size-4" /> : <Plus className="size-4" />}
            {ended ? "Reconnect" : "New session"}
          </Button>
        </div>
      </div>
      <div className="min-h-0 flex-1 overflow-hidden rounded-[10px] border border-border bg-card p-2">
        <div ref={hostRef} className="h-full w-full" />
      </div>
    </div>
  );
}
