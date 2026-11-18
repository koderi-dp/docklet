import type { ReactNode } from "react";
import type { PortCheck } from "@bindings/ports";
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
import { Button } from "@/components/ui/button";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { holderText } from "@/lib/portCheck";
import { HolderDetails } from "./HolderDetails";

export interface PortConflictDialogProps {
  title: string;
  intro: ReactNode;
  conflicts: PortCheck[];
  confirmLabel: string;
  onConfirm: () => void;
  onCancel: () => void;
  /** Offers "Pick another port" (create and recreate only). */
  onPick?: () => void;
  picking?: boolean;
}

/** Lists the host ports that something already holds and lets the user decide: go on anyway, pick another port, or cancel. */
export function PortConflictDialog({ title, intro, conflicts, confirmLabel, onConfirm, onCancel, onPick, picking }: PortConflictDialogProps) {
  return (
    <AlertDialog open onOpenChange={(o) => !o && !picking && onCancel()}>
      <AlertDialogContent className="sm:max-w-lg">
        <AlertDialogHeader>
          <AlertDialogTitle>{title}</AlertDialogTitle>
          <AlertDialogDescription asChild>
            <div className="space-y-3">
              <p>{intro}</p>
              <ul className="max-h-56 divide-y divide-border overflow-auto rounded-md border border-border text-[13px] text-foreground">
                {conflicts.map((c) => (
                  <li key={`${c.hostIp}:${c.hostPort}/${c.protocol}`} className="flex items-center gap-3 px-3 py-2">
                    <span className="w-16 shrink-0 font-mono font-medium tabular-nums">{c.hostPort}{c.protocol !== "tcp" && <span className="text-muted-foreground">/{c.protocol}</span>}</span>
                    <span className="min-w-0 flex-1 truncate" title={c.holder ? holderText(c.holder) : ""}>{c.holder ? holderText(c.holder) : c.detail}</span>
                    <Popover>
                      <PopoverTrigger asChild>
                        <Button type="button" variant="link" size="sm" className="h-auto shrink-0 p-0 text-xs">Details</Button>
                      </PopoverTrigger>
                      <PopoverContent align="end" className="w-80 text-[13px]">
                        <HolderDetails check={c} />
                      </PopoverContent>
                    </Popover>
                  </li>
                ))}
              </ul>
            </div>
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel disabled={picking}>Cancel</AlertDialogCancel>
          {onPick && (
            <Button variant="outline" onClick={onPick} disabled={picking}>
              {picking ? "Looking for a free port…" : conflicts.length === 1 ? "Pick another port" : "Pick other ports"}
            </Button>
          )}
          <AlertDialogAction disabled={picking} onClick={onConfirm}>{confirmLabel}</AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}
