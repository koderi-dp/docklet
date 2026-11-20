import { useEffect, useState } from "react";
import { ChevronDown, ChevronRight, FolderOpen, Loader2, Plus, X } from "lucide-react";
import { toast } from "sonner";
import { DialogService } from "@bindings/services";
import type { Operation } from "@bindings/model";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from "@/components/ui/collapsible";
import { useBuildImage } from "@/hooks/useImages";
import { useOperations } from "@/hooks/useOperations";
import { quickRefProblem } from "../imageFilters";
import { errorMessage } from "@/lib/errors";
import { BuildAdvancedFields } from "./BuildAdvancedFields";
import { BuilderLine } from "./BuilderLine";
import { emptyForm, toSpec, type BuildForm } from "./buildForm";
import { relativeInside } from "./buildFormat";
import { useBuildPlan } from "./useBuildApi";

interface Props {
  connectionId: string;
  engineName: string;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /** Values to start from (Build again); null starts empty. */
  initial?: BuildForm | null;
  /** Called after the Engine or CLI accepted the build. */
  onStarted?: (op: Operation) => void;
}

const LABEL = "text-xs font-medium text-muted-foreground";

export function BuildImageDialog({ connectionId, engineName, open, onOpenChange, initial, onStarted }: Props) {
  const [form, setForm] = useState<BuildForm>(emptyForm);
  const [advanced, setAdvanced] = useState(false);
  const [touched, setTouched] = useState(false);
  const [serverError, setServerError] = useState<string | null>(null);
  const build = useBuildImage(connectionId);
  const { openOperations } = useOperations();
  const plan = useBuildPlan(connectionId, form, open);

  useEffect(() => {
    if (open) {
      const start = initial ?? emptyForm();
      setForm(start);
      setAdvanced(start.args.length > 0 || !!start.target || !!start.platform || start.noCache || start.pull);
      setTouched(false);
      setServerError(null);
    }
    // Only a (re)open starts from `initial`.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open]);

  const patch = (p: Partial<BuildForm>) => {
    setForm((f) => ({ ...f, ...p }));
    setServerError(null);
  };
  const tagProblems = form.tags.map((t) => (t.trim() ? quickRefProblem(t) : null));
  const dockerfileProblem = !form.dockerfile.trim()
    ? "Enter a Dockerfile path relative to the context"
    : /^([a-zA-Z]:|[\\/])/.test(form.dockerfile.trim()) || form.dockerfile.split(/[\\/]/).includes("..")
      ? "The Dockerfile must be inside the build context (relative path)"
      : null;
  const argProblem = form.args.find((a) => a.key.trim() === "" && a.value !== "");
  const planProblem = plan.pending ? null : (plan.data?.contextProblem ?? null);
  const problem = !form.contextDir.trim()
    ? "Choose a build context directory"
    : (dockerfileProblem ?? tagProblems.find((p) => p) ?? (argProblem ? "Build argument names cannot be empty" : planProblem));

  const pickContext = async () => {
    try {
      const dir = await DialogService.PickDirectory("Choose build context directory");
      if (dir) patch({ contextDir: dir });
    } catch (e) {
      toast.error("Could not open the folder dialog", { description: errorMessage(e) });
    }
  };

  const pickDockerfile = async () => {
    try {
      const file = await DialogService.PickFile("Choose Dockerfile", "All files", "*");
      if (!file) return;
      const rel = form.contextDir ? relativeInside(form.contextDir, file) : null;
      if (rel) patch({ dockerfile: rel });
      else setServerError(form.contextDir ? "The Dockerfile must be inside the build context directory." : "Choose the build context directory first.");
    } catch (e) {
      toast.error("Could not open the file dialog", { description: errorMessage(e) });
    }
  };

  const submit = async () => {
    setTouched(true);
    if (problem) return;
    setServerError(null);
    try {
      const op = await build.mutateAsync(toSpec(form));
      onOpenChange(false);
      onStarted?.(op);
      toast.success(`Build started: ${op.resource}`, {
        description: "Progress is shown on the Builds tab and in Operations.",
        action: { label: "View", onClick: () => openOperations(op.id) },
      });
    } catch (e) {
      setServerError(errorMessage(e));
    }
  };

  const shown = serverError ?? (touched && !planProblem ? problem : null);
  const setTag = (i: number, v: string) => patch({ tags: form.tags.map((x, j) => (j === i ? v : x)) });

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[90vh] overflow-y-auto overflow-x-hidden sm:max-w-xl [&>*]:min-w-0">
        <DialogHeader>
          <DialogTitle>Build image</DialogTitle>
          <DialogDescription>
            Build on <span className="font-medium text-foreground">{engineName}</span>. Closing this dialog does not cancel the build; progress and cancel are on
            the Builds tab and in Operations.
          </DialogDescription>
        </DialogHeader>

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
                value={form.contextDir}
                onChange={(e) => patch({ contextDir: e.target.value })}
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
              <Input id="build-dockerfile" value={form.dockerfile} onChange={(e) => patch({ dockerfile: e.target.value })} className="font-mono" spellCheck={false} autoComplete="off" />
              <Button type="button" variant="outline" onClick={() => void pickDockerfile()}>
                Select…
              </Button>
            </div>
          </div>

          <div className="space-y-1">
            <span className={LABEL}>Tags (optional)</span>
            {form.tags.map((t, i) => (
              <div key={i} className="flex gap-2">
                <Input
                  aria-label={`Tag ${i + 1}`}
                  value={t}
                  onChange={(e) => setTag(i, e.target.value)}
                  placeholder="myapp:1.0"
                  className="font-mono"
                  aria-invalid={!!tagProblems[i]}
                  spellCheck={false}
                  autoComplete="off"
                />
                {form.tags.length > 1 && (
                  <Button type="button" variant="ghost" size="icon" className="size-8 shrink-0" aria-label={`Remove tag ${i + 1}`} title={`Remove tag ${i + 1}`} onClick={() => patch({ tags: form.tags.filter((_, j) => j !== i) })}>
                    <X className="size-4" />
                  </Button>
                )}
              </div>
            ))}
            {tagProblems.find((p) => p) && <p className="text-xs text-destructive">{tagProblems.find((p) => p)}</p>}
            <Button type="button" variant="ghost" size="sm" onClick={() => patch({ tags: [...form.tags, ""] })}>
              <Plus className="size-3.5" /> Add tag
            </Button>
          </div>

          <BuilderLine plan={plan.data} pending={plan.pending} failed={plan.isError ? errorMessage(plan.error) : null} builder={form.builder} onBuilder={(builder) => patch({ builder })} />

          <Collapsible open={advanced} onOpenChange={setAdvanced}>
            <CollapsibleTrigger asChild>
              <button type="button" className="inline-flex items-center gap-1 text-xs font-medium text-muted-foreground hover:text-foreground">
                {advanced ? <ChevronDown className="size-3.5" /> : <ChevronRight className="size-3.5" />}
                Advanced
                <span className="font-normal">· build arguments, target stage, platform, cache</span>
              </button>
            </CollapsibleTrigger>
            <CollapsibleContent>
              <BuildAdvancedFields form={form} onChange={patch} />
            </CollapsibleContent>
          </Collapsible>

          {shown && <p className="whitespace-pre-wrap text-xs text-destructive">{shown}</p>}

          <DialogFooter className="pt-2">
            <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
              Cancel
            </Button>
            <Button type="submit" disabled={build.isPending || !!planProblem}>
              {build.isPending && <Loader2 className="size-4 animate-spin" />}
              Build
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
