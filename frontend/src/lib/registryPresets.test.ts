import { describe, expect, it } from "vitest";
import { authFailureHost, presetForHost } from "./registryPresets";

describe("authFailureHost", () => {
  it("reads the registry from the backend's authentication failures", () => {
    expect(authFailureHost("No credentials for ghcr.io. Sign in to ghcr.io in Settings > Registries (or run `docker login ghcr.io` in a terminal), then retry.")).toBe("ghcr.io");
    expect(authFailureHost("No credentials for 127.0.0.1:5055. Sign in to 127.0.0.1:5055 in Settings > Registries, then retry. The registry said: x")).toBe("127.0.0.1:5055");
    expect(authFailureHost("docker.io rejected the saved credentials (docklet). Sign in again in Settings > Registries")).toBe("docker.io");
  });
  it("ignores other failures", () => {
    expect(authFailureHost("Image not found: manifest unknown")).toBeNull();
    expect(authFailureHost(undefined)).toBeNull();
  });
});

describe("presetForHost", () => {
  it("falls back to the custom entry for unknown hosts", () => {
    expect(presetForHost("ghcr.io").id).toBe("ghcr.io");
    expect(presetForHost("127.0.0.1:5055").id).toBe("custom");
  });
});
