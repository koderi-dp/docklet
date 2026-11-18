import { describe, expect, it } from "vitest";
import { formatElapsed, serviceArgs } from "./watchFormat";

describe("formatElapsed", () => {
  it("formats seconds, minutes and hours", () => {
    expect(formatElapsed(0)).toBe("0s");
    expect(formatElapsed(45_900)).toBe("45s");
    expect(formatElapsed(133_000)).toBe("2m 13s");
    expect(formatElapsed(3_720_000)).toBe("1h 02m");
    expect(formatElapsed(-5)).toBe("0s");
  });
});

describe("serviceArgs", () => {
  const all = ["api", "web"];
  it("passes no service args when every service is ticked", () => {
    expect(serviceArgs(all, new Set(all))).toEqual([]);
  });
  it("passes exactly the ticked services otherwise", () => {
    expect(serviceArgs(all, new Set(["web"]))).toEqual(["web"]);
  });
});
