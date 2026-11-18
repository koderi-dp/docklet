import { useEffect, useState } from "react";
import { FolderOpen, Info, Loader2, Plus, X } from "lucide-react";
import { toast } from "sonner";
import { DialogService } from "@bindings/services";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { useBuildImage } from "@/hooks/useImages";
import { useOperations } from "@/hooks/useOperations";
import { quickRefProblem } from "./imageFilters";
import { errorMessage } from "@/lib/errors";

interface Props {
  connectionId: string;
  engineName: string;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

interface Arg {
  id: number;
  key: string;
  value: string;
}

const PLATFORMS = ["linux/amd64", "linux/arm64", "linux/arm/v7", "linux/386"];
let nextId = 1;

/** Path of file relative to dir (slash-separated), or null when it is outside dir. */
export function relativeInside(dir: string, file: string): string | null {
  const norm = (p: string) => p.replace(/\\/g, "/").replace(/\/+$/, "");
  const d = norm(dir);
  const f = norm(file);
  const windowsLike = /^[a-zA-Z]:/.test(d);
  const dd = windowsLike ? d.toLowerCase() : d;
  const ff = windowsLike ? f.toLowerCase() : f;
  if (!ff.startsWith(dd + "/")) return null;
  return f.slice(d.length + 1);
}

const LABEL = "text-xs font-medium text-muted-foreground";

export function BuildImageDialog({ connectionId, engineName, open, onOpenChange }: Props) {
  const [contextDir, setContextDir] = useState("");
  const [dockerfile, setDockerfile] = useState("Dockerfile");
  const [tags, setTags] = useState<string[]>([""]);
  const [args, setArgs] = useState<Arg[]>([]);
  const [target, setTarget] = useState("");
  const [platform, setPlatform] = useState("");
  const [noCache, setNoCache] = useState(false);
  const [pull, setPull] = useState(false);
  const [touched, setTouched] = useState(false);
  const [serverError, setServerError] = useState<string | null>(null);
  const build = useBuildImage(connectionId);
  const { setCenterOpen } = useOperations();

  useEffect(() => {
    if (open) {
      setContextDir("");
      setDockerfile("Dockerfile");
      setTags([""]);
      setArgs([]);
      setTarget("");
      setPlatform("");
      setNoCache(false);
      setPull(false);
      setTouched(false);
      setServerError(null);
    }
  }, [open]);

  const tagProblems = tags.map((t) => (t.trim() ? quickRefProblem(t) : null));
  const dockerfileProblem = !dockerfile.trim()
    ? "Enter a Dockerfile path relative to the context"
    : /^([a-zA-Z]:|[\\/])/.test(dockerfile.trim()) || dockerfile.split(/[\\/]/).includes("..")
      ? "The Dockerfile must be inside the build context (relative path)"
      : null;
  const argProblem = args.find((a) => a.key.trim() === "" && a.value !== "");
  const problem = !contextDir
    ? "Choose a build context directory"
    : (dockerfileProblem ?? tagProblems.find((p) => p) ?? (argProblem ? "Build argument names cannot be empty" : null));

  const pickContext = async () => {
    try {
      const dir = await DialogService.PickDirectory("Choose build context directory");
      if (dir) {
        setContextDir(dir);
        setServerError(null);
      }
    } catch (e) {
      toast.error("Could not open the folder dialog", { description: errorMessage(e) });
    }
  };

  const pickDockerfile = async () => {
    try {
      const file = await DialogService.PickFile("Choose Dockerfile", "All files", "*");
      if (!file) return;
      const rel = contextDir ? relativeInside(contextDir, file) : null;
      if (rel) {
        setDockerfile(rel);
        setServerError(null);
      } else {
        setServerError(contextDir ? "The Dockerfile must be inside the build context directory." : "Choose the build context directory first.");
      }
    } catch (e) {
      toast.error("Could not open the file dialog", { description: errorMessage(e) });
    }
  };

  const submit = async () => {
    setTouched(true);
    if (problem) return;
    setServerError(null);
    const buildArgs: Record<string, string> = {};
    for (const a of args) if (a.key.trim()) buildArgs[a.key.trim()] = a.value;
    try {
      const op = await build.mutateAsync({
        contextDir,
        dockerfile: dockerfile.trim(),
        tags: tags.map((t) => t.trim()).filter(Boolean),
        buildArgs,
        target: target.trim(),
        platform: platform.trim(),
        noCache,
        pull,
      });
      onOpenChange(false);
      toast.success(`Build started: ${op.resource}`, {
        description: "Build output is shown in the Activity center.",
        action: { label: "View", onClick: () => setCenterOpen(true) },
      });
    } catch (e) {
      setServerError(errorMessage(e));
    }
  };

  const shown = serverError ?? (touched ? problem : null);

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[90vh] overflow-y-auto sm:max-w-xl">
        <DialogHeader>
          <DialogTitle>Build image</DialogTitle>
          <DialogDescription>
            Build on <span className="font-medium text-foreground">{engineName}</span>. Closing this dialog does not cancel the build; output and cancel are in
            the Activity center.
          </DialogDescription>
        </DialogHeader>

        <div className="flex gap-2 rounded-[10px] border border-border bg-muted/40 p-3 text-xs text-muted-foreground">
          <Info className="mt-0.5 size-4 shrink-0 text-info" />
          <p>
            The build context folder is read <span className="font-medium text-foreground">on this computer</span> and uploaded to{" "}
            <span className="font-medium text-foreground">{engineName}</span>, including for WSL and remote Engines. Files matched by{" "}
            <code className="font-mono">.dockerignore</code> are not uploaded. The Engine&apos;s classic builder is used, so BuildKit-only Dockerfile features
            may not work.
          </p>
        </div>

        <form
          onSubmit={(e) => {
            e.preventDefault();
            void submit();
          }}
          className="space-y-3"
        >
          <div className="space-y-1">
            <label htmlFor="build-context" className={LABEL}>
              Build context directory
            </label>
            <div className="flex gap-2">
              <Input
                id="build-context"
                value={contextDir}
                onChange={(e) => {
                  setContextDir(e.target.value);
                  setServerError(null);
                }}
                placeholder="C:\path\to\project"
                className="font-mono"
                spellCheck={false}
                autoComplete="off"
              />
              <Button type="button" variant="outline" onClick={() => void pickContext()}>
                <FolderOpen className="size-4" /> Browse…
              </Button>
            </div>
          </div>

          <div className="space-y-1">
            <label htmlFor="build-dockerfile" className={LABEL}>
              Dockerfile (relative to the context)
            </label>
            <div className="flex gap-2">
              <Input
                id="build-dockerfile"
                value={dockerfile}
                onChange={(e) => {
                  setDockerfile(e.target.value);
                  setServerError(null);
                }}
                className="font-mono"
                spellCheck={false}
                autoComplete="off"
              />
              <Button type="button" variant="outline" onClick={() => void pickDockerfile()}>
                Select…
              </Button>
            </div>
          </div>

          <div className="space-y-1">
            <span className={LABEL}>Tags (optional)</span>
            {tags.map((t, i) => (
              <div key={i} className="flex gap-2">
                <Input
                  aria-label={`Tag ${i + 1}`}
                  value={t}
                  onChange={(e) => setTags((all) => all.map((x, j) => (j === i ? e.target.value : x)))}
                  placeholder="myapp:1.0"
                  className="font-mono"
                  aria-invalid={!!tagProblems[i]}
                  spellCheck={false}
                  autoComplete="off"
                />
                {tags.length > 1 && (
                  <Button type="button" variant="ghost" size="icon" className="size-8" aria-label={`Remove tag ${i + 1}`} title={`Remove tag ${i + 1}`} onClick={() => setTags((all) => all.filter((_, j) => j !== i))}>
                    <X className="size-4" />
                  </Button>
                )}
              </div>
            ))}
            {tagProblems.find((p) => p) && <p className="text-xs text-destructive">{tagProblems.find((p) => p)}</p>}
            <Button type="button" variant="ghost" size="sm" onClick={() => setTags((all) => [...all, ""])}>
              <Plus className="size-3.5" /> Add tag
            </Button>
          </div>

          <div className="space-y-1">
            <span className={LABEL}>Build arguments (optional)</span>
            {args.map((a) => (
              <div key={a.id} className="flex gap-2">
                <Input
                  aria-label="Argument name"
                  value={a.key}
                  onChange={(e) => setArgs((all) => all.map((x) => (x.id === a.id ? { ...x, key: e.target.value } : x)))}
                  placeholder="NAME"
                  className="w-2/5 font-mono"
                  spellCheck={false}
                  autoComplete="off"
                />
                <Input
                  aria-label="Argument value"
                  value={a.value}
                  onChange={(e) => setArgs((all) => all.map((x) => (x.id === a.id ? { ...x, value: e.target.value } : x)))}
                  placeholder="value"
                  className="font-mono"
                  spellCheck={false}
                  autoComplete="off"
                />
                <Button type="button" variant="ghost" size="icon" className="size-8" aria-label="Remove argument" title="Remove argument" onClick={() => setArgs((all) => all.filter((x) => x.id !== a.id))}>
                  <X className="size-4" />
                </Button>
              </div>
            ))}
            <Button type="button" variant="ghost" size="sm" onClick={() => setArgs((all) => [...all, { id: nextId++, key: "", value: "" }])}>
              <Plus className="size-3.5" /> Add argument
            </Button>
          </div>

          <div className="grid grid-cols-2 gap-3">
            <div className="space-y-1">
              <label htmlFor="build-target" className={LABEL}>
                Target stage (optional)
              </label>
              <Input id="build-target" value={target} onChange={(e) => setTarget(e.target.value)} className="font-mono" spellCheck={false} autoComplete="off" />
            </div>
            <div className="space-y-1">
              <label htmlFor="build-platform" className={LABEL}>
                Platform (optional)
              </label>
              <Input
                id="build-platform"
                list="build-platforms"
                value={platform}
                onChange={(e) => setPlatform(e.target.value)}
                placeholder="Engine default"
                className="font-mono"
                spellCheck={false}
                autoComplete="off"
              />
              <datalist id="build-platforms">
                {PLATFORMS.map((p) => (
                  <option key={p} value={p} />
                ))}
              </datalist>
            </div>
          </div>

          <div className="flex flex-wrap gap-x-6 gap-y-2 text-[13px]">
            <label className="flex items-center gap-2">
              <input type="checkbox" checked={noCache} onChange={(e) => setNoCache(e.target.checked)} className="size-3.5 accent-primary" />
              Do not use cache
            </label>
            <label className="flex items-center gap-2">
              <input type="checkbox" checked={pull} onChange={(e) => setPull(e.target.checked)} className="size-3.5 accent-primary" />
              Always pull newer base images
            </label>
          </div>

          {shown && <p className="whitespace-pre-wrap text-xs text-destructive">{shown}</p>}

          <DialogFooter className="pt-2">
            <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
              Cancel
            </Button>
            <Button type="submit" disabled={build.isPending}>
              {build.isPending && <Loader2 className="size-4 animate-spin" />}
              Build
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
