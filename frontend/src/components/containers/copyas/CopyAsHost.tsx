import { ConnectionStringDialog } from "./ConnectionStringDialog";
import { CopyAsTextDialog } from "./CopyAsTextDialog";
import { useCopyAsStore } from "./copyAsStore";

/** Mount once on pages that offer "Copy as…": renders the dialog the menus asked for. */
export function CopyAsHost() {
  const target = useCopyAsStore((s) => s.target);
  const close = useCopyAsStore((s) => s.close);
  if (!target) return null;
  const key = `${target.kind}:${target.containerId}`;
  if (target.kind === "connection") {
    return <ConnectionStringDialog key={key} connectionId={target.connectionId} containerId={target.containerId} name={target.name} onClose={close} />;
  }
  return <CopyAsTextDialog key={key} kind={target.kind} connectionId={target.connectionId} containerId={target.containerId} name={target.name} onClose={close} />;
}
