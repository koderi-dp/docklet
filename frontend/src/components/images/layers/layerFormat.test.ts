import { describe, expect, it } from "vitest";
import { formatAge, formatShare } from "./layerFormat";

describe("layerFormat", () => {
  it("formats shares without inventing precision", () => {
    expect(formatShare(0)).toBe("—");
    expect(formatShare(0.004)).toBe("<1%");
    expect(formatShare(0.614)).toBe("61%");
  });
  it("formats ages", () => {
    const now = Date.UTC(2026, 0, 1);
    expect(formatAge(0, now)).toBe("—");
    expect(formatAge(now / 1000 - 3 * 86400, now)).toBe("3d ago");
    expect(formatAge(now / 1000 - 90 * 86400, now)).toBe("3mo ago");
  });
});
