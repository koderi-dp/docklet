import { describe, expect, it } from "vitest";
import { normalizeTextSize, stepTextSize, textSizeShortcut } from "./textSize";

describe("stepTextSize", () => {
  it("walks through the four steps and stops at both ends", () => {
    expect(stepTextSize(100, 1)).toBe(110);
    expect(stepTextSize(110, 1)).toBe(125);
    expect(stepTextSize(125, 1)).toBe(125);
    expect(stepTextSize(100, -1)).toBe(90);
    expect(stepTextSize(90, -1)).toBe(90);
  });
  it("starts from the nearest step when the stored value is not one", () => {
    expect(stepTextSize(104, 1)).toBe(110);
  });
});

describe("normalizeTextSize", () => {
  it("falls back to 100 for values that are not numbers and snaps others to a step", () => {
    expect(normalizeTextSize(undefined)).toBe(100);
    expect(normalizeTextSize("125")).toBe(100);
    expect(normalizeTextSize(NaN)).toBe(100);
    expect(normalizeTextSize(300)).toBe(125);
    expect(normalizeTextSize(10)).toBe(90);
  });
});

describe("textSizeShortcut", () => {
  const k = (key: string, mods: Partial<KeyboardEvent> = {}) => ({ key, ctrlKey: true, metaKey: false, altKey: false, ...mods });
  it("maps Ctrl/Cmd with =, +, - and 0", () => {
    expect(textSizeShortcut(k("="))).toBe("larger");
    expect(textSizeShortcut(k("+"))).toBe("larger");
    expect(textSizeShortcut(k("-"))).toBe("smaller");
    expect(textSizeShortcut(k("0"))).toBe("reset");
    expect(textSizeShortcut(k("=", { ctrlKey: false, metaKey: true }))).toBe("larger");
  });
  it("ignores keys without the modifier or with Alt", () => {
    expect(textSizeShortcut(k("=", { ctrlKey: false }))).toBeNull();
    expect(textSizeShortcut(k("-", { altKey: true }))).toBeNull();
    expect(textSizeShortcut(k("a"))).toBeNull();
  });
});
