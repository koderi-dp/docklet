import { useState } from "react";
import { Eye } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import type { ComposeWatchService } from "@bindings/model";
import { serviceArgs } from "./watchFormat";

interface Props {
  project: string;
  services: readonly ComposeWatchService[];
  /** Pre-selection when starting again. */
  initial?: { services: readonly string[]; noUp: boolean };
  onStart: (opts: { services: string[]; noUp: boolean }) => void;
}

/** Confirm panel of Compose watch: which services to watch, --no-up and what "running" means. */
export function WatchStartPanel({ project, services, initial, onStart }: Props) {
  const names = services.map((s) => s.name);
  const [ticked, setTicked] = useState<Set<string>>(new Set(initial?.services?.length ? initial.services : names));
  const [noUp, setNoUp] = useState(initial?.noUp ?? false);
  const toggle = (n: string, on: boolean) =>
    setTicked((cur) => {
      const next = new Set(cur);
      if (on) next.add(n);
      else next.delete(n);
      return next;
    });
  return (
    <div className="space-y-3 text-sm">
      <div className="font-medium">Watch {project}</div>
      <div className="space-y-1.5">
        <div className="text-xs text-muted-foreground">Services with watch rules</div>
        {services.map((s) => (
          <label key={s.name} className="flex items-center gap-2">
            <Checkbox checked={ticked.has(s.name)} onCheckedChange={(v) => toggle(s.name, v === true)} />
            <span className="min-w-0 flex-1 truncate">{s.name}</span>
            <span className="text-xs text-muted-foreground">
              {(s.rules ?? []).length} rule{(s.rules ?? []).length === 1 ? "" : "s"}
            </span>
          </label>
        ))}
      </div>
      <label className="flex items-start gap-2">
        <Checkbox checked={noUp} onCheckedChange={(v) => setNoUp(v === true)} className="mt-0.5" />
        <span>
          Do not start services first <span className="block text-xs text-muted-foreground">--no-up: watch only, containers are not created or built</span>
        </span>
      </label>
      <p className="text-xs text-muted-foreground">Watch keeps running until you stop it or close Docklet.</p>
      <Button size="sm" className="w-full" disabled={ticked.size === 0} onClick={() => onStart({ services: serviceArgs(names, ticked), noUp })}>
        <Eye className="size-3.5" /> Start watch
      </Button>
    </div>
  );
}
