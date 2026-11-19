import { describe, expect, it } from "vitest";
import { formatEntries, formatLogClock, formatLogTime, makeEntry, type LogEntry } from "./logEntry";

describe("makeEntry", () => {
  it("copies container, stream and seq and keeps the raw Engine timestamp", () => {
    const ts = "2026-01-02T03:04:05.123456789Z";
    const e = makeEntry({ timestamp: ts, stream: "stderr", text: "hi" }, "web", 7, 0);
    expect(e).toMatchObject({ container: "web", stream: "stderr", seq: 7, timestamp: ts, text: "hi" });
  });
  it("sorts by the Engine time, ignoring the fallback key, when the time is readable", () => {
    const e = makeEntry({ timestamp: "2026-01-02T03:04:05Z", stream: "stdout", text: "x" }, "web", 1, 5);
    expect(e.sortKey).toBe(Date.parse("2026-01-02T03:04:05Z"));
  });
  it("uses the fallback key and leaves time undefined for an unreadable timestamp", () => {
    const e = makeEntry({ timestamp: "not a time", stream: "stdout", text: "x" }, "web", 1, 42);
    expect(e.time).toBeUndefined();
    expect(e.sortKey).toBe(42);
    expect(e.timestamp).toBe("not a time");
  });
  it("treats a missing timestamp as empty text", () => {
    const e = makeEntry({ stream: "stdout", text: "x" } as never, "web", 1, 3);
    expect(e.timestamp).toBe("");
    expect(e.sortKey).toBe(3);
  });
  it("strips ANSI colours from the stored text", () => {
    expect(makeEntry({ timestamp: "", stream: "stdout", text: "\u001b[32mok\u001b[0m" }, "c", 1, 0).text).toBe("ok");
  });
});

describe("formatLogTime / formatLogClock", () => {
  // Built from local-time components so the expectation does not depend on the machine time zone.
  const at = new Date(2026, 0, 2, 3, 4, 5, 6).getTime();
  it("prints local MM-DD HH:mm:ss.SSS with zero padding", () => {
    expect(formatLogTime(at)).toBe("01-02 03:04:05.006");
    expect(formatLogTime(new Date(2026, 10, 23, 13, 14, 15, 678).getTime())).toBe("11-23 13:14:15.678");
  });
  it("prints local HH:mm:ss for the compact clock", () => {
    expect(formatLogClock(at)).toBe("03:04:05");
    expect(formatLogClock(new Date(2026, 5, 1, 23, 59, 59, 999).getTime())).toBe("23:59:59");
  });
  it("returns empty text when the time is unknown but not for epoch 0", () => {
    expect(formatLogTime(undefined)).toBe("");
    expect(formatLogClock(undefined)).toBe("");
    expect(formatLogTime(0)).not.toBe("");
    expect(formatLogClock(0)).not.toBe("");
  });
});

describe("formatEntries", () => {
  const e = (container: string, text: string, timestamp: string): LogEntry => makeEntry({ timestamp, stream: "stdout", text }, container, 1, 0);
  const list = [e("web", "one", "2026-01-01T00:00:01Z"), e("db", "two", "")];
  it("is empty for no entries", () => {
    expect(formatEntries([], { timestamps: true, container: true })).toBe("");
  });
  it("joins plain text lines with newlines", () => {
    expect(formatEntries(list, { timestamps: false, container: false })).toBe("one\ntwo");
  });
  it("adds the Engine timestamp when asked and skips it for lines that have none", () => {
    expect(formatEntries(list, { timestamps: true, container: false })).toBe("2026-01-01T00:00:01Z one\ntwo");
  });
  it("adds the container in brackets, after the timestamp", () => {
    expect(formatEntries(list, { timestamps: true, container: true })).toBe("2026-01-01T00:00:01Z [web] one\n[db] two");
  });
});
