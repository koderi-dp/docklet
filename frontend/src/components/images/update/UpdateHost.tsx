import { UpdateImageDialog } from "./UpdateImageDialog";
import { useUpdateStore } from "./updateStore";

/** Mount once on the Images page: renders the "Update…" dialog for the image chosen in a menu or chip. */
export function UpdateHost() {
  const target = useUpdateStore((s) => s.target);
  const close = useUpdateStore((s) => s.closeDialog);
  if (!target) return null;
  return <UpdateImageDialog key={`${target.connectionId}|${target.ref}`} connectionId={target.connectionId} imageRef={target.ref} onClose={close} />;
}
