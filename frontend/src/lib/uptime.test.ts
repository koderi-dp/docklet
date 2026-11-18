import { describe, expect, it } from "vitest";
import { uptimeSeconds } from "./uptime";

describe("uptimeSeconds", () => {
  it("reads the Engine's humanised durations", () => {
    expect(uptimeSeconds("2 hours")).toBe(7200);
    expect(uptimeSeconds("About an hour")).toBe(3600);
    expect(uptimeSeconds("45 seconds")).toBe(45);
    expect(uptimeSeconds("Less than a second")).toBe(0);
    expect(uptimeSeconds("3 days")).toBeGreaterThan(uptimeSeconds("20 hours")!);
  });
  it("is undefined for anything else", () => {
    expect(uptimeSeconds(undefined)).toBeUndefined();
    expect(uptimeSeconds("")).toBeUndefined();
    expect(uptimeSeconds("forever")).toBeUndefined();
  });
});
