import { describe, expect, it } from "vitest";
import { currentStep } from "./useWslInstall";

describe("currentStep", () => {
  it("reads the latest step marker and ignores command output that merely looks similar", () => {
    const lines = ["[1/3] Refresh package lists", "$ apt-get update", "[2/3] Install Docker Engine", "Get:1 [4/4] not a marker", "  [3/3] indented"];
    expect(currentStep(lines)).toEqual({ index: 2, total: 3, title: "Install Docker Engine" });
  });

  it("is null before the first step starts", () => {
    expect(currentStep(["Installing Docker Engine in Ubuntu (12 steps)."])).toBeNull();
  });
});
