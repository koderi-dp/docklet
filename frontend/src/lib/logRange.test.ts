import { describe, expect, it } from "vitest";
import { inTimeRange, parseLogTime } from "./logRange";

const now = Date.parse("2024-05-01T12:00:00Z");

describe("inTimeRange", () => {
  it("keeps lines inside the window and drops older ones", () => {
    expect(inTimeRange("2024-05-01T11:57:00.123456789Z", "5m", now)).toBe(true);
    expect(inTimeRange("2024-05-01T11:50:00Z", "5m", now)).toBe(false);
    expect(inTimeRange("2024-05-01T11:50:00Z", "15m", now)).toBe(true);
    expect(inTimeRange("2024-05-01T10:30:00Z", "1h", now)).toBe(false);
  });
  it("all keeps everything, and unreadable times are kept", () => {
    expect(inTimeRange("2020-01-01T00:00:00Z", "all", now)).toBe(true);
    expect(inTimeRange("", "5m", now)).toBe(true);
    expect(inTimeRange("garbage", "5m", now)).toBe(true);
  });
  it("parses nanosecond timestamps", () => {
    expect(parseLogTime("2024-05-01T12:00:00.999999999Z")).toBe(now + 999);
  });
});
