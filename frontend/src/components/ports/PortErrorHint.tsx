import { useEffect, useState, type ReactNode } from "react";
import { PortService } from "@bindings/services";
import type { PortCheck } from "@bindings/ports";
import { checkText, conflictingPortsInError, isInUse } from "@/lib/portCheck";

/**
 * Under a "port is already allocated" style error: a fresh check of the named ports says who holds them now,
 * with a link to the Ports view. Renders nothing for other errors, and nothing when the check finds nothing
 * (the Engine's own message stays the only text).
 */
export function PortErrorHint({ connectionId, message }: { connectionId: string; message: string }) {
  const [held, setHeld] = useState<PortCheck[]>([]);
  useEffect(() => {
    const ports = conflictingPortsInError(message);
    if (ports.length === 0) return;
    let live = true;
    PortService.Check(connectionId, ports.map((hostPort) => ({ hostIp: "", hostPort, protocol: "tcp" })), [])
      .then((r) => live && setHeld((r ?? []).filter(isInUse)))
      .catch(() => undefined); // a failed lookup leaves the Engine's message as it is
    return () => {
      live = false;
    };
  }, [connectionId, message]);
  if (held.length === 0) return null;
  return (
    <div className="mt-1.5 space-y-0.5 text-[13px]">
      {held.map((c) => (
        <p key={c.hostPort} className="font-medium">{checkText(c)}</p>
      ))}
      <a href={`#/system?tab=ports&q=${held[0].hostPort}`} className="text-primary underline-offset-2 hover:underline">
        Show what uses the port
      </a>
    </div>
  );
}

/** A toast description: the message, plus the holder of the port when it is a port conflict. */
export function portAwareDescription(connectionId: string, text: string): ReactNode {
  return (
    <div>
      <p className="whitespace-pre-line">{text}</p>
      <PortErrorHint connectionId={connectionId} message={text} />
    </div>
  );
}
