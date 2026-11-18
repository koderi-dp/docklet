import { describe, expect, it } from "vitest";
import { shortcutLabel } from "./platform";

describe("shortcutLabel", () => {
  it("uses the Ctrl form outside macOS (the test environment is not a Mac)", () => {
    expect(shortcutLabel("K")).toBe("Ctrl+K");
  });
});
