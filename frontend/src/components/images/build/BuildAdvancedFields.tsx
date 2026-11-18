import { Info } from "lucide-react";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { BuildArgsEditor } from "./BuildArgsEditor";
import type { BuildForm } from "./buildForm";

const LABEL = "text-xs font-medium text-muted-foreground";
const DEFAULT_PLATFORM = "__default";
const PLATFORMS = ["linux/amd64", "linux/arm64"];

interface Props {
  form: BuildForm;
  onChange: (patch: Partial<BuildForm>) => void;
}

/** Everything under "Advanced": build arguments, target stage, platform and the cache switches. */
export function BuildAdvancedFields({ form, onChange }: Props) {
  // A platform from an older build that is not in the list stays selectable.
  const platforms = form.platform && !PLATFORMS.includes(form.platform) ? [...PLATFORMS, form.platform] : PLATFORMS;
  return (
    <div className="space-y-3 pt-2">
      <div className="space-y-1">
        <span className={LABEL}>Build arguments</span>
        <BuildArgsEditor args={form.args} onChange={(args) => onChange({ args })} />
      </div>

      <div className="grid grid-cols-2 gap-3">
        <div className="space-y-1">
          <label htmlFor="build-target" className={LABEL}>
            Target stage
          </label>
          <Input id="build-target" value={form.target} onChange={(e) => onChange({ target: e.target.value })} placeholder="Last stage" className="font-mono" spellCheck={false} autoComplete="off" />
        </div>
        <div className="space-y-1">
          <span className="flex items-center gap-1">
            <label htmlFor="build-platform" className={LABEL}>
              Platform
            </label>
            <Tooltip>
              <TooltipTrigger asChild>
                <button type="button" aria-label="About platforms" className="text-muted-foreground hover:text-foreground">
                  <Info className="size-3.5" />
                </button>
              </TooltipTrigger>
              <TooltipContent className="max-w-64">
                One platform per build. A multi-platform build produces a manifest list, which cannot be loaded into the Engine&apos;s image store (--load), so it is not offered here.
              </TooltipContent>
            </Tooltip>
          </span>
          <Select value={form.platform || DEFAULT_PLATFORM} onValueChange={(v) => onChange({ platform: v === DEFAULT_PLATFORM ? "" : v })}>
            <SelectTrigger id="build-platform" className="w-full font-mono">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value={DEFAULT_PLATFORM}>Engine default</SelectItem>
              {platforms.map((p) => (
                <SelectItem key={p} value={p} className="font-mono">
                  {p}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
      </div>

      <div className="flex flex-wrap gap-x-6 gap-y-2 text-[13px]">
        <label className="flex items-center gap-2">
          <input type="checkbox" checked={form.noCache} onChange={(e) => onChange({ noCache: e.target.checked })} className="size-3.5 accent-primary" />
          Do not use cache
        </label>
        <label className="flex items-center gap-2">
          <input type="checkbox" checked={form.pull} onChange={(e) => onChange({ pull: e.target.checked })} className="size-3.5 accent-primary" />
          Always pull newer base images
        </label>
      </div>
    </div>
  );
}
