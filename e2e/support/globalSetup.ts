import { readFileSync } from "node:fs";
import path from "node:path";
import { compose, e2eDir } from "./engine";
import { COMPOSE_DIR } from "./names";

const port = process.env.DOCKLET_E2E_PORT ?? "9260";

/** Puts the test engine into a known state and gives Docklet an empty profile. */
export default async function globalSetup() {
  try {
    compose(["exec", "-T", "engine", "docker", "info", "--format", "{{.ServerVersion}}"]);
  } catch {
    throw new Error("The e2e stack is not running. Start it with `wails3 task e2e:up`, or run `wails3 task e2e`.");
  }
  const read = (...parts: string[]) => readFileSync(path.join(e2eDir, ...parts), "utf8");
  compose(["exec", "-T", "engine", "sh", "-c", `mkdir -p ${COMPOSE_DIR} && cat > ${COMPOSE_DIR}/compose.yaml`], read("projects", "demo", "compose.yaml"));
  compose(["exec", "-T", "engine", "sh"], read("support", "seed.sh"));

  // A new Docklet container has no saved connections and shows first-time setup again.
  compose(["up", "-d", "--force-recreate", "--no-deps", "docklet"]);
  await waitForDocklet();
}

async function waitForDocklet() {
  const deadline = Date.now() + 60_000;
  while (Date.now() < deadline) {
    try {
      if ((await fetch(`http://localhost:${port}/health`)).ok) return;
    } catch {
      // not up yet
    }
    await new Promise((r) => setTimeout(r, 500));
  }
  throw new Error(`Docklet did not answer on http://localhost:${port}/health within 60 seconds`);
}
