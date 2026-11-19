import { describe, expect, it } from "vitest";
import { formatDuration, isSecretName, relativeInside, summaryLine } from "./buildFormat";

describe("formatDuration", () => {
  it("shows tenths of a second below one second", () => {
    expect(formatDuration(0)).toBe("0.0 s");
    expect(formatDuration(400)).toBe("0.4 s");
  });
  it("shows whole seconds under a minute", () => {
    expect(formatDuration(1000)).toBe("1 s");
    expect(formatDuration(41_000)).toBe("41 s");
    expect(formatDuration(59_400)).toBe("59 s");
  });
  it("rolls 59.5 s over to a full minute rather than showing 60 s", () => {
    expect(formatDuration(59_500)).toBe("1 m 00 s");
  });
  it("shows minutes with zero-padded seconds", () => {
    expect(formatDuration(60_000)).toBe("1 m 00 s");
    expect(formatDuration(125_000)).toBe("2 m 05 s");
    expect(formatDuration(3_599_000)).toBe("59 m 59 s");
  });
  it("shows hours with zero-padded minutes and drops seconds", () => {
    expect(formatDuration(3_600_000)).toBe("1 h 00 m");
    expect(formatDuration(3_780_000)).toBe("1 h 03 m");
    expect(formatDuration(3_599_600)).toBe("1 h 00 m");
    expect(formatDuration(26 * 3_600_000 + 5 * 60_000)).toBe("26 h 05 m");
  });
  it("shows a dash for negative or non-finite durations", () => {
    for (const v of [-1, NaN, Infinity, -Infinity]) expect(formatDuration(v)).toBe("—");
  });
});

describe("summaryLine", () => {
  it("pluralises steps", () => {
    expect(summaryLine(1, 1, 0)).toBe("1 of 1 step");
    expect(summaryLine(0, 0, 0)).toBe("0 of 0 steps");
    expect(summaryLine(5, 9, 0)).toBe("5 of 9 steps");
  });
  it("adds cached and time only when known, cached only when above zero", () => {
    expect(summaryLine(5, 9, 2, 41_000)).toBe("5 of 9 steps · 2 cached · 41 s");
    expect(summaryLine(5, 9, 0, 41_000)).toBe("5 of 9 steps · 41 s");
    expect(summaryLine(5, 9, 2)).toBe("5 of 9 steps · 2 cached");
  });
  it("keeps a zero elapsed time (it is known, just fast)", () => {
    expect(summaryLine(1, 1, 0, 0)).toBe("1 of 1 step · 0.0 s");
  });
});

describe("isSecretName", () => {
  it("flags credential-looking names in any case and separator style", () => {
    for (const n of ["API_KEY", "api-key", "ApiKey", "GITHUB_TOKEN", "DB_PASSWORD", "DB_PWD", "AWS_SECRET_ACCESS_KEY", "ACCESS_KEY", "SSH_PRIVATE", "npm.token", "key", "SSH_KEY", "my-key", "KEY_ID", "credentials"]) {
      expect(isSecretName(n), n).toBe(true);
    }
  });
  it("leaves ordinary names alone, including words that merely contain 'key'", () => {
    for (const n of ["NODE_ENV", "PATH", "VERSION", "MONKEY", "TURKEY_COUNT", "KEYBOARD_LAYOUT", ""]) {
      expect(isSecretName(n), n).toBe(false);
    }
  });
});

describe("relativeInside", () => {
  it("returns the slash-separated path below a POSIX directory", () => {
    expect(relativeInside("/home/me/proj", "/home/me/proj/src/app.go")).toBe("src/app.go");
    expect(relativeInside("/home/me/proj", "/home/me/proj/Dockerfile")).toBe("Dockerfile");
  });
  it("ignores trailing separators on the directory", () => {
    expect(relativeInside("/home/me/proj/", "/home/me/proj/a")).toBe("a");
    expect(relativeInside("/home/me/proj//", "/home/me/proj/a")).toBe("a");
  });
  it("is null for the directory itself", () => {
    expect(relativeInside("/a/b", "/a/b")).toBeNull();
    expect(relativeInside("/a/b/", "/a/b")).toBeNull();
    expect(relativeInside("/a/b", "/a/b/")).toBeNull();
  });
  it("is null for a sibling that only shares a name prefix and for a parent", () => {
    expect(relativeInside("/a/b", "/a/bc/x")).toBeNull();
    expect(relativeInside("/a/b/c", "/a/b")).toBeNull();
    expect(relativeInside("/a/b", "/x/y")).toBeNull();
  });
  it("is case sensitive on POSIX paths", () => {
    expect(relativeInside("/a/B", "/a/b/c")).toBeNull();
  });
  it("handles Windows paths with backslashes and keeps the file's own case", () => {
    expect(relativeInside("C:\\proj", "C:\\proj\\src\\Dockerfile")).toBe("src/Dockerfile");
    expect(relativeInside("C:\\proj\\", "C:\\proj\\Sub\\File.TXT")).toBe("Sub/File.TXT");
  });
  it("compares Windows paths case-insensitively and accepts mixed separators", () => {
    expect(relativeInside("c:\\PROJ", "C:\\proj\\Sub\\F")).toBe("Sub/F");
    expect(relativeInside("C:/proj", "C:\\proj\\a\\b")).toBe("a/b");
  });
  it("is null for another drive, a sibling prefix or the same dir on Windows", () => {
    expect(relativeInside("C:\\proj", "D:\\proj\\a")).toBeNull();
    expect(relativeInside("C:\\proj", "C:\\project\\a")).toBeNull();
    expect(relativeInside("C:\\proj", "C:\\proj")).toBeNull();
    expect(relativeInside("C:\\proj", "C:\\proj\\")).toBeNull();
  });
  it("works for filesystem roots", () => {
    expect(relativeInside("/", "/etc/hosts")).toBe("etc/hosts");
    expect(relativeInside("C:\\", "C:\\x\\y")).toBe("x/y");
  });
});
