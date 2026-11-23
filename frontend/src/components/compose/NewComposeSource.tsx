import type { Container } from "@bindings/model";
import { Checkbox } from "@/components/ui/checkbox";
import { Label } from "@/components/ui/label";
import { orderContainers, type NewSourceKind } from "@/lib/newComposeFile";
import { cn } from "@/lib/utils";

/** A ready-made starting file the user can pick. */
export interface TemplateChoice {
  id: string;
  title: string;
}

export interface NewComposeSourceProps {
  source: NewSourceKind;
  onSource: (s: NewSourceKind) => void;
  containers: readonly Container[] | undefined;
  ticked: ReadonlySet<string>;
  onToggle: (id: string, on: boolean) => void;
  includeSecrets: boolean;
  onIncludeSecrets: (on: boolean) => void;
  templates: readonly TemplateChoice[];
  template: string;
  onTemplate: (id: string) => void;
}

function Choice({ value, current, onChange, children }: { value: NewSourceKind; current: NewSourceKind; onChange: (s: NewSourceKind) => void; children: string }) {
  return (
    <label className="flex cursor-pointer items-center gap-2 text-sm">
      <input type="radio" name="compose-new-source" className="size-3.5 accent-primary" checked={current === value} onChange={() => onChange(value)} />
      {children}
    </label>
  );
}

/** "Start from": a minimal example, a template, or the containers that already run. */
export function NewComposeSource(p: NewComposeSourceProps) {
  const list = orderContainers(p.containers ?? []);
  return (
    <div className="space-y-1.5">
      <Label>Start from</Label>
      <div role="radiogroup" aria-label="Start from" className="space-y-1.5">
        <Choice value="minimal" current={p.source} onChange={p.onSource}>
          Minimal example
        </Choice>
        {p.templates.length > 0 && (
          <div className="flex flex-wrap items-center gap-2">
            <Choice value="template" current={p.source} onChange={p.onSource}>
              Template
            </Choice>
            <select
              aria-label="Template"
              value={p.template}
              disabled={p.source !== "template"}
              onChange={(e) => p.onTemplate(e.target.value)}
              className="h-8 rounded-md border border-input bg-background px-2 text-xs text-foreground disabled:opacity-50"
            >
              {p.templates.map((t) => (
                <option key={t.id} value={t.id}>
                  {t.title}
                </option>
              ))}
            </select>
          </div>
        )}
        <Choice value="containers" current={p.source} onChange={p.onSource}>
          Running containers
        </Choice>
      </div>
      {p.source === "containers" && (
        <div className="ml-5 space-y-2">
          {list.length === 0 ? (
            <p className="text-xs text-muted-foreground">{p.containers ? "There are no containers on this Engine." : "Loading containers…"}</p>
          ) : (
            <ul className="max-h-36 overflow-auto rounded-md border border-border" aria-label="Containers">
              {list.map((c) => (
                <li key={c.id} className="border-b border-border/60 last:border-b-0">
                  <label className="flex cursor-pointer items-center gap-2 px-2 py-1.5 text-xs">
                    <Checkbox checked={p.ticked.has(c.id)} onCheckedChange={(on) => p.onToggle(c.id, on === true)} aria-label={`Include ${c.name}`} />
                    <span className={cn("w-40 shrink-0 truncate font-medium", c.state !== "running" && "text-muted-foreground")} title={c.name}>
                      {c.name}
                    </span>
                    <span className="min-w-0 flex-1 truncate font-mono text-muted-foreground" title={c.image}>
                      {c.image}
                    </span>
                    {c.composeProject && (
                      <span className="shrink-0 text-muted-foreground" title={`Compose project ${c.composeProject}`}>
                        {c.composeProject}
                      </span>
                    )}
                    {c.state !== "running" && <span className="shrink-0 text-muted-foreground">{c.state}</span>}
                  </label>
                </li>
              ))}
            </ul>
          )}
          <label className="flex items-center gap-2 text-xs">
            <Checkbox checked={p.includeSecrets} onCheckedChange={(on) => p.onIncludeSecrets(on === true)} />
            <span>Include secret values</span>
          </label>
          <p className="text-xs text-muted-foreground">The original containers keep running. Up will clash with their names and ports until they are stopped or removed.</p>
        </div>
      )}
    </div>
  );
}
