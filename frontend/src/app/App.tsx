import RouteError from "./RouteError";
import { lazy, Suspense, useEffect, type ReactNode } from "react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createHashRouter, Navigate, RouterProvider } from "react-router-dom";
import { TooltipProvider } from "@/components/ui/tooltip";
import { Toaster } from "@/components/ui/sonner";
import { GC_MS, STALE_MS } from "@/hooks/queryUtils";
import { useUI } from "@/stores/uiStore";
import AppShell from "@/components/layout/AppShell";
import Unavailable from "@/pages/Unavailable";
import { useOnboardingGate } from "@/hooks/useOnboardingGate";

const OnboardingView = lazy(() => import("@/components/onboarding/OnboardingView"));
const ContainersPage = lazy(() => import("@/pages/ContainersPage"));
const ContainerDetailsPage = lazy(() => import("@/pages/ContainerDetailsPage"));
const ConnectionsPage = lazy(() => import("@/pages/ConnectionsPage"));
const DashboardPage = lazy(() => import("@/pages/DashboardPage"));
const ImagesPage = lazy(() => import("@/pages/ImagesPage"));
const ComposePage = lazy(() => import("@/pages/ComposePage"));
const ComposeProjectPage = lazy(() => import("@/pages/ComposeProjectPage"));
const VolumesPage = lazy(() => import("@/pages/VolumesPage"));
const NetworksPage = lazy(() => import("@/pages/NetworksPage"));
const SystemPage = lazy(() => import("@/pages/SystemPage"));
const SettingsPage = lazy(() => import("@/pages/SettingsPage"));

/** Same footprint as a page body so the shell never jumps while a route chunk loads. */
function PageFallback() {
  return (
    <div className="flex h-full flex-col gap-3 p-4" aria-busy="true">
      <div className="h-8 w-48 animate-pulse rounded-md bg-muted" />
      <div className="h-10 w-full animate-pulse rounded-md bg-muted" />
      <div className="min-h-0 flex-1 animate-pulse rounded-[10px] bg-muted/60" />
    </div>
  );
}

const page = (el: ReactNode) => <Suspense fallback={<PageFallback />}>{el}</Suspense>;

const queryClient = new QueryClient({
  defaultOptions: { queries: { refetchOnWindowFocus: false, staleTime: STALE_MS, gcTime: GC_MS, refetchIntervalInBackground: false } },
});

const router = createHashRouter([
  {
    path: "/",
    element: <AppShell />,
    errorElement: <RouteError />,
    children: [
      {
        errorElement: <RouteError />,
        children: [
      { index: true, element: <Navigate to="/containers" replace /> },
      { path: "containers", element: page(<ContainersPage />) },
      { path: "containers/:containerId", element: page(<ContainerDetailsPage />) },
      { path: "connections", element: page(<ConnectionsPage />) },
      { path: "dashboard", element: page(<DashboardPage />) },
      { path: "images", element: page(<ImagesPage />) },
      { path: "compose", element: page(<ComposePage />) },
      { path: "compose/:projectName", element: page(<ComposeProjectPage />) },
      { path: "volumes", element: page(<VolumesPage />) },
      { path: "networks", element: page(<NetworksPage />) },
      { path: "system", element: page(<SystemPage />) },
      { path: "settings", element: page(<SettingsPage />) },
      { path: "*", element: <Unavailable title="Not found" /> },
        ],
      },
    ],
  },
]);

/** Decides before the first paint between the first-run wizard and the app, so neither flashes. */
function Root() {
  const gate = useOnboardingGate();
  if (gate === "loading") return <div className="h-screen w-screen bg-background" aria-busy="true" />;
  if (gate === "onboarding")
    return (
      <Suspense fallback={<div className="h-screen w-screen bg-background" aria-busy="true" />}>
        <OnboardingView />
      </Suspense>
    );
  return <RouterProvider router={router} />;
}

export default function App() {
  const theme = useUI((s) => s.theme);
  useEffect(() => {
    document.documentElement.classList.toggle("dark", theme === "dark");
  }, [theme]);
  return (
    <QueryClientProvider client={queryClient}>
      <TooltipProvider delayDuration={300}>
        <Root />
        {/* Bottom right, clear of the 32px status bar. Sonner stacks toasts as a pile and fans them out on hover. */}
        <Toaster position="bottom-right" offset={{ bottom: 44, right: 16 }} visibleToasts={4} gap={8} closeButton />
      </TooltipProvider>
    </QueryClientProvider>
  );
}
