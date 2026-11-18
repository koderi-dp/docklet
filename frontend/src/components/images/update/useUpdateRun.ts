import { useState } from "react";
import { ContainerService, ImageService } from "@bindings/services";
import { OperationStatus, type ContainerRecreateInfo } from "@bindings/model";
import { awaitOperation } from "@/lib/awaitOperation";
import { errorMessage } from "@/lib/errors";
import { checkImageUpdate } from "./updateStore";

export type ItemStatus = "pending" | "running" | "done" | "failed";

export interface RunItem {
  id: string;
  name: string;
  status: ItemStatus;
  error?: string;
}

export type Phase = "idle" | "pulling" | "recreating" | "done";

/**
 * "Update…": pull the image, then recreate each chosen standalone container one
 * after the other (the pull already happened, so Recreate runs with PullFirst=false).
 */
export function useUpdateRun(connectionId: string, ref: string) {
  const [phase, setPhase] = useState<Phase>("idle");
  const [pullError, setPullError] = useState<string | null>(null);
  const [items, setItems] = useState<RunItem[]>([]);

  const patch = (id: string, p: Partial<RunItem>) => setItems((list) => list.map((i) => (i.id === id ? { ...i, ...p } : i)));

  const run = async (chosen: ContainerRecreateInfo[]) => {
    setItems(chosen.map((c) => ({ id: c.id, name: c.name, status: "pending" })));
    setPullError(null);
    setPhase("pulling");
    try {
      const pull = await awaitOperation((await ImageService.Pull(connectionId, ref)).id);
      if (pull.status !== OperationStatus.OpSucceeded) {
        setPullError(pull.error || "The pull did not finish.");
        setPhase("done");
        return;
      }
    } catch (e) {
      setPullError(errorMessage(e));
      setPhase("done");
      return;
    }
    setPhase("recreating");
    for (const c of chosen) {
      patch(c.id, { status: "running" });
      try {
        const op = await awaitOperation((await ContainerService.Recreate(connectionId, c.id, c.spec, { pullFirst: false, keepOriginal: false })).id);
        if (op.status === OperationStatus.OpSucceeded) patch(c.id, { status: "done" });
        else patch(c.id, { status: "failed", error: op.error || "Cancelled" });
      } catch (e) {
        patch(c.id, { status: "failed", error: errorMessage(e) });
      }
    }
    void checkImageUpdate({ connectionId, ref });
    setPhase("done");
  };

  return { phase, pullError, items, run };
}
