import { useState } from "react";
import { useNavigate, useRouteError } from "react-router-dom";
import { TriangleAlert } from "lucide-react";
import { Button } from "@/components/ui/button";
import { errorMessage } from "@/lib/errors";

/** Shown instead of a blank window when a screen throws while rendering. */
export default function RouteError() {
  const error = useRouteError();
  const navigate = useNavigate();
  const [showDetails, setShowDetails] = useState(false);
  const message = errorMessage(error);
  const stack = error instanceof Error ? error.stack : undefined;

  return (
    <div className="flex h-full flex-col items-center justify-center gap-3 p-6 text-center">
      <TriangleAlert className="size-8 text-destructive" />
      <h1 className="text-lg font-semibold">This screen hit an unexpected error</h1>
      <p className="max-w-md text-sm text-muted-foreground">
        Your Docker Engine was not affected. Reload the screen, or go back to Containers.
      </p>
      <div className="flex gap-2">
        <Button size="sm" onClick={() => window.location.reload()}>
          Reload
        </Button>
        <Button size="sm" variant="outline" onClick={() => navigate("/containers")}>
          Go to Containers
        </Button>
        <Button size="sm" variant="ghost" onClick={() => setShowDetails((v) => !v)}>
          {showDetails ? "Hide details" : "Show details"}
        </Button>
      </div>
      {showDetails && (
        <pre className="max-h-60 w-full max-w-2xl overflow-auto rounded-md border border-border bg-muted/40 p-3 text-left font-mono text-[11px]">
          {stack ?? message}
        </pre>
      )}
    </div>
  );
}
