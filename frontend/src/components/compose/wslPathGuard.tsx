import { useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { ArrowRight } from "lucide-react";
import { toast } from "sonner";
import { create } from "zustand";
import { ComposeService } from "@bindings/services";
import type { ComposeWSLPathIssue, ComposeWSLPathReport } from "@bindings/model";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { errorMessage } from "@/lib/errors";

interface Pending {
  connectionId: string;
  project: string;
  profiles: string[];
  report: ComposeWSLPathReport;
  resolve: (go: boolean) => void;
}

const usePending = create<{ pending: Pending | null }>(() => ({ pending: null }));

/**
 * Before `compose up` on a WSL connection: finds bind mounts written as Windows paths, which the
 * Linux engine in the distribution refuses. Resolves true to go ahead: at once when there are
 * none (no dialog), after the fix, or after "Start as it is". A check that cannot run never
 * blocks the start.
 */
export async function confirmWSLPaths(connectionId: string, project: string, profiles: string[]): Promise<boolean> {
  let report: ComposeWSLPathReport;
  try {
    report = await ComposeService.WSLPathIssues(connectionId, project, profiles);
  } catch {
    return true;
  }
  if (!report.issues?.length) return true;
  return new Promise<boolean>((resolve) => usePending.setState({ pending: { connectionId, project, profiles, report, resolve } }));
}

function Issue({ issue: i }: { issue: ComposeWSLPathIssue }) {
  return (
    <li className="min-w-0 space-y-1.5 px-3 py-2.5">
      <div className="flex flex-wrap items-center gap-x-2 gap-y-0.5 font-mono text-xs">
        <span className="break-all">{i.source}</span>
        {i.linuxPath && (
          <>
            <ArrowRight className="size-3.5 shrink-0 text-muted-foreground" aria-hidden />
            <span className="break-all text-foreground">{i.linuxPath}</span>
          </>
        )}
      </div>
      <div className="text-xs text-muted-foreground">
        Service <span className="font-medium text-foreground">{i.service}</span>, mounted at <span className="font-mono">{i.target}</span>
        {i.variable && !i.problem && (
          <>
            {" "}
            · variable <span className="font-mono text-foreground">{i.variable}</span>
          </>
        )}
      </div>
      {i.problem && <p className="text-xs text-warning">{i.problem}</p>}
      {!i.problem && i.newLine !== i.oldLine && (
        <div className="space-y-1 rounded border border-border bg-muted/40 px-2 py-1.5 font-mono text-[11px] leading-5">
          <div className="break-all text-muted-foreground line-through decoration-muted-foreground/50">{i.oldLine.trim()}</div>
          <div className="break-all text-foreground">{i.newLine.trim()}</div>
        </div>
      )}
      {!i.problem && i.newLine === i.oldLine && <p className="text-xs text-muted-foreground">The Compose file already uses this variable; only its value is set.</p>}
    </li>
  );
}

/** What the dialog says about a report (no data fetching, so it can be rendered on its own). */
export function WSLPathBody({ project, report }: { project: string; report: ComposeWSLPathReport }) {
  const issues = report.issues ?? [];
  const n = issues.length;
  return (
    <div className="min-w-0 space-y-3 text-sm">
      <p className="text-muted-foreground">
        Project <span className="font-medium text-foreground">{project}</span> runs inside a WSL distribution, where the engine is Linux. It refuses {n === 1 ? "this mount" : "these mounts"} because{" "}
        {n === 1 ? "it is" : "they are"} written as {n === 1 ? "a Windows path" : "Windows paths"}.
      </p>
      <ul className="max-h-[45vh] divide-y divide-border overflow-x-hidden overflow-y-auto rounded-md border border-border">
        {issues.map((i) => (
          <Issue key={`${i.service}:${i.target}`} issue={i} />
        ))}
      </ul>
      {report.fixable && (
        <p className="text-xs text-muted-foreground">
          The fix keeps the Windows path in the Compose file as the default, so the file works as before on Docker Desktop and for everyone else. The Linux path goes to{" "}
          <span className="break-all font-mono text-foreground">{report.envFile}</span>, which Docklet passes to Compose for this connection only.
        </p>
      )}
    </div>
  );
}

/** Mounted once in the app shell: shows the dialog of a pending Windows-path check. */
export function WSLPathGuardHost() {
  const pending = usePending((s) => s.pending);
  const qc = useQueryClient();
  const [fixing, setFixing] = useState(false);
  const [error, setError] = useState<string | null>(null);
  if (!pending) return null;
  const done = (go: boolean) => {
    usePending.setState({ pending: null });
    setError(null);
    pending.resolve(go);
  };
  const fix = async () => {
    setFixing(true);
    setError(null);
    try {
      await ComposeService.FixWSLPaths(pending.connectionId, pending.project, pending.profiles);
      void qc.invalidateQueries({ queryKey: ["compose", pending.connectionId] });
      toast.success(`Mounts of ${pending.project} now work on WSL`, { description: `The Linux paths are in ${pending.report.envFile}.` });
      done(true);
    } catch (e) {
      setError(errorMessage(e));
    } finally {
      setFixing(false);
    }
  };
  const n = pending.report.issues?.length ?? 0;
  return (
    <Dialog open onOpenChange={(o) => !o && !fixing && done(false)}>
      <DialogContent className="sm:max-w-xl">
        <DialogHeader>
          <DialogTitle>{n === 1 ? "A mount uses a Windows path" : `${n} mounts use Windows paths`}</DialogTitle>
          <DialogDescription>The project would fail to start as it is.</DialogDescription>
        </DialogHeader>
        <WSLPathBody project={pending.project} report={pending.report} />
        {error && <p className="whitespace-pre-wrap break-words text-xs text-destructive">{error}</p>}
        <DialogFooter className="gap-2">
          <Button variant="outline" size="sm" disabled={fixing} onClick={() => done(false)}>
            Cancel
          </Button>
          <Button variant="outline" size="sm" disabled={fixing} onClick={() => done(true)}>
            Start as it is
          </Button>
          {pending.report.fixable && (
            <Button size="sm" disabled={fixing} onClick={() => void fix()}>
              {fixing ? "Fixing…" : "Fix and start"}
            </Button>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
