import { useCallback, useEffect, useMemo, useReducer, useState } from "react";
import { useEnvironment } from "@/hooks/useEnvironment";
import { initialOnboarding, needsSetup, onboardingReducer, selectedCandidate, type StepId, type StepResult } from "@/lib/onboarding";
import { buildCandidates } from "@/lib/onboardingCandidates";
import { detectSummary } from "@/lib/onboardingDetect";
import { errorMessage } from "@/lib/errors";

/** The wizard's state: the pure reducer plus the live detection that feeds it. */
export function useOnboarding() {
  const [state, dispatch] = useReducer(onboardingReducer, initialOnboarding);
  const env = useEnvironment();
  const [setupNotes, setSetupNotes] = useState<string[]>([]);
  const [advanceFor, setAdvanceFor] = useState<string | null>(null);

  useEffect(() => {
    if (!env.data) return;
    const candidates = buildCandidates(env.data);
    dispatch({ type: "detected", candidates });
    const sum = detectSummary(candidates);
    dispatch({ type: "result", step: "detect", result: { text: sum.text, attention: sum.attention } });
  }, [env.data]);

  // Notes describe the chosen engine; a different choice starts clean.
  useEffect(() => setSetupNotes([]), [state.selectedId]);

  useEffect(() => {
    if (env.isError) dispatch({ type: "result", step: "detect", result: { text: "Detection failed", attention: true } });
  }, [env.isError, env.errorUpdatedAt]);

  const advance = useCallback((result?: StepResult) => {
    if (result) dispatch({ type: "result", step: state.step, result });
    dispatch({ type: "next" });
  }, [state.step]);

  // After a setup action, move on by itself as soon as the engine no longer needs anything.
  useEffect(() => {
    if (!advanceFor || state.step !== "setup" || state.selectedId !== advanceFor || needsSetup(state) || env.isFetching) return;
    setAdvanceFor(null);
    dispatch({ type: "result", step: "setup", result: { text: [...new Set(setupNotes)].join(", ") || "Done" } });
    dispatch({ type: "next" });
  }, [advanceFor, state, env.isFetching, setupNotes]);

  const selected = selectedCandidate(state);
  const recheck = useCallback(() => env.refetch(), [env]);
  const note = useCallback((text: string) => setSetupNotes((n) => (n.includes(text) ? n : [...n, text])), []);

  return useMemo(
    () => ({
      state, dispatch, selected, advance, recheck, note, setupNotes,
      requestAdvance: () => setAdvanceFor(state.selectedId),
      detecting: env.isFetching,
      report: env.data,
      detectError: env.isError ? errorMessage(env.error) : null,
    }),
    [state, selected, advance, recheck, note, setupNotes, env.isFetching, env.data, env.isError, env.error],
  );
}

export type Onboarding = ReturnType<typeof useOnboarding>;
export type { StepId };
