import { Loader2, Wrench } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import { CUSTOM_IMAGE, TOOLBOX_IMAGES, isPresetImage } from "./debugShell";

/** Card shown over the terminal area for the debug-shell steps. */
function Card({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div className="absolute inset-0 z-10 flex items-start justify-center overflow-auto rounded-[10px] bg-card p-4">
      <div className="mt-4 w-full max-w-lg space-y-3 text-[13px]">
        <h3 className="flex items-center gap-2 text-sm font-medium text-foreground">
          <Wrench className="size-4 text-muted-foreground" />
          {title}
        </h3>
        {children}
      </div>
    </div>
  );
}

export function NoShellOffer({ onOpen, disabledReason }: { onOpen: () => void; disabledReason?: string }) {
  return (
    <Card title="This container has no shell">
      <p className="text-muted-foreground">
        The image does not ship a shell (typical for distroless and scratch images), so a regular terminal cannot start. You can open a
        debug shell instead: a temporary toolbox container that joins this container&apos;s process and network namespaces.
      </p>
      <Button size="sm" onClick={onOpen} disabled={!!disabledReason} title={disabledReason}>
        <Wrench className="size-4" /> Open a debug shell
      </Button>
    </Card>
  );
}

export interface ConfirmProps {
  image: string;
  onImage: (v: string) => void;
  sysPtrace: boolean;
  onSysPtrace: (v: boolean) => void;
  skip: boolean;
  onSkip: (v: boolean) => void;
  onOpen: () => void;
  onCancel: () => void;
}

export function DebugShellConfirm(p: ConfirmProps) {
  const custom = !isPresetImage(p.image);
  return (
    <Card title="Open a debug shell">
      <ul className="list-disc space-y-1 pl-5 text-muted-foreground">
        <li>
          A temporary container is created from <span className="font-mono text-foreground">{p.image || "…"}</span> on this Engine.
        </li>
        <li>It shares this container&apos;s process and network namespaces, so you can run ps and reach its ports on localhost.</li>
        <li>It is removed when you close the terminal, leave this page or the container stops.</li>
      </ul>
      <div className="flex flex-wrap items-center gap-2">
        <label className="flex items-center gap-1.5 text-muted-foreground">
          Image
          <select
            value={custom ? CUSTOM_IMAGE : p.image}
            onChange={(e) => p.onImage(e.target.value === CUSTOM_IMAGE ? "" : e.target.value)}
            className="h-8 rounded-md border border-border bg-card px-2 text-[13px] text-foreground"
            aria-label="Toolbox image"
          >
            {TOOLBOX_IMAGES.map((i) => (
              <option key={i.value} value={i.value}>
                {i.label}
              </option>
            ))}
            <option value={CUSTOM_IMAGE}>Custom image…</option>
          </select>
        </label>
        {custom && (
          <Input
            value={p.image}
            onChange={(e) => p.onImage(e.target.value)}
            placeholder="registry/name:tag"
            aria-label="Custom toolbox image"
            className="h-8 w-56 font-mono text-[13px] md:text-[13px]"
            autoFocus
          />
        )}
      </div>
      <label className="flex items-center gap-2">
        <Checkbox checked={p.sysPtrace} onCheckedChange={(v) => p.onSysPtrace(v === true)} />
        <span>
          Allow process inspection <span className="font-mono text-xs text-muted-foreground">(SYS_PTRACE)</span>
        </span>
      </label>
      <label className="flex items-center gap-2 text-muted-foreground">
        <Checkbox checked={p.skip} onCheckedChange={(v) => p.onSkip(v === true)} />
        Do not ask again
      </label>
      <div className="flex items-center gap-2">
        <Button size="sm" onClick={p.onOpen} disabled={!p.image.trim()}>
          Open debug shell
        </Button>
        <Button size="sm" variant="ghost" onClick={p.onCancel}>
          Cancel
        </Button>
      </div>
    </Card>
  );
}

export function ImageMissingOffer({ image, onPull, onCancel }: { image: string; onPull: () => void; onCancel: () => void }) {
  return (
    <Card title="The toolbox image is not on this Engine">
      <p className="text-muted-foreground">
        <span className="font-mono text-foreground">{image}</span> is not present on this Engine. Docklet never pulls without asking.
      </p>
      <div className="flex items-center gap-2">
        <Button size="sm" onClick={onPull}>
          Pull and open
        </Button>
        <Button size="sm" variant="ghost" onClick={onCancel}>
          Cancel
        </Button>
      </div>
    </Card>
  );
}

export function PullingNotice({ image }: { image: string }) {
  return (
    <Card title="Pulling the toolbox image">
      <p className="flex items-center gap-2 text-muted-foreground" role="status">
        <Loader2 className="size-4 animate-spin" />
        <span>
          Pulling <span className="font-mono text-foreground">{image}</span>… progress is also in the activity center.
        </span>
      </p>
    </Card>
  );
}
