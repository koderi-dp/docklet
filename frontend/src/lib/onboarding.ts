import type { ConnectionProfile } from "@bindings/model";
import { setupNeed, type Candidate } from "./onboardingCandidates";

export type StepId = "detect" | "choose" | "setup" | "check" | "ready";
export const STEP_ORDER: StepId[] = ["detect", "choose", "setup", "check", "ready"];
export type StepStatus = "pending" | "active" | "done" | "skipped" | "attention";

export interface StepResult {
  text: string;
  attention?: boolean;
}

export interface OnboardingState {
  step: StepId;
  candidates: Candidate[];
  selectedId: string | null;
  /** Short outcome of each finished step, shown under it in the rail. */
  results: Partial<Record<StepId, StepResult>>;
  /** Index in STEP_ORDER of the furthest step reached; earlier steps can be revisited. */
  reached: number;
}

export type OnboardingAction =
  | { type: "detected"; candidates: Candidate[] }
  | { type: "select"; id: string }
  | { type: "add"; candidate: Candidate }
  | { type: "saved"; id: string; savedId: string }
  | { type: "result"; step: StepId; result: StepResult }
  | { type: "next" }
  | { type: "back" }
  | { type: "goto"; step: StepId };

export const initialOnboarding: OnboardingState = { step: "detect", candidates: [], selectedId: null, results: {}, reached: 0 };

const index = (s: StepId) => STEP_ORDER.indexOf(s);

export function selectedCandidate(s: OnboardingState): Candidate | undefined {
  return s.candidates.find((c) => c.id === s.selectedId);
}

/** The setup step is only shown when the chosen engine needs something before it can be tested. */
export function needsSetup(s: OnboardingState): boolean {
  return setupNeed(selectedCandidate(s)) !== "none";
}

/** The step that follows `step` for the current choice (setup is skipped when nothing has to be set up). */
export function nextStep(s: OnboardingState, from: StepId = s.step): StepId | null {
  const n = STEP_ORDER[index(from) + 1];
  if (!n) return null;
  return n === "setup" && !needsSetup(s) ? "check" : n;
}

export function previousStep(s: OnboardingState, from: StepId = s.step): StepId | null {
  const p = STEP_ORDER[index(from) - 1];
  if (!p) return null;
  return p === "setup" && !needsSetup(s) ? "choose" : p;
}

/** The first step the user can leave: detection has to have produced its result. */
export function canAdvance(s: OnboardingState): boolean {
  switch (s.step) {
    case "detect":
      return !!s.results.detect;
    case "choose":
      return !!selectedCandidate(s);
    case "setup":
      return !needsSetup(s);
    case "check":
      return !!s.results.check && !s.results.check.attention;
    default:
      return false;
  }
}

/** A step can be revisited when it was reached, is not the current one and has a result to show. */
export function canGoto(s: OnboardingState, step: StepId): boolean {
  if (step === s.step || index(step) > s.reached || step === "ready") return false;
  return !!s.results[step] && (step !== "setup" || needsSetup(s));
}

export function stepStatuses(s: OnboardingState): Record<StepId, StepStatus> {
  const out = {} as Record<StepId, StepStatus>;
  for (const step of STEP_ORDER) {
    const i = index(step);
    if (step === s.step) out[step] = s.results[step]?.attention ? "attention" : "active";
    else if (i > s.reached) out[step] = "pending";
    else if (s.results[step]) out[step] = s.results[step]?.attention ? "attention" : "done";
    else if (step === "setup" && !needsSetup(s) && s.reached > i) out[step] = "skipped";
    else out[step] = "pending";
  }
  return out;
}

/** Clears what depends on the choice of engine, so a changed choice never shows another engine's results. */
function resetAfterChoice(s: OnboardingState): OnboardingState {
  const results = { ...s.results };
  for (const step of ["setup", "check", "ready"] as StepId[]) delete results[step];
  return { ...s, results, reached: Math.min(s.reached, index(s.step)) };
}

export function onboardingReducer(s: OnboardingState, a: OnboardingAction): OnboardingState {
  switch (a.type) {
    case "detected": {
      // Keep the user's choice when it is still there; otherwise preselect the best candidate.
      const keep = a.candidates.some((c) => c.id === s.selectedId);
      return { ...s, candidates: a.candidates, selectedId: keep ? s.selectedId : (a.candidates[0]?.id ?? null) };
    }
    case "select":
      if (a.id === s.selectedId || !s.candidates.some((c) => c.id === a.id)) return s;
      return resetAfterChoice({ ...s, selectedId: a.id });
    case "add":
      return resetAfterChoice({ ...s, candidates: [a.candidate, ...s.candidates.filter((c) => c.id !== a.candidate.id)], selectedId: a.candidate.id });
    case "saved":
      return { ...s, candidates: s.candidates.map((c) => (c.id === a.id ? { ...c, savedId: a.savedId } : c)) };
    case "result":
      return { ...s, results: { ...s.results, [a.step]: a.result } };
    case "next": {
      const n = canAdvance(s) ? nextStep(s) : null;
      return n ? { ...s, step: n, reached: Math.max(s.reached, index(n)) } : s;
    }
    case "back": {
      const p = previousStep(s);
      return p ? { ...s, step: p } : s;
    }
    case "goto":
      return canGoto(s, a.step) ? { ...s, step: a.step } : s;
  }
}

/** Decides before the first paint whether the wizard replaces the app. */
export function shouldOnboard(onboarding: string | undefined, connections: ConnectionProfile[]): boolean {
  if (onboarding === "pending") return true;
  if (onboarding) return false; // completed or skipped
  return !connections.some((c) => !c.discovered);
}
