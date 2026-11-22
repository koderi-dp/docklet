import type { ComposeOutlineItem } from "@bindings/model";

interface Props {
  services: readonly ComposeOutlineItem[];
  onJump: (line: number) => void;
}

export function ServiceOutline({ services, onJump }: Props) {
  return (
    <nav aria-label="Services in this file" className="flex h-full min-h-0 flex-col rounded-[10px] border border-border bg-card">
      <h3 className="border-b border-border px-3 py-2 text-[10px] font-semibold uppercase tracking-wide text-muted-foreground">Services</h3>
      {services.length === 0 ? (
        <p className="px-3 py-2 text-xs text-muted-foreground">No services found.</p>
      ) : (
        <ul className="min-h-0 flex-1 overflow-auto py-1">
          {services.map((s) => (
            <li key={`${s.name}:${s.line}`}>
              <button
                type="button"
                onClick={() => onJump(s.line)}
                className="flex w-full items-baseline justify-between gap-2 px-3 py-1 text-left text-xs hover:bg-muted"
              >
                <span className="min-w-0 truncate">{s.name}</span>
                <span className="font-mono text-[10px] text-muted-foreground">{s.line}</span>
              </button>
            </li>
          ))}
        </ul>
      )}
    </nav>
  );
}
