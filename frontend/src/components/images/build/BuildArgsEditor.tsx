import { Plus, X } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { isSecretName } from "./buildFormat";
import { newArgRow, type ArgRow } from "./buildForm";

interface Props {
  args: ArgRow[];
  onChange: (args: ArgRow[]) => void;
}

/** Key/value rows. Values of secret-looking names are masked; no value is ever stored in the build history. */
export function BuildArgsEditor({ args, onChange }: Props) {
  const set = (id: number, patch: Partial<ArgRow>) => onChange(args.map((a) => (a.id === id ? { ...a, ...patch } : a)));
  const anySecret = args.some((a) => isSecretName(a.key.trim()));
  return (
    <div className="space-y-1.5">
      {args.map((a) => {
        const secret = isSecretName(a.key.trim());
        return (
          <div key={a.id} className="flex gap-2">
            <Input
              aria-label="Argument name"
              value={a.key}
              onChange={(e) => set(a.id, { key: e.target.value })}
              placeholder="NAME"
              className="w-2/5 font-mono"
              spellCheck={false}
              autoComplete="off"
            />
            <Input
              aria-label={secret ? "Argument value (masked)" : "Argument value"}
              type={secret ? "password" : "text"}
              value={a.value}
              onChange={(e) => set(a.id, { value: e.target.value })}
              placeholder="value"
              className="font-mono"
              spellCheck={false}
              autoComplete="off"
            />
            <Button type="button" variant="ghost" size="icon" className="size-8 shrink-0" aria-label="Remove argument" title="Remove argument" onClick={() => onChange(args.filter((x) => x.id !== a.id))}>
              <X className="size-4" />
            </Button>
          </div>
        );
      })}
      <Button type="button" variant="ghost" size="sm" onClick={() => onChange([...args, newArgRow()])}>
        <Plus className="size-3.5" /> Add argument
      </Button>
      {args.length > 0 && (
        <p className="text-xs text-muted-foreground">
          Values are never saved in the build history, only the names.
          {anySecret && " Values of names that look like credentials are masked and kept out of the command line and logs."}
        </p>
      )}
    </div>
  );
}
