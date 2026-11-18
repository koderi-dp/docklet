import { useState } from "react";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { SettingRow } from "./SettingRow";
import { useUI } from "@/stores/uiStore";

/** Template must contain the placeholder and start with a program; mirrors the backend check (cmdtemplate.Validate). */
export function templateProblem(template: string, token = "{cmd}", example = "alacritty -e {cmd}"): string | null {
  if (!template.includes(token)) return `The command must contain ${token}, e.g. ${example}`;
  if (template.trimStart().startsWith(token)) return `${token} cannot be the program itself, e.g. ${example}`;
  return null;
}

/** Settings row: which terminal "Open in external terminal" launches (auto-detect or a custom template). */
export function ExternalTerminalRow() {
  const setting = useUI((s) => s.externalTerminal);
  const set = useUI((s) => s.setExternalTerminal);
  const [draft, setDraft] = useState(setting.template);
  const problem = setting.mode === "custom" ? templateProblem(draft) : null;
  return (
    <SettingRow
      label="External terminal"
      htmlFor="ext-term"
      hint={
        <>
          Used by Open in external terminal. Auto picks Windows Terminal or cmd, Terminal.app, or the first Linux terminal found. Requires the docker CLI.
          {problem && <span className="mt-1 block text-destructive">{problem}</span>}
        </>
      }
    >
      <div className="flex flex-wrap items-center justify-end gap-2">
        <Select value={setting.mode} onValueChange={(v) => set({ mode: v as "auto" | "custom", template: draft })}>
          <SelectTrigger id="ext-term" className="w-44" size="sm">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="auto">Auto</SelectItem>
            <SelectItem value="custom">Custom command</SelectItem>
          </SelectContent>
        </Select>
        {setting.mode === "custom" && (
          <Input
            aria-label="Custom terminal command"
            aria-invalid={!!problem}
            placeholder="alacritty -e {cmd}"
            value={draft}
            className="h-8 w-52 font-mono text-xs"
            onChange={(e) => {
              setDraft(e.target.value);
              if (!templateProblem(e.target.value)) set({ mode: "custom", template: e.target.value });
            }}
          />
        )}
      </div>
    </SettingRow>
  );
}
