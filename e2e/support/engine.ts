import { execFileSync } from "node:child_process";
import path from "node:path";
import { fileURLToPath } from "node:url";

/** The e2e folder: where docker-compose.yml of the test stack lives. */
export const e2eDir = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");

/** Runs `docker compose …` for the test stack and returns its output. */
export function compose(args: string[], input?: string): string {
  return execFileSync("docker", ["compose", ...args], { cwd: e2eDir, input, encoding: "utf8", stdio: ["pipe", "pipe", "pipe"] });
}

/** Runs a shell script inside the test engine container. */
export function engineSh(script: string): string {
  return compose(["exec", "-T", "engine", "sh", "-ec", script]);
}

/** Runs a shell script inside the Docklet container: where Compose files of a TCP connection live. */
export function dockletSh(script: string): string {
  return compose(["exec", "-T", "docklet", "sh", "-ec", script]);
}

/** Runs `docker …` against the test engine and returns trimmed output. */
export function engineDocker(...args: string[]): string {
  return compose(["exec", "-T", "engine", "docker", ...args]).trim();
}

/** State of a container in the test engine ("running", "exited", …), or "missing". */
export function containerState(name: string): string {
  try {
    return engineDocker("inspect", "-f", "{{.State.Status}}", name);
  } catch {
    return "missing";
  }
}
