import { cn } from "@/lib/utils";

const DOT = { success: "bg-success", warning: "bg-warning", muted: "bg-muted-foreground/60" } as const;

export function StatusDot({ tone, text }: { tone: keyof typeof DOT; text: string }) {
  return (
    <span className="inline-flex items-center gap-2">
      <span aria-hidden className={cn("size-2 shrink-0 rounded-full", DOT[tone])} />
      {text}
    </span>
  );
}
