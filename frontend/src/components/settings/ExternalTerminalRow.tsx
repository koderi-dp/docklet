import { useState } from "react";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { useUI } from "@/stores/uiStore";

/** Template must contain {cmd}; mirrors the backend check (ValidateTerminalTemplate). */
export function templateProblem(template: string): string | null {
  if (!template.includes("{cmd}")) return "The command must contain {cmd}, e.g. alacritty -e {cmd}";
  return null;
}

/** Settings row: which terminal "Open in external terminal" launches (auto-detect or a custom template). */
export function ExternalTerminalRow() {
  const setting = useUI((s) => s.externalTerminal);
  const set = useUI((s) => s.setExternalTerminal);
  const [draft, setDraft] = useState(setting.template);
  const problem = setting.mode === "custom" ? templateProblem(draft) : null;
  return (
    <div className="flex flex-wrap items-start justify-between gap-3 px-4 py-3">
      <div className="min-w-0 max-w-md">
        <Label htmlFor="ext-term" className="text-sm font-medium">
          External terminal
        </Label>
        <p className="text-xs text-muted-foreground">
          Used by Open in external terminal. Auto picks Windows Terminal or cmd, Terminal.app, or the first Linux terminal found. Requires the docker CLI.
        </p>
      </div>
      <div className="flex w-72 shrink-0 flex-col gap-2">
        <Select value={setting.mode} onValueChange={(v) => set({ mode: v as "auto" | "custom", template: draft })}>
          <SelectTrigger id="ext-term" className="w-full" size="sm">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="auto">Auto</SelectItem>
            <SelectItem value="custom">Custom command</SelectItem>
          </SelectContent>
        </Select>
        {setting.mode === "custom" && (
          <>
            <Input
              aria-label="Custom terminal command"
              aria-invalid={!!problem}
              placeholder="alacritty -e {cmd}"
              value={draft}
              className="h-8 font-mono text-xs"
              onChange={(e) => {
                setDraft(e.target.value);
                if (!templateProblem(e.target.value)) set({ mode: "custom", template: e.target.value });
              }}
            />
            {problem && <p className="text-xs text-destructive">{problem}</p>}
          </>
        )}
      </div>
    </div>
  );
}
