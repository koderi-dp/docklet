import type { ReactNode } from "react";

interface Props {
  title: string;
  lede?: ReactNode;
  children: ReactNode;
  /** Left of the action row: a secondary or back action. */
  secondary?: ReactNode;
  /** Right of the action row: the one primary action, optionally after quieter ones. */
  actions?: ReactNode;
}

/** The shape every step shares: centred content that scrolls, and an action row that never leaves the window. */
export function StepFrame({ title, lede, children, secondary, actions }: Props) {
  return (
    <div className="flex h-full min-h-0 flex-col">
      <div className="flex min-h-0 flex-1 overflow-y-auto">
        <div className="m-auto w-full max-w-[600px] px-8 py-8">
          <h1 className="text-2xl font-semibold tracking-tight">{title}</h1>
          {lede && <p className="mt-1.5 text-sm text-muted-foreground">{lede}</p>}
          <div className="mt-6">{children}</div>
        </div>
      </div>
      <div className="shrink-0 border-t bg-card/40">
        <div className="mx-auto flex w-full max-w-[600px] items-center justify-between gap-3 px-8 py-3">
          <div className="-ml-3 flex items-center gap-2">{secondary}</div>
          <div className="flex items-center gap-2">{actions}</div>
        </div>
      </div>
    </div>
  );
}

/** A bordered block used inside steps (same surface as the Settings sections). */
export function Panel({ children, className }: { children: ReactNode; className?: string }) {
  return <div className={`rounded-lg border bg-card ${className ?? ""}`}>{children}</div>;
}
