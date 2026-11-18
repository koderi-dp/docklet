import { useState } from "react";
import { Play, SlidersHorizontal } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { DEFAULT_UP, type UpOptions } from "@/hooks/useCompose";

const PROFILE_RE = /^[A-Za-z0-9][A-Za-z0-9_.-]*$/;

/** Parses "a, b c" into profile names; null when any token is not a valid profile name. */
export function parseProfiles(text: string): string[] | null {
  const parts = text.split(/[\s,]+/).filter(Boolean);
  return parts.every((p) => PROFILE_RE.test(p)) ? parts : null;
}

interface Props {
  project: string;
  disabledReason?: string;
  labels?: boolean;
  onUp: (opts: UpOptions) => void;
}

/** Compose Up with explicit options: build, remove orphans, profiles. */
export function UpOptionsPopover({ project, disabledReason, labels, onUp }: Props) {
  const [open, setOpen] = useState(false);
  const [build, setBuild] = useState(DEFAULT_UP.build);
  const [orphans, setOrphans] = useState(DEFAULT_UP.removeOrphans);
  const [profiles, setProfiles] = useState("");
  const parsed = parseProfiles(profiles);
  return (
    <Popover open={open} onOpenChange={setOpen}>
      <Tooltip>
        <TooltipTrigger asChild>
          <span className="inline-flex">
            <PopoverTrigger asChild>
              <Button
                variant="outline"
                size={labels ? "sm" : "icon"}
                className={labels ? "" : "size-7"}
                disabled={!!disabledReason}
                aria-label={`Compose Up options for ${project}`}
              >
                <SlidersHorizontal className="size-3.5" />
                {labels && "Up options"}
              </Button>
            </PopoverTrigger>
          </span>
        </TooltipTrigger>
        <TooltipContent className="max-w-sm">{disabledReason ?? "Up with options: build, remove orphans, profiles"}</TooltipContent>
      </Tooltip>
      <PopoverContent align="end" className="w-80 space-y-3 text-sm">
        <div className="font-medium">Compose Up — {project}</div>
        <label className="flex items-start gap-2">
          <Checkbox checked={build} onCheckedChange={(v) => setBuild(v === true)} className="mt-0.5" />
          <span>
            Build images first <span className="block text-xs text-muted-foreground">--build</span>
          </span>
        </label>
        <label className="flex items-start gap-2">
          <Checkbox checked={orphans} onCheckedChange={(v) => setOrphans(v === true)} className="mt-0.5" />
          <span>
            Remove orphan containers{" "}
            <span className="block text-xs text-muted-foreground">--remove-orphans: removes containers of services no longer in the files</span>
          </span>
        </label>
        <div className="space-y-1">
          <Label htmlFor={`profiles-${project}`}>Profiles</Label>
          <Input
            id={`profiles-${project}`}
            value={profiles}
            onChange={(e) => setProfiles(e.target.value)}
            placeholder="debug, worker"
            spellCheck={false}
            aria-invalid={parsed === null}
            className="h-8"
          />
          {parsed === null && <p className="text-xs text-destructive">Profile names may contain letters, digits, _ . and -.</p>}
        </div>
        <Button
          size="sm"
          className="w-full"
          disabled={parsed === null}
          onClick={() => {
            setOpen(false);
            onUp({ build, removeOrphans: orphans, profiles: parsed ?? [] });
          }}
        >
          <Play className="size-3.5" /> Up
        </Button>
      </PopoverContent>
    </Popover>
  );
}
