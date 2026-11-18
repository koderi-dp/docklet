import { useEffect } from "react";
import { AppService } from "@bindings/services";
import { useAppSettings } from "@/hooks/useApp";
import { useConnections } from "@/hooks/useActiveConnection";
import { shouldOnboard } from "@/lib/onboarding";

/**
 * Decides which view the app shows before its first paint: "loading" until the stored setup state and the
 * saved connections are known, then "onboarding" or "app". A failure to read either never traps the user in the wizard.
 */
export function useOnboardingGate(): "loading" | "onboarding" | "app" {
  const { query } = useAppSettings();
  const conns = useConnections();
  const settings = query.data;
  const connections = conns.data;
  const failed = query.isError || conns.isError;
  const should = settings && connections ? shouldOnboard(settings.onboarding, connections) : null;

  // First run: remember that setup is pending, so saving a connection inside the wizard does not end it.
  useEffect(() => {
    if (should && settings && settings.onboarding === "") void AppService.SetOnboarding("pending").then(() => query.refetch()).catch(() => undefined);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [should]);

  if (failed) return "app";
  if (should === null) return "loading";
  return should ? "onboarding" : "app";
}
