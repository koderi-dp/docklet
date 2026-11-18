import { Browser } from "@wailsio/runtime";
import { Copy, ExternalLink } from "lucide-react";
import { Button } from "@/components/ui/button";
import { copyToClipboard } from "@/lib/clipboard";
import { toastError } from "@/lib/errors";
import { Panel } from "./StepFrame";

const WSL_DOCS = "https://learn.microsoft.com/windows/wsl/install";
const DOCKER_DOCS = "https://docs.docker.com/engine/install/";
const DOCKER_DESKTOP_MAC = "https://docs.docker.com/desktop/setup/install/mac-install/";

function openUrl(url: string) {
  Browser.OpenURL(url).catch((e) => toastError("Could not open the browser", e));
}

/** No engine and no distribution to put one in: what to do outside Docklet. Docklet never runs this itself. */
export function NothingFound({ os }: { os: string }) {
  if (os === "windows") {
    return (
      <Panel className="p-5">
        <h2 className="text-sm font-semibold">A local engine needs WSL</h2>
        <p className="mt-1 text-sm text-muted-foreground">
          Docker Engine runs inside a WSL distribution on Windows, and none was found on this computer. To set one up, open a terminal as administrator and run:
        </p>
        <div className="mt-3 flex items-center justify-between gap-2 rounded-md border bg-muted/40 py-1.5 pr-1.5 pl-3">
          <code className="font-mono text-sm">wsl --install</code>
          <Button variant="outline" size="sm" onClick={() => copyToClipboard("wsl --install", "Command")}>
            <Copy />
            Copy
          </Button>
        </div>
        <p className="mt-3 text-xs text-muted-foreground">
          It needs administrator rights and usually a restart. Docklet does not run it for you. After the restart, open Docklet and check again.
        </p>
        <Button variant="link" size="sm" className="mt-1 -ml-2.5" onClick={() => openUrl(WSL_DOCS)}>
          <ExternalLink />
          Microsoft guide to installing WSL
        </Button>
      </Panel>
    );
  }
  const mac = os === "darwin";
  return (
    <Panel className="p-5">
      <h2 className="text-sm font-semibold">No Docker Engine found</h2>
      <p className="mt-1 text-sm text-muted-foreground">
        No Docker socket was found on this computer. Install Docker {mac ? "Desktop (or Colima or OrbStack)" : "Engine"} and start it, then check again.
      </p>
      <Button variant="link" size="sm" className="mt-1 -ml-2.5" onClick={() => openUrl(mac ? DOCKER_DESKTOP_MAC : DOCKER_DOCS)}>
        <ExternalLink />
        Docker install guide
      </Button>
    </Panel>
  );
}
