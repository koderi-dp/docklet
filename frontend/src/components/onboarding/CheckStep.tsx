import { useState, type ReactNode } from "react";
import { Check, CircleAlert, Loader2 } from "lucide-react";
import type { ConnectionStatus } from "@bindings/model";
import { ErrorWithDiagnostics } from "@/components/connections/DiagnosticsBlock";
import { formatBytes } from "@/components/containers/format";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Label } from "@/components/ui/label";
import type { Candidate } from "@/lib/onboardingCandidates";
import { engineVersionLabel } from "@/lib/engineKind";
import { HELLO_IMAGE } from "./helloTest";
import { Panel, StepFrame } from "./StepFrame";
import type { Onboarding } from "./useOnboarding";
import { useEngineCheck, type HelloState } from "./useEngineCheck";

function Facts({ st }: { st: ConnectionStatus }) {
  const rows: [string, string][] = [
    ["Engine", st.serverVersion ? engineVersionLabel(st.kind, st.serverVersion) : "Unknown"],
    ["API", st.apiVersion || "Unknown"],
    ["System", [st.os, st.arch].filter(Boolean).join("/") || "Unknown"],
    ["CPUs", st.ncpu ? String(st.ncpu) : "Unknown"],
    ["Memory", st.memTotal ? formatBytes(st.memTotal) : "Unknown"],
    ["Containers", String(st.containers)],
  ];
  return (
    <dl className="grid grid-cols-2 gap-x-6 gap-y-3 sm:grid-cols-3">
      {rows.map(([k, v]) => (
        <div key={k} className="min-w-0">
          <dt className="text-xs text-muted-foreground">{k}</dt>
          <dd className="truncate text-sm font-medium tabular-nums">{v}</dd>
        </div>
      ))}
    </dl>
  );
}

const STEP_TEXT = { pulling: `Pulling ${HELLO_IMAGE}…`, running: "Running the container…", removing: "Removing the container…" } as const;

function HelloReport({ hello }: { hello: HelloState }) {
  if (hello.kind === "idle") return null;
  if (hello.kind === "running")
    return (
      <p className="flex items-center gap-2 text-sm text-muted-foreground" role="status">
        <Loader2 className="size-4 animate-spin" /> {STEP_TEXT[hello.step]}
      </p>
    );
  const r = hello.result;
  return (
    <div role="status" className="space-y-2 text-sm">
      {r.ok ? (
        <p className="flex items-center gap-2 text-success">
          <Check className="size-4" strokeWidth={3} /> The test container ran and printed:
        </p>
      ) : (
        <p role="alert" className="flex items-start gap-2 text-warning">
          <CircleAlert className="mt-0.5 size-4 shrink-0" /> <span className="break-words">The test container did not work: {r.error}</span>
        </p>
      )}
      {r.line && <pre className="rounded-md border bg-muted/40 px-3 py-2 font-mono text-xs">{r.line}</pre>}
      <p className="text-xs text-muted-foreground">
        {r.leftover ? `The container could not be removed: ${r.leftover}` : "The container was removed again."} The {HELLO_IMAGE} image stays on the engine; remove it from the Images page if you do not want it.
      </p>
    </div>
  );
}

function Report({ title, children }: { title: ReactNode; children: ReactNode }) {
  return (
    <Panel className="p-5">
      <h2 className="mb-3 flex items-center gap-2 text-sm font-semibold">{title}</h2>
      {children}
    </Panel>
  );
}

/** Step 4: test the chosen engine, show what it reported, then save and connect it. */
export function CheckStep({ ob, candidate }: { ob: Onboarding; candidate: Candidate }) {
  const [withHello, setWithHello] = useState(false);
  const chk = useEngineCheck(candidate, (savedId) => ob.dispatch({ type: "saved", id: candidate.id, savedId }));
  const { test, connect, hello } = chk;
  const connected = connect.kind === "connected";
  const working = connect.kind === "saving" || connect.kind === "connecting" || hello.kind === "running";

  const go = async () => {
    const r = await chk.connectNow();
    if (!r) return;
    const result = { text: engineVersionLabel(r.status.kind, r.status.serverVersion) };
    if (!withHello) return ob.advance(result);
    ob.dispatch({ type: "result", step: "check", result });
    await chk.runHello(r.id);
  };

  let primary: ReactNode;
  if (test.kind === "failed") primary = <Button data-primary onClick={chk.retest}>Test again</Button>;
  else if (connected && hello.kind !== "idle") primary = <Button data-primary onClick={() => ob.dispatch({ type: "next" })} disabled={working}>Continue</Button>;
  else if (connect.kind === "failed") primary = <Button data-primary onClick={() => void go()}>Try again</Button>;
  else
    primary = (
      <Button data-primary onClick={() => void go()} disabled={test.kind !== "ok" || working}>
        {working && <Loader2 className="animate-spin" />}
        {working ? "Connecting…" : candidate.savedId ? "Connect" : "Save and connect"}
      </Button>
    );

  return (
    <StepFrame
      title="Check the connection"
      lede={`Docklet tests ${candidate.name} before it saves or connects anything.`}
      secondary={
        <Button variant="ghost" onClick={() => ob.dispatch({ type: "back" })} disabled={working}>
          Back
        </Button>
      }
      actions={primary}
    >
      <div className="space-y-3">
        {test.kind === "testing" && (
          <Report title="Testing the connection…">
            <p className="flex items-center gap-2 text-sm text-muted-foreground" role="status">
              <Loader2 className="size-4 animate-spin" /> Asking {candidate.name} for its version and resources.
            </p>
          </Report>
        )}
        {test.kind === "failed" && <ErrorWithDiagnostics message={test.message} diagnostics={test.diagnostics} />}
        {test.kind === "ok" && (
          <Report
            title={
              <>
                <span className="flex size-5 items-center justify-center rounded-full bg-success/15 text-success">
                  <Check className="size-3.5" strokeWidth={3} />
                </span>
                {candidate.name} answers
              </>
            }
          >
            <Facts st={test.status} />
            {candidate.viaDesktop && <p className="mt-3 text-xs text-muted-foreground">This is Docker Desktop&apos;s engine, reached through its WSL integration.</p>}
          </Report>
        )}
        {test.kind === "ok" && connect.kind !== "failed" && !connected && (
          <>
            <div className="flex items-start gap-2.5 px-1">
              <Checkbox id="hello" checked={withHello} onCheckedChange={(v) => setWithHello(v === true)} disabled={working} className="mt-0.5" />
              <div>
                <Label htmlFor="hello" className="text-sm font-medium">
                  Run a test container (pulls the small hello-world image)
                </Label>
                <p className="mt-0.5 text-xs text-muted-foreground">
                  Optional. Pulls {HELLO_IMAGE}, runs it once, shows its output and removes the container. The image stays on the engine.
                </p>
              </div>
            </div>
            <p className="px-1 text-xs text-muted-foreground">
              {candidate.savedId ? "This engine is already in your connections." : `Docklet will save this engine as "${candidate.profile.name}" in your connections and connect to it.`}
            </p>
          </>
        )}
        {connect.kind === "failed" && <ErrorWithDiagnostics message={connect.message} diagnostics={connect.diagnostics} />}
        {connected && hello.kind === "idle" && (
          <p role="status" className="flex items-center gap-2 text-sm text-success">
            <Check className="size-4" strokeWidth={3} /> Connected.
          </p>
        )}
        <HelloReport hello={hello} />
      </div>
    </StepFrame>
  );
}
