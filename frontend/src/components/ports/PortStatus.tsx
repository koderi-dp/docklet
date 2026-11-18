import { AlertTriangle, CheckCircle2, HelpCircle, Loader2 } from "lucide-react";
import { Status, type PortCheck } from "@bindings/ports";
import { Button } from "@/components/ui/button";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { checkText } from "@/lib/portCheck";
import type { RowCheck } from "@/hooks/usePortChecks";
import { HolderDetails } from "./HolderDetails";

/** The inline answer under a port mapping row: free (green), used by something (warning) or could not check (muted). */
export function PortStatus({ row }: { row: RowCheck | undefined }) {
  if (!row) return null;
  if (row.state === "checking") {
    return (
      <p className="mb-1.5 flex items-center gap-1.5 text-xs text-muted-foreground" role="status">
        <Loader2 className="size-3.5 animate-spin" /> Checking the port…
      </p>
    );
  }
  if (row.state === "failed") return <Line icon={<HelpCircle />} tone="muted" text={`Could not check: ${row.reason}`} />;
  const c = row.check;
  if (c.status === Status.StatusFree) return <Line icon={<CheckCircle2 />} tone="success" text={checkText(c)} note={c.note} />;
  if (c.status === Status.StatusInUse) return <Line icon={<AlertTriangle />} tone="warning" text={checkText(c)} note={c.note} details={c} />;
  return <Line icon={<HelpCircle />} tone="muted" text={checkText(c)} note={c.note} />;
}

const TONE = { success: "text-success", warning: "text-warning", muted: "text-muted-foreground" } as const;

function Line({ icon, tone, text, note, details }: { icon: React.ReactNode; tone: keyof typeof TONE; text: string; note?: string; details?: PortCheck }) {
  return (
    <div className="mb-1.5 text-xs" role="status">
      <p className={`flex min-w-0 items-center gap-1.5 ${TONE[tone]}`}>
        <span className="shrink-0 [&>svg]:size-3.5">{icon}</span>
        <span className="min-w-0 truncate" title={text}>{text}</span>
        {details && (
          <Popover>
            <PopoverTrigger asChild>
              <Button type="button" variant="link" size="sm" className="h-auto shrink-0 p-0 text-xs">Details</Button>
            </PopoverTrigger>
            <PopoverContent align="start" className="w-80 text-[13px]">
              <HolderDetails check={details} />
            </PopoverContent>
          </Popover>
        )}
      </p>
      {note && <p className="mt-0.5 pl-5 text-muted-foreground">{note}</p>}
    </div>
  );
}
