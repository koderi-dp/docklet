import type { WSLInfo } from "@bindings/model";

export type WslActionId = "reclaim-memory" | "restart-docker" | "restart-distro" | "shutdown";

export interface WslActionText {
  id: WslActionId;
  title: string;
  /** One line shown next to the button. */
  summary: string;
  /** Label of the confirm button. */
  confirm: string;
  /** What exactly will happen; shown in the confirmation before anything runs. */
  effects: (info: Pick<WSLInfo, "distro" | "otherRunning">) => string[];
}

const others = (i: Pick<WSLInfo, "otherRunning">) => i.otherRunning ?? [];
const names = (list: string[]) => list.join(", ");

export const ACTIONS: Record<WslActionId, WslActionText> = {
  "reclaim-memory": {
    id: "reclaim-memory",
    title: "Reclaim memory",
    summary: "Drops the Linux page cache. Containers keep running.",
    confirm: "Reclaim memory",
    effects: (i) => [
      `Drops the page cache inside ${i.distro} so Linux gives that memory back.`,
      "Nothing is stopped: containers, shells and editors keep running and no file changes. Reads can be a little slower while the cache fills again.",
      "How much of it Windows takes back depends on the memory reclaim setting of WSL.",
      "Memory before and after is written to the operation log.",
    ],
  },
  "restart-docker": {
    id: "restart-docker",
    title: "Restart Docker",
    summary: "Restarts the Docker service in this distribution.",
    confirm: "Restart Docker",
    effects: (i) => [
      `Restarts the Docker service inside ${i.distro}.`,
      "All containers on this engine stop. Only containers with a restart policy (always, unless-stopped or on-failure) come back by themselves; the others stay stopped.",
      "Docklet reconnects when Docker answers again.",
    ],
  },
  "restart-distro": {
    id: "restart-distro",
    title: "Restart distribution",
    summary: "Stops this distribution and starts it again.",
    confirm: "Restart distribution",
    effects: (i) => [
      `Stops ${i.distro} and starts it again.`,
      `Everything running in ${i.distro} stops: containers, shells and editors connected to it (VS Code, terminals).`,
      others(i).length > 0
        ? `Other distributions are not affected (running now: ${names(others(i))}).`
        : "No other distribution is running.",
      "Docker is started again afterwards and Docklet reconnects.",
    ],
  },
  shutdown: {
    id: "shutdown",
    title: "Shut down WSL",
    summary: "Stops every distribution and the WSL virtual machine.",
    confirm: "Shut down WSL",
    effects: (i) => [
      "Stops the whole WSL virtual machine, so ALL distributions stop, not only this one.",
      others(i).length > 0
        ? `Other distributions that are running now and will stop: ${names(others(i))}.`
        : `${i.distro} is the only running distribution.`,
      "Docker Desktop stops too if it uses WSL. Everything running inside any distribution is stopped, including unsaved work in connected editors.",
      `Docklet disconnects. WSL starts again the next time ${i.distro} is used, for example when you connect again.`,
    ],
  },
};

/** Display order: the shutdown is deliberately last and rendered apart. */
export const SAFE_ACTIONS: WslActionId[] = ["reclaim-memory", "restart-docker", "restart-distro"];
