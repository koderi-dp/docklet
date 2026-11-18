import { describe, expect, it } from "vitest";
import { ConnectionType, WSLInstallState, type ConnectionProfile, type EnvironmentReport, type WSLDistroReport } from "@bindings/model";
import { initialOnboarding, nextStep, onboardingReducer, shouldOnboard, stepStatuses, type OnboardingState } from "./onboarding";
import { buildCandidates, setupNeed } from "./onboardingCandidates";
import { detectRows, detectSummary } from "./onboardingDetect";

const docker = (over: Partial<WSLDistroReport["docker"]>): WSLDistroReport["docker"] => ({
  state: WSLInstallState.WSLStateNotInstalled, reachable: false, supported: false, ...over,
});
const distro = (name: string, over: Partial<WSLDistroReport> = {}): WSLDistroReport => ({
  name, version: 2, state: "Running", default: false, internal: false, docker: docker({}), ...over,
});
const report = (over: Partial<EnvironmentReport> = {}): EnvironmentReport => ({
  os: "windows", wsl: { available: true, distros: [] }, local: [], contexts: [], saved: [], ...over,
});
const local = (reachable: boolean): NonNullable<EnvironmentReport["local"]>[number] => ({
  profile: { id: "local-default", name: "Local Docker Engine", type: ConnectionType.ConnNamedPipe, endpoint: "npipe:////./pipe/docker_engine", discovered: true } as ConnectionProfile,
  reachable, serverVersion: reachable ? "29.1.0" : undefined, error: reachable ? undefined : "not running",
});

describe("candidate ranking", () => {
  it("puts a running engine before stopped Docker, a stopped distribution and one without Docker", () => {
    const names = buildCandidates(
      report({
        wsl: {
          available: true,
          distros: [
            distro("none", { docker: docker({ supported: true }) }),
            distro("off", { state: "Stopped", docker: docker({ state: WSLInstallState.WSLStateStopped }) }),
            distro("installed", { docker: docker({ state: WSLInstallState.WSLStateDaemonStopped }) }),
            distro("up", { docker: docker({ state: WSLInstallState.WSLStateRunning, reachable: true, version: "28.0.0" }) }),
          ],
        },
      }),
    ).map((c) => c.name);
    expect(names).toEqual(["up", "installed", "off", "none"]);
  });

  it("prefers the default WSL distribution when two are equally good", () => {
    const up = { state: WSLInstallState.WSLStateRunning, reachable: true, version: "28.0.0" };
    const list = buildCandidates(report({ wsl: { available: true, distros: [distro("a", { docker: docker(up) }), distro("b", { default: true, docker: docker(up) })] } }));
    expect(list.map((c) => c.name)).toEqual(["b", "a"]);
  });

  it("ranks a real engine above Docker Desktop's proxy and says which one it is", () => {
    const up = { reachable: true, version: "29.0.0" };
    const list = buildCandidates(
      report({
        wsl: {
          available: true,
          distros: [
            distro("proxy", { default: true, docker: docker({ ...up, state: WSLInstallState.WSLStateDockerDesktop }) }),
            distro("own", { docker: docker({ ...up, state: WSLInstallState.WSLStateRunning }) }),
          ],
        },
      }),
    );
    expect(list.map((c) => c.name)).toEqual(["own", "proxy"]);
    expect(list[1].facts).toContain("through Docker Desktop");
    expect(list[0].facts).toBe("own · WSL 2 · Docker Engine 29.0.0 running");
  });

  it("never offers an unreachable local engine or Docker Desktop's internal distribution", () => {
    const list = buildCandidates(report({ local: [local(false)], wsl: { available: true, distros: [distro("docker-desktop", { internal: true })] } }));
    expect(list).toEqual([]);
  });

  it("treats a saved connection to the same engine as that engine instead of a second card", () => {
    const saved = { id: "s1", name: "Local Docker Engine", type: ConnectionType.ConnNamedPipe, endpoint: "npipe:////./pipe/docker_engine" } as ConnectionProfile;
    const list = buildCandidates(report({ local: [local(true)], saved: [saved] }));
    expect(list).toHaveLength(1);
    expect(list[0].savedId).toBe("s1");
  });
});

describe("what a candidate needs before it can be tested", () => {
  const need = (d: Partial<WSLDistroReport>) => setupNeed(buildCandidates(report({ wsl: { available: true, distros: [distro("x", d)] } }))[0]);
  it.each([
    ["a stopped distribution is started first", { state: "Stopped", docker: docker({ state: WSLInstallState.WSLStateStopped }) }, "start-distro"],
    ["installed but stopped Docker is started", { docker: docker({ state: WSLInstallState.WSLStateDaemonStopped }) }, "start-docker"],
    ["a supported distribution without Docker gets the install", { docker: docker({ supported: true }) }, "install-docker"],
    ["an unsupported distribution gets the guide", { docker: docker({ supported: false }) }, "install-guide"],
    ["a running engine needs nothing", { docker: docker({ state: WSLInstallState.WSLStateRunning, reachable: true }) }, "none"],
  ] as const)("%s", (_n, d, expected) => expect(need(d as Partial<WSLDistroReport>)).toBe(expected));
});

describe("step flow", () => {
  const running = buildCandidates(report({ local: [local(true)] }));
  const stopped = buildCandidates(report({ wsl: { available: true, distros: [distro("x", { docker: docker({ state: WSLInstallState.WSLStateDaemonStopped }) })] } }));
  const at = (candidates: typeof running, step: OnboardingState["step"]): OnboardingState => {
    let s = onboardingReducer(initialOnboarding, { type: "detected", candidates });
    s = onboardingReducer(s, { type: "result", step: "detect", result: { text: "found" } });
    while (s.step !== step) s = onboardingReducer(s, { type: "next" });
    return s;
  };

  it("preselects the best candidate and keeps the user's choice when detection runs again", () => {
    const two = buildCandidates(report({ local: [local(true)], wsl: { available: true, distros: [distro("x", { docker: docker({ supported: true }) })] } }));
    let s = onboardingReducer(initialOnboarding, { type: "detected", candidates: two });
    expect(s.selectedId).toBe(two[0].id);
    s = onboardingReducer(s, { type: "select", id: two[1].id });
    s = onboardingReducer(s, { type: "detected", candidates: two });
    expect(s.selectedId).toBe(two[1].id);
  });

  it("skips the setup step for an engine that already runs", () => {
    const s = at(running, "choose");
    expect(nextStep(s)).toBe("check");
    expect(onboardingReducer(s, { type: "next" }).step).toBe("check");
    expect(stepStatuses(onboardingReducer(s, { type: "next" })).setup).toBe("skipped");
  });

  it("goes through setup for an engine that needs it, and cannot leave it until it is done", () => {
    const s = at(stopped, "setup");
    expect(onboardingReducer(s, { type: "next" }).step).toBe("setup");
    const fixed = onboardingReducer(s, { type: "detected", candidates: [{ ...stopped[0], state: "running" }] });
    expect(onboardingReducer(fixed, { type: "next" }).step).toBe("check");
  });

  it("goes back past the skipped setup step", () => {
    const s = at(running, "check");
    expect(onboardingReducer(s, { type: "back" }).step).toBe("choose");
  });

  it("only returns to steps that were completed, and forgets later results when another engine is chosen", () => {
    const two = [...running, ...stopped];
    let s = at(two, "check");
    s = onboardingReducer(s, { type: "result", step: "choose", result: { text: "Local" } });
    s = onboardingReducer(s, { type: "result", step: "check", result: { text: "Docker 29" } });
    s = onboardingReducer(s, { type: "goto", step: "choose" });
    expect(s.step).toBe("choose");
    expect(onboardingReducer(s, { type: "goto", step: "ready" }).step).toBe("choose");
    s = onboardingReducer(s, { type: "select", id: two[1].id });
    expect(s.results.check).toBeUndefined();
    expect(s.reached).toBe(1);
  });

  it("cannot reach the ready step before the check has succeeded", () => {
    const s = at(running, "check");
    expect(onboardingReducer(s, { type: "next" }).step).toBe("check");
    const failed = onboardingReducer(s, { type: "result", step: "check", result: { text: "failed", attention: true } });
    expect(onboardingReducer(failed, { type: "next" }).step).toBe("check");
    expect(stepStatuses(failed).check).toBe("attention");
    const ok = onboardingReducer(s, { type: "result", step: "check", result: { text: "ok" } });
    expect(onboardingReducer(ok, { type: "next" }).step).toBe("ready");
  });
});

describe("when the wizard appears", () => {
  const saved = { id: "a", name: "A", type: ConnectionType.ConnWSL, endpoint: "" } as ConnectionProfile;
  const found = { ...saved, id: "local-default", discovered: true };
  it("shows on a first launch with only the discovered default", () => expect(shouldOnboard("", [found])).toBe(true));
  it("never shows for someone who already saved a connection", () => expect(shouldOnboard("", [saved, found])).toBe(false));
  it("stays away once completed or skipped, even with nothing saved", () => {
    expect(shouldOnboard("completed", [found])).toBe(false);
    expect(shouldOnboard("skipped", [found])).toBe(false);
  });
  it("comes back when it was asked for again, even with saved connections", () => expect(shouldOnboard("pending", [saved])).toBe(true));
});

describe("detection summary", () => {
  it("reports nothing found as something that needs attention", () => {
    expect(detectSummary([])).toEqual({ text: "Nothing found", attention: true });
  });
  it("lists a stopped local engine as not found instead of hiding it", () => {
    const rows = detectRows(report({ os: "linux", local: [local(false)] }));
    expect(rows[0]).toMatchObject({ tone: "none", detail: expect.stringContaining("not running") });
  });
});
