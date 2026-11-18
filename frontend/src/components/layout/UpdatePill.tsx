import { ArrowUpCircle } from "lucide-react";
import { Link } from "react-router-dom";
import { useUpdateStatus } from "@/hooks/useApp";

/** Small status-bar notice when a newer release exists and was not dismissed. Never a modal. */
export function UpdatePill() {
  const { query } = useUpdateStatus();
  const st = query.data;
  if (!st?.updateAvailable || st.dismissed) return null;
  return (
    <Link
      to="/settings"
      title="Open Settings to read the release notes or dismiss this version"
      className="ml-auto flex items-center gap-1.5 rounded-full bg-primary/15 px-2 py-0.5 text-primary hover:bg-primary/25"
    >
      <ArrowUpCircle className="size-3.5" />
      Update {st.latestVersion} available
    </Link>
  );
}
