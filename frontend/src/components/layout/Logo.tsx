import { cn } from "@/lib/utils";

/** The Docklet mark: a white D holding three containers, on the violet tile (same art as the app icon). */
export function Logo({ className }: { className?: string }) {
  return (
    <svg viewBox="0 0 1024 1024" className={cn("shrink-0", className)} role="img" aria-label="Docklet">
      <defs>
        <linearGradient id="docklet-logo-bg" x1="0" y1="0" x2="1" y2="1">
          <stop offset="0" stopColor="#8B7CFF" />
          <stop offset="1" stopColor="#4B3FD6" />
        </linearGradient>
      </defs>
      <rect x="64" y="64" width="896" height="896" rx="208" fill="url(#docklet-logo-bg)" />
      <path
        d="M 292 232 H 500 A 280 280 0 0 1 500 792 H 292 A 36 36 0 0 1 256 756 V 268 A 36 36 0 0 1 292 232 Z"
        fill="#FFFFFF"
      />
      <g fill="#5A4CE6">
        <rect x="352" y="356" width="200" height="80" rx="26" />
        <rect x="352" y="472" width="268" height="80" rx="26" />
        <rect x="352" y="588" width="200" height="80" rx="26" />
      </g>
    </svg>
  );
}
