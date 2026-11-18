import { useState } from "react";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { SettingRow } from "./SettingRow";
import { templateProblem } from "./ExternalTerminalRow";
import { useUI, type EditorSetting } from "@/stores/uiStore";

const EXAMPLE = "zed {path}";

/** Settings row: which editor "Open in editor" starts (auto-detect, VS Code, Cursor or a custom template with {path}). */
export function EditorRow() {
  const setting = useUI((s) => s.editor);
  const set = useUI((s) => s.setEditor);
  const [draft, setDraft] = useState(setting.template);
  const problem = setting.mode === "custom" ? templateProblem(draft, "{path}", EXAMPLE) : null;
  return (
    <SettingRow
      label="Editor"
      htmlFor="editor-mode"
      hint={
        <>
          Used by Open in editor for Compose projects and bind mounts on this computer. Auto picks the first of VS Code, Cursor, Zed or Sublime Text it finds.
          {problem && <span className="mt-1 block text-destructive">{problem}</span>}
        </>
      }
    >
      <div className="flex flex-wrap items-center justify-end gap-2">
        <Select value={setting.mode} onValueChange={(v) => set({ mode: v as EditorSetting["mode"], template: draft })}>
          <SelectTrigger id="editor-mode" className="w-44" size="sm">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="auto">Auto</SelectItem>
            <SelectItem value="vscode">VS Code</SelectItem>
            <SelectItem value="cursor">Cursor</SelectItem>
            <SelectItem value="custom">Custom command</SelectItem>
          </SelectContent>
        </Select>
        {setting.mode === "custom" && (
          <Input
            aria-label="Custom editor command"
            aria-invalid={!!problem}
            placeholder={EXAMPLE}
            value={draft}
            className="h-8 w-52 font-mono text-xs"
            onChange={(e) => {
              setDraft(e.target.value);
              if (!templateProblem(e.target.value, "{path}", EXAMPLE)) set({ mode: "custom", template: e.target.value });
            }}
          />
        )}
      </div>
    </SettingRow>
  );
}
