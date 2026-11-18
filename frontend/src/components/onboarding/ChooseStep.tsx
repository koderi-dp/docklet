import { type KeyboardEvent } from "react";
import { Plus, RefreshCw } from "lucide-react";
import { Button } from "@/components/ui/button";
import { useUI } from "@/stores/uiStore";
import { isWindows } from "@/lib/platform";
import { cn } from "@/lib/utils";
import { CandidateCard } from "./CandidateCard";
import { NothingFound } from "./NothingFound";
import { StepFrame } from "./StepFrame";
import type { Onboarding } from "./useOnboarding";

/** Step 2: the engines that were found, best first, one preselected. */
export function ChooseStep({ ob, onSkip }: { ob: Onboarding; onSkip: () => void }) {
  const { candidates, selectedId } = ob.state;
  const openDialog = useUI((s) => s.openConnectionDialog);
  const addRemote = () =>
    openDialog({
      mode: "add",
      onSaved: (p) =>
        ob.dispatch({
          type: "add",
          candidate: {
            id: `saved:${p.id}`, kind: "saved", name: p.name, profile: p, savedId: p.id, state: "unchecked",
            viaDesktop: false, isDefault: false, facts: "Saved. It is tested in the next step.",
          },
        }),
    });

  const onKey = (e: KeyboardEvent) => {
    const d = e.key === "ArrowDown" || e.key === "ArrowRight" ? 1 : e.key === "ArrowUp" || e.key === "ArrowLeft" ? -1 : 0;
    if (!d || !candidates.length) return;
    e.preventDefault();
    const i = candidates.findIndex((c) => c.id === selectedId);
    const next = candidates[(i + d + candidates.length) % candidates.length];
    ob.dispatch({ type: "select", id: next.id });
    requestAnimationFrame(() => document.querySelector<HTMLElement>(`[data-candidate="${CSS.escape(next.id)}"]`)?.focus());
  };

  const nothing = candidates.length === 0;
  return (
    <StepFrame
      title={nothing ? "No engine found yet" : "Choose an engine"}
      lede={nothing ? "Docklet looked and found nothing it can connect to." : "These are the engines Docklet found, best first. You can change this later."}
      secondary={
        nothing ? (
          <Button variant="ghost" onClick={onSkip}>
            Skip for now
          </Button>
        ) : (
          // "Skip setup" is always in the rail; here Back matches the other steps.
          <Button variant="ghost" onClick={() => ob.dispatch({ type: "back" })}>
            Back
          </Button>
        )
      }
      actions={
        nothing ? (
          <>
            <Button variant="outline" onClick={addRemote}>
              <Plus />
              Add a remote engine…
            </Button>
            <Button data-primary onClick={() => void ob.recheck()} disabled={ob.detecting}>
              <RefreshCw className={cn(ob.detecting && "animate-spin")} />
              Check again
            </Button>
          </>
        ) : (
          <Button data-primary onClick={() => ob.advance({ text: ob.selected?.name ?? "" })} disabled={!ob.selected}>
            Continue
          </Button>
        )
      }
    >
      {nothing ? (
        <NothingFound os={ob.report?.os ?? (isWindows ? "windows" : "linux")} />
      ) : (
        <>
          <div role="radiogroup" aria-label="Docker engine" className="space-y-2" onKeyDown={onKey}>
            {candidates.map((c, i) => (
              <CandidateCard
                key={c.id}
                c={c}
                selected={c.id === selectedId}
                recommended={i === 0 && c.state === "running" && candidates.length > 1}
                onSelect={() => ob.dispatch({ type: "select", id: c.id })}
              />
            ))}
          </div>
          <Button variant="link" size="sm" className="mt-3 -ml-2.5" onClick={addRemote}>
            <Plus />
            Add a remote engine (SSH or TCP)…
          </Button>
        </>
      )}
    </StepFrame>
  );
}
