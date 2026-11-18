import { useEffect, useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { AppService } from "@bindings/services";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { ConnectionDialogHost } from "@/components/connections/ConnectionDialog";
import { appSettingsKey } from "@/hooks/useApp";
import { useSystemTheme } from "@/hooks/useSystemTheme";
import { toastError } from "@/lib/errors";
import { CheckStep } from "./CheckStep";
import { ChooseStep } from "./ChooseStep";
import { DetectStep } from "./DetectStep";
import { ReadyStep } from "./ReadyStep";
import { SetupStep } from "./SetupStep";
import { StepRail } from "./StepRail";
import { useOnboarding } from "./useOnboarding";

/** Enter runs the step's primary action unless focus is on something that uses Enter itself. */
function onEnter(e: KeyboardEvent) {
  if (e.key !== "Enter" || e.defaultPrevented) return;
  const t = e.target instanceof Element ? e.target : document.body;
  if (t.closest("button, a, input, textarea, select, [role=radio], [role=switch], [role=checkbox], [role=dialog], [role=alertdialog]")) return;
  document.querySelector<HTMLButtonElement>("[data-primary]:not(:disabled)")?.click();
}

/** The first-run setup wizard. It replaces the app until it is completed or skipped. */
export default function OnboardingView() {
  useSystemTheme();
  const qc = useQueryClient();
  const ob = useOnboarding();
  const [confirmSkip, setConfirmSkip] = useState(false);
  const [finishing, setFinishing] = useState(false);
  useEffect(() => {
    document.addEventListener("keydown", onEnter);
    return () => document.removeEventListener("keydown", onEnter);
  }, []);

  const leave = async (state: "completed" | "skipped") => {
    setFinishing(true);
    try {
      const settings = await AppService.SetOnboarding(state);
      window.location.hash = "#/containers";
      qc.setQueryData(appSettingsKey, settings);
    } catch (e) {
      setFinishing(false);
      toastError("Could not save the setup state", e);
    }
  };

  const { step } = ob.state;
  return (
    <div className="flex h-screen w-screen overflow-hidden bg-background text-foreground">
      <StepRail state={ob.state} onGoto={(s) => ob.dispatch({ type: "goto", step: s })} onSkip={() => setConfirmSkip(true)} />
      <main className="min-w-0 flex-1">
        {step === "detect" && <DetectStep ob={ob} />}
        {step === "choose" && <ChooseStep ob={ob} onSkip={() => setConfirmSkip(true)} />}
        {step === "setup" && ob.selected && <SetupStep key={ob.selected.id} ob={ob} />}
        {step === "check" && ob.selected && <CheckStep key={ob.selected.id} ob={ob} candidate={ob.selected} />}
        {step === "ready" && <ReadyStep ob={ob} finishing={finishing} onFinish={() => void leave("completed")} />}
      </main>
      <ConnectionDialogHost />
      <AlertDialog open={confirmSkip} onOpenChange={setConfirmSkip}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Skip setup?</AlertDialogTitle>
            <AlertDialogDescription>
              Docklet opens without finishing setup. Anything already done, such as a saved connection or a started distribution, stays as it is. You can run setup again any time from Settings, under Connections.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Keep going</AlertDialogCancel>
            <AlertDialogAction onClick={() => void leave("skipped")}>Skip setup</AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}
