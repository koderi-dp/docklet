import { useState } from "react";
import { Power } from "lucide-react";
import type { WSLAction, WSLInfo } from "@bindings/model";
import { Button } from "@/components/ui/button";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { Card } from "@/components/system/parts";
import { cn } from "@/lib/utils";
import { ACTIONS, SAFE_ACTIONS, type WslActionId } from "./wslActions";

function ActionButton({ action, label, busy, destructive, onClick }: { action?: WSLAction; label: string; busy: boolean; destructive?: boolean; onClick: () => void }) {
  const reason = action?.disabled || (action ? "" : "Not available");
  const btn = (
    <Button
      size="sm"
      variant="outline"
      disabled={busy || !!reason}
      onClick={onClick}
      className={cn("w-40 shrink-0", destructive && "border-destructive/50 text-destructive hover:bg-destructive/10 hover:text-destructive")}
    >
      {label}…
    </Button>
  );
  if (!reason) return btn;
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <span tabIndex={0}>{btn}</span>
      </TooltipTrigger>
      <TooltipContent>{reason}</TooltipContent>
    </Tooltip>
  );
}

export function WslActionsCard({ info, busy, onRun }: { info: WSLInfo; busy: boolean; onRun: (id: WslActionId) => void }) {
  const [pending, setPending] = useState<WslActionId | null>(null);
  const find = (id: string) => (info.actions ?? []).find((a) => a.id === id);
  const text = pending ? ACTIONS[pending] : null;
  const command = pending ? find(pending)?.command : undefined;
  const shutdown = ACTIONS.shutdown;
  return (
    <Card title="Maintenance" subtitle="Run as root through wsl.exe. No password is needed and no configuration file is changed.">
      <ul className="divide-y divide-border/50">
        {SAFE_ACTIONS.map((id) => {
          const t = ACTIONS[id];
          return (
            <li key={id} className="flex items-center justify-between gap-4 py-2.5 first:pt-0">
              <div className="min-w-0 text-[13px]">
                <div className="font-medium">{t.title}</div>
                <div className="text-muted-foreground">{t.summary}</div>
              </div>
              <ActionButton action={find(id)} label={t.title} busy={busy} onClick={() => setPending(id)} />
            </li>
          );
        })}
      </ul>
      <div className="mt-3 flex items-center justify-between gap-4 rounded-lg border border-destructive/30 bg-destructive/5 px-3 py-2.5">
        <div className="min-w-0 text-[13px]">
          <div className="flex items-center gap-1.5 font-medium text-destructive">
            <Power className="size-3.5" />
            {shutdown.title}
          </div>
          <div className="text-muted-foreground">{shutdown.summary} Affects every distribution and Docker Desktop.</div>
        </div>
        <ActionButton action={find("shutdown")} label={shutdown.title} busy={busy} destructive onClick={() => setPending("shutdown")} />
      </div>

      <AlertDialog open={!!pending} onOpenChange={(o) => !o && setPending(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{text?.title}?</AlertDialogTitle>
            <AlertDialogDescription asChild>
              <div className="space-y-2">
                {text?.effects(info).map((line) => <p key={line}>{line}</p>)}
                {command && (
                  <p className="pt-1">
                    <span className="text-xs text-muted-foreground">Runs:</span>
                    <code className="mt-1 block break-all rounded-md bg-muted px-2 py-1.5 font-mono text-xs text-foreground">{command}</code>
                  </p>
                )}
              </div>
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction
              className={pending === "shutdown" ? "bg-destructive! text-white! hover:bg-destructive/90!" : undefined}
              onClick={() => {
                if (pending) onRun(pending);
                setPending(null);
              }}
            >
              {text?.confirm}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </Card>
  );
}
