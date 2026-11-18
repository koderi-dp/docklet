import { RefreshCw } from "lucide-react";
import { Button } from "@/components/ui/button";
import { detectRows, pendingTitles } from "@/lib/onboardingDetect";
import { isWindows } from "@/lib/platform";
import { cn } from "@/lib/utils";
import { ChecklistRow } from "./ChecklistRow";
import { Panel, StepFrame } from "./StepFrame";
import type { Onboarding } from "./useOnboarding";

/** Step 1: look at what this computer already has, with real checks, and say what was found. */
export function DetectStep({ ob }: { ob: Onboarding }) {
  const { report, detecting, detectError } = ob;
  const rows = report && !detecting ? detectRows(report) : null;
  return (
    <StepFrame
      title="Welcome to Docklet"
      lede="Docklet connects to a Docker Engine you already have, or helps you set one up. It does not bring its own. First, a look at what this computer has."
      secondary={
        <Button variant="ghost" onClick={() => void ob.recheck()} disabled={detecting}>
          <RefreshCw className={cn(detecting && "animate-spin")} />
          Check again
        </Button>
      }
      actions={
        <Button data-primary onClick={() => ob.advance()} disabled={detecting || (!report && !detectError)}>
          Continue
        </Button>
      }
    >
      <Panel>
        <ul className="divide-y" aria-busy={detecting} aria-live="polite">
          {rows
            ? rows.map((r) => <ChecklistRow key={r.id} title={r.title} detail={r.detail} tone={r.tone} items={r.items} />)
            : pendingTitles(isWindows, false).map((r) => <ChecklistRow key={r.id} title={r.title} pending />)}
        </ul>
      </Panel>
      {detectError && !detecting && (
        <p role="alert" className="mt-3 rounded-md border border-destructive/30 bg-destructive/10 p-3 text-sm break-words text-destructive">
          Could not check this computer: {detectError}
        </p>
      )}
    </StepFrame>
  );
}
