import { useRef, useState } from "react";
import { Bookmark, Check, Download, Trash2, Upload } from "lucide-react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { cn } from "@/lib/utils";
import { downloadText } from "@/lib/downloadText";
import { MAX_FILE_BYTES, parseSavedFilters, serializeSavedFilters } from "@/lib/logFilter/savedFiltersFile";
import type { LogFilterState } from "./useLogFilter";

/** Saved rule sets: save the open rules under a name, switch between sets, delete. */
export function SavedFilters({ f }: { f: LogFilterState }) {
  const [name, setName] = useState("");
  const [confirmId, setConfirmId] = useState<string | null>(null);
  const [importError, setImportError] = useState<string | null>(null);
  const fileInput = useRef<HTMLInputElement>(null);
  const canSave = f.rules.length > 0 && name.trim() !== "";
  const save = () => {
    if (!canSave) return;
    f.saveAs(name);
    setName("");
  };
  const exportAll = () => {
    downloadText("docklet-log-filters.json", serializeSavedFilters(f.saved));
    toast.success(`Exported ${f.saved.length} saved filter${f.saved.length === 1 ? "" : "s"}`);
  };
  const importFile = async (file: File) => {
    setImportError(null);
    if (file.size > MAX_FILE_BYTES) return setImportError("This file is too large to be a saved-filters file (over 1 MB).");
    const parsed = parseSavedFilters(await file.text().catch(() => ""));
    if (!parsed.ok) return setImportError(parsed.error);
    const r = f.importSaved(parsed.filters);
    toast.success(`Imported ${r.added} saved filter${r.added === 1 ? "" : "s"}`, { description: r.renamed > 0 ? `${r.renamed} had a name in use and got “(imported)” added.` : undefined });
  };
  return (
    <Popover
      onOpenChange={() => {
        setConfirmId(null);
        setImportError(null);
      }}
    >
      <PopoverTrigger asChild>
        <Button variant="outline" size="sm" className="max-w-[200px] shrink-0 gap-1.5" title="Saved filters">
          <Bookmark className="size-3.5" />
          <span className="truncate">{f.activeSaved ? f.activeSaved.name : "Saved"}</span>
          {f.modified && <span aria-label="modified" title="Changed since it was saved" className="size-1.5 shrink-0 rounded-full bg-warning" />}
        </Button>
      </PopoverTrigger>
      <PopoverContent align="end" className="w-72 space-y-2 p-2">
        <div className="px-1 text-xs font-medium text-muted-foreground">Saved filters</div>
        {f.saved.length === 0 ? (
          <p className="px-1 py-1 text-xs text-muted-foreground">Nothing saved yet. Build some rules, name them below and they will be here next time.</p>
        ) : (
          <ul className="max-h-56 space-y-0.5 overflow-auto">
            {f.saved.map((s) =>
              confirmId === s.id ? (
                <li key={s.id} className="flex items-center gap-1.5 rounded-md bg-destructive/10 px-2 py-1 text-xs">
                  <span className="min-w-0 flex-1 truncate">Delete “{s.name}”?</span>
                  <Button size="xs" variant="destructive" onClick={() => { f.deleteSaved(s.id); setConfirmId(null); }}>
                    Delete
                  </Button>
                  <Button size="xs" variant="ghost" onClick={() => setConfirmId(null)}>
                    Cancel
                  </Button>
                </li>
              ) : (
                <li key={s.id} className={cn("flex items-center rounded-md hover:bg-muted", f.activeSaved?.id === s.id && "bg-muted")}>
                  <button type="button" onClick={() => f.applySaved(s.id)} className="flex min-w-0 flex-1 items-center gap-2 px-2 py-1.5 text-left text-[13px]">
                    <Check className={cn("size-3.5 shrink-0", f.activeSaved?.id === s.id ? "text-primary" : "opacity-0")} />
                    <span className="min-w-0 flex-1 truncate">{s.name}</span>
                    <span className="shrink-0 text-xs text-muted-foreground">{s.rules.length} {s.rules.length === 1 ? "rule" : "rules"}</span>
                  </button>
                  <Button variant="ghost" size="icon-xs" className="mr-1 shrink-0" aria-label={`Delete saved filter ${s.name}`} title="Delete" onClick={() => setConfirmId(s.id)}>
                    <Trash2 />
                  </Button>
                </li>
              ),
            )}
          </ul>
        )}
        {f.activeSaved && f.modified && (
          <Button size="sm" variant="secondary" className="w-full" onClick={f.updateSaved}>
            Update “{f.activeSaved.name}”
          </Button>
        )}
        <form
          className="flex gap-1.5 border-t border-border pt-2"
          onSubmit={(e) => {
            e.preventDefault();
            save();
          }}
        >
          <Input value={name} onChange={(e) => setName(e.target.value)} placeholder="Save current rules as…" aria-label="Name for the saved filter" className="h-8 text-[13px]" />
          <Button type="submit" size="sm" disabled={!canSave} title={f.rules.length === 0 ? "Add at least one rule chip first (press Enter in the filter box)" : undefined}>
            Save
          </Button>
        </form>
        {importError && (
          <p role="alert" className="rounded-md bg-destructive/10 px-2 py-1.5 text-xs text-destructive">
            Could not import: {importError}
          </p>
        )}
        <div className="flex items-center gap-1.5 border-t border-border pt-2">
          <Button size="xs" variant="ghost" disabled={f.saved.length === 0} title={f.saved.length === 0 ? "Nothing saved to export yet" : "Save all saved filters to a JSON file"} onClick={exportAll}>
            <Download /> Export…
          </Button>
          <Button size="xs" variant="ghost" title="Add saved filters from a JSON file made by Export" onClick={() => fileInput.current?.click()}>
            <Upload /> Import…
          </Button>
          <input
            ref={fileInput}
            type="file"
            accept=".json,application/json"
            className="hidden"
            aria-label="Saved filters file"
            onChange={(e) => {
              const file = e.target.files?.[0];
              e.target.value = "";
              if (file) void importFile(file);
            }}
          />
        </div>
      </PopoverContent>
    </Popover>
  );
}
