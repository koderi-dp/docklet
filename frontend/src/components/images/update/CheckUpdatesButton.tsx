import { useState } from "react";
import { BellRing } from "lucide-react";
import type { ImageRow } from "@bindings/model";
import { Button } from "@/components/ui/button";
import { PODMAN_NO_UPDATE_CHECK, useIsPodman } from "@/lib/engineKind";
import { checkAllImageUpdates } from "./updateStore";

/** Toolbar action: asks the registries about every tagged image, on demand only. */
export function CheckUpdatesButton({ connectionId, rows }: { connectionId: string; rows: ImageRow[] }) {
  const [busy, setBusy] = useState(false);
  const tagged = rows.filter((r) => !r.dangling).length;
  const podman = useIsPodman(connectionId);
  return (
    <Button
      size="sm"
      variant="outline"
      disabled={busy || tagged === 0 || podman}
      title={podman ? PODMAN_NO_UPDATE_CHECK : tagged === 0 ? "No tagged images to check" : "Ask the registries whether newer images exist for the tags on this Engine"}
      onClick={() => {
        setBusy(true);
        void checkAllImageUpdates(connectionId, rows).finally(() => setBusy(false));
      }}
    >
      <BellRing className="size-3.5" />
      <span className="hidden @[900px]:inline">Check for updates</span>
    </Button>
  );
}
