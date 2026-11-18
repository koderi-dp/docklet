import { useState, type ReactNode } from "react";
import { Browser } from "@wailsio/runtime";
import { ExternalLink, Info, Loader2, Play, RefreshCw } from "lucide-react";
import { ConnectionService } from "@bindings/services";
import { useInstallOperation } from "@/components/connections/wslinstall/useWslInstall";
import { InstallDockerFlow } from "@/components/connections/wslinstall/InstallDockerFlow";
import { Button } from "@/components/ui/button";
import { errorMessage, toastError } from "@/lib/errors";
import { setupNeed, type Candidate, type SetupNeed } from "@/lib/onboardingCandidates";
import { cn } from "@/lib/utils";
import { Panel, StepFrame } from "./StepFrame";
import type { Onboarding } from "./useOnboarding";

function Notice({ title, children, tone = "default" }: { title: string; children: ReactNode; tone?: "default" | "warning" }) {
  return (
    <Panel className={cn("p-5", tone === "warning" && "border-warning/40 bg-warning/10")}>
      <h2 className="text-sm font-semibold">{title}</h2>
      <div className="mt-1 space-y-2 text-sm text-muted-foreground">{children}</div>
    </Panel>
  );
}

const TITLES: Record<SetupNeed, string> = {
  none: "Ready to test",
  "start-distro": "Start the distribution",
  "start-docker": "Start Docker Engine",
  "install-docker": "Install Docker Engine",
  "install-guide": "Docker Engine is not installed",
  "docker-desktop": "Docker Desktop is not running",
};

/** Step 3: only shown when the chosen engine needs something first. Each action reports its real result and can be retried. */
export function SetupStep({ ob }: { ob: Onboarding }) {
  const c = ob.selected as Candidate;
  const need = setupNeed(c);
  const installing = !!useInstallOperation(c.distro ?? c.name);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const run = async (action: () => Promise<string>) => {
    setBusy(true);
    setError(null);
    try {
      ob.note(await action());
      await ob.recheck();
      ob.requestAdvance();
    } catch (e) {
      setError(errorMessage(e));
    } finally {
      setBusy(false);
    }
  };
  const distro = c.distro ?? c.name;
  const startDistro = () => run(async () => (await ConnectionService.StartWSLDistro(distro), `Started ${distro}`));
  const startDocker = () =>
    run(async () => {
      const info = await ConnectionService.StartDockerInWSL(distro);
      if (!info.daemonReachable) throw new Error(info.detail || "Docker did not answer after it was started.");
      return `Docker ${info.serverVersion} started`;
    });
  const openGuide = () => c.docsUrl && Browser.OpenURL(c.docsUrl).catch((e) => toastError("Could not open the browser", e));

  const primary = (label: string, onClick: () => void, icon: ReactNode) => (
    <Button data-primary onClick={onClick} disabled={busy}>
      {busy ? <Loader2 className="animate-spin" /> : icon}
      {label}
    </Button>
  );
  const recheck = (variant: "default" | "outline") => (
    <Button data-primary={variant === "default" ? true : undefined} variant={variant} onClick={() => void ob.recheck()} disabled={ob.detecting}>
      <RefreshCw className={cn(ob.detecting && "animate-spin")} />
      Check again
    </Button>
  );

  let body: ReactNode = null;
  let actions: ReactNode = null;
  switch (need) {
    case "start-distro":
      body = (
        <Notice title={`${distro} is stopped`}>
          <p>Docklet never starts a distribution just to look inside it. Starting it boots this WSL distribution so Docklet can look for Docker there. Nothing is installed or changed.</p>
        </Notice>
      );
      actions = primary(`Start ${distro}`, () => void startDistro(), <Play />);
      break;
    case "start-docker":
      body = (
        <Notice title={`Docker Engine is installed in ${distro} but not running`} tone="warning">
          <p>Starting it runs the Docker service as root inside {distro} (with systemctl when systemd is running, otherwise service docker start), then waits until the daemon answers.</p>
        </Notice>
      );
      actions = primary("Start Docker Engine", () => void startDocker(), <Play />);
      break;
    case "install-docker":
      body = (
        <InstallDockerFlow
          distro={distro}
          inline
          successAction={{ label: "Continue", run: () => {
              ob.note(`Installed Docker Engine in ${distro}`);
              void ob.recheck().then(() => ob.requestAdvance());
            } }}
          header={({ title, description, phase }) =>
            phase === "confirm" ? null : (
            <div>
              <h2 className="text-sm font-semibold">{title}</h2>
              <p className="mt-0.5 text-xs text-muted-foreground">{description}</p>
            </div>
            )
          }
        />
      );
      actions = installing ? null : recheck("outline");
      break;
    case "install-guide":
      body = (
        <Notice title={`${distro}: no automatic install`}>
          <p>{c.reason || "Docker Engine cannot be installed in this distribution automatically."}</p>
          {c.docsUrl && (
            <Button variant="outline" size="sm" onClick={openGuide}>
              <ExternalLink />
              Open install guide
            </Button>
          )}
        </Notice>
      );
      actions = recheck("default");
      break;
    case "docker-desktop":
      body = (
        <Notice title="Docker Desktop is not running" tone="warning">
          <p className="flex gap-1.5">
            <Info className="mt-0.5 size-3.5 shrink-0" />
            In {distro}, the docker command is Docker Desktop&apos;s integration. It answers only while Docker Desktop is running, and Docklet will not install a second engine over it.
          </p>
          <p>Start Docker Desktop, or choose another engine, then check again.</p>
        </Notice>
      );
      actions = recheck("default");
      break;
    default:
      body = <Notice title="Nothing left to set up">{c.name} is ready to be tested.</Notice>;
      actions = (
        <Button data-primary onClick={() => ob.advance({ text: [...new Set(ob.setupNotes)].join(", ") || "Done" })}>
          Continue
        </Button>
      );
  }

  return (
    <StepFrame
      title={TITLES[need]}
      lede={`${c.name} needs one thing before Docklet can connect to it.`}
      secondary={
        <Button variant="ghost" onClick={() => ob.dispatch({ type: "back" })}>
          Back
        </Button>
      }
      actions={actions}
    >
      {body}
      {error && (
        <p role="alert" className="mt-3 rounded-md border border-destructive/30 bg-destructive/10 p-3 text-sm break-words text-destructive">
          {error}
        </p>
      )}
    </StepFrame>
  );
}
