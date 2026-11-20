import { ContainerService, ImageService } from "@bindings/services";
import { OperationStatus, type ContainerCreateSpec } from "@bindings/model";
import { awaitOperation } from "@/lib/awaitOperation";
import { errorMessage } from "@/lib/errors";

export const HELLO_IMAGE = "hello-world:latest";
const GREETING = "Hello from Docker!";

export type HelloStep = "pulling" | "running" | "removing";

export interface HelloResult {
  ok: boolean;
  /** The real output line when the container printed it. */
  line?: string;
  error?: string;
  /** The container could not be removed afterwards. */
  leftover?: string;
}

function spec(name: string): ContainerCreateSpec {
  return {
    name, image: HELLO_IMAGE, command: "", env: [], ports: [], mounts: [], restartPolicy: "no", maxRetries: 0, memoryMiB: 0, cpus: 0,
    autoStart: true, entrypoint: "", workDir: "", user: "", hostname: "", labels: { "docklet.setup": "hello-world" }, stopSignal: "", stopTimeout: null,
    networkMode: "", networks: [],
  };
}

async function readGreeting(connectionId: string, id: string): Promise<string | undefined> {
  for (let i = 0; i < 20; i++) {
    const lines = await ContainerService.Logs(connectionId, id, 50);
    const hit = (lines ?? []).find((l) => l.text.includes(GREETING));
    if (hit) return hit.text.trim();
    await new Promise((r) => setTimeout(r, 500));
  }
}

/**
 * Pulls hello-world (an operation, also listed in Operations), runs it once and removes the container again.
 * The image stays on the engine. Everything reported here was read from the engine.
 */
export async function runHelloWorld(connectionId: string, onStep: (s: HelloStep) => void): Promise<HelloResult> {
  onStep("pulling");
  const op = await awaitOperation((await ImageService.Pull(connectionId, HELLO_IMAGE)).id);
  if (op.status !== OperationStatus.OpSucceeded) return { ok: false, error: op.error || "The image could not be pulled." };

  onStep("running");
  let id = "";
  const result: HelloResult = { ok: false };
  try {
    const created = await ContainerService.Create(connectionId, spec(`docklet-hello-${Date.now().toString(36)}`));
    id = created.id;
    if (created.error) throw new Error(created.error);
    const line = await readGreeting(connectionId, id);
    if (line) Object.assign(result, { ok: true, line });
    else result.error = `The container ran but did not print "${GREETING}".`;
  } catch (e) {
    result.error = errorMessage(e);
  } finally {
    if (id) {
      onStep("removing");
      try {
        const [r] = (await ContainerService.Remove(connectionId, [id], true)) ?? [];
        if (r && !r.ok) result.leftover = r.error || "The container could not be removed.";
      } catch (e) {
        result.leftover = errorMessage(e);
      }
    }
  }
  return result;
}
