import { describe, expect, it } from "vitest";
import { ACTIONS } from "./wslActions";

describe("WSL action confirmations", () => {
  it("shutdown names every other running distribution and Docker Desktop", () => {
    const text = ACTIONS.shutdown.effects({ distro: "Ubuntu-24.04", otherRunning: ["Debian", "docker-desktop"] }).join("\n");
    expect(text).toContain("ALL distributions");
    expect(text).toContain("Debian, docker-desktop");
    expect(text).toContain("Docker Desktop");
  });

  it("distribution restart says others are unaffected and Docker restart says containers stop", () => {
    expect(ACTIONS["restart-distro"].effects({ distro: "U", otherRunning: ["Debian"] }).join("\n")).toContain("not affected (running now: Debian)");
    expect(ACTIONS["restart-docker"].effects({ distro: "U", otherRunning: [] }).join("\n")).toContain("restart policy");
  });
});
